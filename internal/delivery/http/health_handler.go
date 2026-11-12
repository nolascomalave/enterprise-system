package http

import (
	"net/http"
	"time"

	"enterprise-system/internal/delivery/http/dto"
	"github.com/gin-gonic/gin"
)

// Health godoc
// @Summary      Health check
// @Description  Returns the liveness state of the API server.
// @Tags         Health
// @Produce      json
// @Success      200 {object} dto.HealthEnvelope
// @Router       /health [get]
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": dto.HealthResponse{
		Status:    "ok",
		Service:   "enterprise-system",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}})
}
