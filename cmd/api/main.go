// @title           Enterprise System API
// @version         1.0
// @description     Multi-tenant Party Model API with JWT, refresh tokens, 2FA, onboarding and password recovery.
// @BasePath        /api/v1
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Bearer access token, e.g. "Bearer <token>".
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"enterprise-system/internal/auth"
	deliveryhttp "enterprise-system/internal/delivery/http"
	emailinfra "enterprise-system/internal/infrastructure/email"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/infrastructure/repository"
	"enterprise-system/internal/infrastructure/storage/local"
	"enterprise-system/internal/infrastructure/transaction"
	"enterprise-system/internal/usecase"
	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET is required and must be at least 32 bytes")
	}
	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		issuer = "enterprise-system"
	}

	client, err := enttenant.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = client.Close() }()

	tokenManager, err := auth.NewManager(auth.Config{
		Secret:     []byte(secret),
		Issuer:     issuer,
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
		TempTTL:    5 * time.Minute,
		ResetTTL:   time.Hour,
		BcryptCost: 12,
		Now:        time.Now,
	})
	if err != nil {
		log.Fatalf("configure auth: %v", err)
	}

	txManager := transaction.NewManager(client)

	partyRepo := repository.NewPartyRepository(client)
	partyService := usecase.NewPartyService(partyRepo, txManager, time.Now)
	credentialRepo := repository.NewCredentialRepository(client)
	partyContactRepo := repository.NewPartyContactRepository(client)
	sessionRepo := repository.NewSessionRepository(client)
	binder := enttenant.ContextBinder{}
	authService := usecase.NewAuthService(
		tokenManager, credentialRepo, partyContactRepo, sessionRepo, txManager, binder, time.Now,
	)
	accountService := usecase.NewAccountService(
		tokenManager, credentialRepo, partyContactRepo, sessionRepo,
		repository.NewPasswordResetTokenRepository(client),
		emailinfra.NewConsoleSender(os.Stdout),
		txManager, binder, time.Now,
	)
	onboardingService := usecase.NewRegisterTenantService(
		tokenManager,
		txManager,
		repository.NewTenantProvisioningStore(client),
		binder, time.Now,
	)
	orgService := usecase.NewOrgStructure(repository.NewOrgStructureRepository(client), txManager, time.Now)
	mergeService := usecase.NewMergeParties(txManager, repository.NewPartyMergeRepository(client), time.Now, nil)

	storageDir := strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_DIR"))
	if storageDir == "" {
		storageDir = "./data/documents"
	}
	documentStorage, err := local.New(storageDir)
	if err != nil {
		log.Fatalf("configure document storage: %v", err)
	}
	documentService := usecase.NewDocumentService(
		documentStorage,
		repository.NewAttachedDocumentRepository(client),
		txManager, "local", time.Now,
	)
	partyNameService := usecase.NewPartyNameService(
		repository.NewPartyNameRepository(client), txManager, time.Now,
	)
	locationRepo := repository.NewLocationRepository(client)
	locationService := usecase.NewLocationService(
		locationRepo, txManager, time.Now,
	)
	identityService := usecase.NewIdentityService(
		repository.NewIdentityRepository(client), locationRepo, txManager, time.Now,
	)

	allowedOrigins := splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000"}
	}

	router := deliveryhttp.NewRouter(deliveryhttp.Dependencies{
		PartyUseCase:        partyService,
		AuthUseCase:         authService,
		AccountUseCase:      accountService,
		OnboardingUseCase:   onboardingService,
		OrgStructureUseCase: orgService,
		MergePartiesUseCase: mergeService,
		DocumentUseCase:     documentService,
		PartyNameUseCase:    partyNameService,
		LocationUseCase:     locationService,
		IdentityUseCase:     identityService,
		TokenManager:        tokenManager,
		CookieSecure:        os.Getenv("COOKIE_SECURE") != "false",
		RefreshTTL:          30 * 24 * time.Hour,
		AuthRateLimit:       5,
		AuthRateWindow:      time.Minute,
		TrustedProxies:      splitCSV(os.Getenv("TRUSTED_PROXIES")),
		AllowedOrigins:      allowedOrigins,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("API listening on :%s", port)
		serverErr <- server.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run API: %v", err)
		}
		return
	case <-shutdownSignal.Done():
		log.Printf("shutdown signal received")
	}

	timeout := shutdownTimeout()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		_ = server.Close()
	} else {
		log.Printf("server stopped gracefully")
	}
	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server error during shutdown: %v", err)
	}
}

func splitCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	result := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func shutdownTimeout() time.Duration {
	value := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT"))
	if value == "" {
		return 10 * time.Second
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		log.Printf("invalid SHUTDOWN_TIMEOUT %q; using 10s", value)
		return 10 * time.Second
	}
	return timeout
}
