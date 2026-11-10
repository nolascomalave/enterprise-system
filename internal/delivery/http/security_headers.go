package http

import "github.com/gin-gonic/gin"

// SecurityHeaders adds conservative headers to every response. HSTS is left to
// the TLS-terminating proxy because it must only be sent over HTTPS.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
