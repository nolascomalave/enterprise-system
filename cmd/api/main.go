package main

import (
	"log"
	"os"
	"strings"
	"time"

	"enterprise-system/internal/auth"
	deliveryhttp "enterprise-system/internal/delivery/http"
	emailinfra "enterprise-system/internal/infrastructure/email"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/infrastructure/repository"
	"enterprise-system/internal/usecase"
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

	partyRepo := repository.NewPartyRepository(client)
	partyService := usecase.NewPartyService(partyRepo, time.Now)
	credentialRepo := repository.NewCredentialRepository(client)
	sessionRepo := repository.NewSessionRepository(client)
	binder := enttenant.ContextBinder{}
	authService := usecase.NewAuthService(
		tokenManager, credentialRepo, sessionRepo, binder, time.Now,
	)
	accountService := usecase.NewAccountService(
		tokenManager, credentialRepo, sessionRepo,
		repository.NewPasswordResetTokenRepository(client),
		emailinfra.NewConsoleSender(os.Stdout),
		binder, time.Now,
	)

	var trustedProxies []string
	if value := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")); value != "" {
		for _, proxy := range strings.Split(value, ",") {
			if proxy = strings.TrimSpace(proxy); proxy != "" {
				trustedProxies = append(trustedProxies, proxy)
			}
		}
	}

	router := deliveryhttp.NewRouter(deliveryhttp.Dependencies{
		PartyUseCase:   partyService,
		AuthUseCase:    authService,
		AccountUseCase: accountService,
		TokenManager:   tokenManager,
		CookieSecure:   os.Getenv("COOKIE_SECURE") != "false",
		RefreshTTL:     30 * 24 * time.Hour,
		AuthRateLimit:  5,
		AuthRateWindow: time.Minute,
		TrustedProxies: trustedProxies,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("run API: %v", err)
	}
}
