package http

import (
	"context"
	"fmt"
	"strings"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type permissionContextKey struct{}

// WithPermissionUniverse injects a resolved permission universe into the context.
func WithPermissionUniverse(ctx context.Context, universe *domain.PermissionUniverse) context.Context {
	return context.WithValue(ctx, permissionContextKey{}, universe)
}

// PermissionUniverseFromContext returns the permission universe previously injected by RBACMiddleware.
func PermissionUniverseFromContext(ctx context.Context) (*domain.PermissionUniverse, bool) {
	universe, ok := ctx.Value(permissionContextKey{}).(*domain.PermissionUniverse)
	return universe, ok
}

// RBACUseCase is the application boundary used by RBACMiddleware.
// It is already defined in rbac_handler.go; this alias keeps the file self-contained.

// RBACMiddleware authorizes the request against the user's resolved permission universe.
// It runs after TenantMiddleware. M2M client-credentials tokens (UserID == uuid.Nil)
// are skipped because TenantMiddleware already validated their OAuth2 scopes.
func RBACMiddleware(uc RBACUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		sc, ok := enttenant.SessionContextFrom(c.Request.Context())
		if !ok {
			c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("missing session context")))
			c.Abort()
			return
		}

		// M2M tokens are authorized by TenantMiddleware via OAuth2 scopes.
		if sc.UserID == uuid.Nil {
			c.Next()
			return
		}

		fullPath := c.FullPath()
		if fullPath == "" {
			fullPath = c.Request.URL.Path
		}

		required, ok := requiredRBACFunction(c.Request.Method, fullPath)
		if !ok {
			c.Error(fmt.Errorf("%w: route is not mapped to a system function", auth.ErrInsufficientScope))
			c.Abort()
			return
		}

		rawAppClientID := strings.TrimSpace(c.GetHeader("X-App-Client-ID"))
		if rawAppClientID == "" {
			c.Error(apperror.NewValidation("missing_app_client", "X-App-Client-ID header is required", nil))
			c.Abort()
			return
		}
		appClientID, err := uuid.Parse(rawAppClientID)
		if err != nil || appClientID == uuid.Nil {
			c.Error(apperror.NewValidation("invalid_app_client", "X-App-Client-ID must be a valid UUID", err))
			c.Abort()
			return
		}

		universe, err := uc.ResolvePermissions(c.Request.Context(), usecase.ResolvePermissionsInput{
			TenantID:          sc.TenantID,
			UserCredentialsID: sc.UserID,
			AppClientID:       appClientID,
		})
		if err != nil {
			c.Error(err)
			c.Abort()
			return
		}

		if !hasPermission(universe.Functions, required) {
			c.Error(fmt.Errorf("%w: missing function %s", auth.ErrInsufficientScope, required))
			c.Abort()
			return
		}

		ctx := WithPermissionUniverse(c.Request.Context(), universe)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func hasPermission(functions []string, required string) bool {
	for _, fn := range functions {
		if fn == required || fn == "*" {
			return true
		}
	}
	return false
}
