package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLocaleMiddleware(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"en-US,en;q=0.9", "en"},
		{"pt-BR,pt;q=0.9,en;q=0.8", "pt-BR"},
		{"es-ES,es;q=0.9", "es"},
		{"fr-FR,de;q=0.8", "es"},
		{"", "es"},
		{"en;q=0", "es"},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(LocaleMiddleware())
			router.GET("/locale", func(c *gin.Context) {
				c.String(http.StatusOK, LocaleFromContext(c.Request.Context()))
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/locale", nil)
			request.Header.Set("Accept-Language", tt.header)
			router.ServeHTTP(recorder, request)

			assert.Equal(t, tt.want, recorder.Body.String())
		})
	}
}
