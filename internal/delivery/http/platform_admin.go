package http

import (
	"crypto/subtle"
	"strings"

	"enterprise-system/internal/common/apperror"
	"github.com/gin-gonic/gin"
)

// PlatformAdminMiddleware authorizes platform-level administrative operations
// with a dedicated key. The key must be configured; an empty key denies all.
func PlatformAdminMiddleware(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader("X-Platform-Admin-Key"))
		if key == "" || provided == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
			c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", nil))
			c.Abort()
			return
		}
		c.Next()
	}
}
