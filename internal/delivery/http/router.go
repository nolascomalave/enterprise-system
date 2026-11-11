package http

import (
	"net/http"
	"time"

	_ "enterprise-system/docs"
	"enterprise-system/internal/auth"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Dependencies struct {
	PartyUseCase        PartyUseCase
	AuthUseCase         AuthUseCase
	AccountUseCase      AccountUseCase
	OnboardingUseCase   OnboardingUseCase
	OrgStructureUseCase OrgStructureUseCase
	MergePartiesUseCase MergePartiesUseCase
	TokenManager        *auth.Manager

	CookieSecure   bool
	RefreshTTL     time.Duration
	AuthRateLimit  int
	AuthRateWindow time.Duration
	TrustedProxies []string
	AllowedOrigins []string
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	if err := router.SetTrustedProxies(deps.TrustedProxies); err != nil {
		// A bad proxy configuration must not silently trust arbitrary clients.
		_ = router.SetTrustedProxies(nil)
	}
	router.Use(
		CORS(deps.AllowedOrigins),
		gin.Logger(),
		gin.CustomRecovery(func(c *gin.Context, _ any) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
				Error: "internal server error",
				Code:  "internal_error",
			})
		}),
		SecurityHeaders(),
		ErrorMiddleware(),
	)

	if deps.AuthRateLimit <= 0 {
		deps.AuthRateLimit = 5
	}
	if deps.AuthRateWindow <= 0 {
		deps.AuthRateWindow = time.Minute
	}
	loginLimiter := NewRateLimiter(deps.AuthRateLimit, deps.AuthRateWindow)
	refreshLimiter := NewRateLimiter(deps.AuthRateLimit, deps.AuthRateWindow)
	twoFALimiter := NewRateLimiter(deps.AuthRateLimit, deps.AuthRateWindow)
	forgotLimiter := NewRateLimiter(deps.AuthRateLimit, deps.AuthRateWindow)
	resetLimiter := NewRateLimiter(deps.AuthRateLimit, deps.AuthRateWindow)
	onboardingLimiter := NewRateLimiter(3, time.Hour)

	authHandler := NewAuthHandler(deps.AuthUseCase, deps.CookieSecure, deps.RefreshTTL)
	accountHandler := NewAccountHandler(deps.AccountUseCase)
	onboardingHandler := NewOnboardingHandler(deps.OnboardingUseCase)
	orgHandler := NewOrgHandler(deps.OrgStructureUseCase)
	mergeHandler := NewMergeHandler(deps.MergePartiesUseCase)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	api.GET("/health", Health)

	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/login", RateLimit(loginLimiter), authHandler.Login)
		authRoutes.POST("/refresh", RateLimit(refreshLimiter), authHandler.Refresh)
		authRoutes.POST("/2fa/verify", RateLimit(twoFALimiter), authHandler.Verify2FA)
		authRoutes.POST("/logout", authHandler.Logout)
		authRoutes.POST("/forgot-password", RateLimit(forgotLimiter), accountHandler.ForgotPassword)
		authRoutes.POST("/reset-password", RateLimit(resetLimiter), accountHandler.ResetPassword)
	}

	api.POST("/onboarding/register", RateLimit(onboardingLimiter), onboardingHandler.Register)

	partyHandler := NewPartyHandler(deps.PartyUseCase)
	protected := api.Group("")
	protected.Use(TenantMiddleware(deps.TokenManager))
	{
		protected.POST("/parties", partyHandler.Create)
		protected.GET("/parties", partyHandler.List)
		protected.GET("/parties/:id", partyHandler.GetByID)
		protected.POST("/parties/merge", mergeHandler.Merge)
		protected.POST("/jobs", orgHandler.CreateJob)
		protected.GET("/jobs", orgHandler.ListJobs)
		protected.PUT("/jobs/:id", orgHandler.UpdateJob)
		protected.POST("/departments", orgHandler.CreateDepartment)
		protected.GET("/departments", orgHandler.ListDepartments)
		protected.POST("/employees", orgHandler.CreateEmployee)
		protected.GET("/employees", orgHandler.ListEmployees)
		protected.POST("/auth/change-password", accountHandler.ChangePassword)
	}

	return router
}
