package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func productionRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return NewRouter(Dependencies{
		TokenManager:   testAuthManager(t),
		AllowedOrigins: []string{"http://localhost:3000"},
	})
}

func TestHealth(t *testing.T) {
	router := productionRouter(t)
	recorder := requestWithToken(router, http.MethodGet, "/api/v1/health", nil, "")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, "ok", data["status"])
	assert.Equal(t, "enterprise-system", data["service"])
}

func TestCORSAllowedOrigin(t *testing.T) {
	router := productionRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "http://localhost:3000", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, recorder.Header().Values("Vary"), "Origin")
}

func TestCORSPreflight(t *testing.T) {
	router := productionRouter(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "http://localhost:3000", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, recorder.Header().Values("Vary"), "Access-Control-Request-Method")
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	router := productionRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestSwaggerUI(t *testing.T) {
	router := productionRouter(t)
	recorder := requestWithToken(router, http.MethodGet, "/swagger/index.html", nil, "")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, strings.Contains(strings.ToLower(recorder.Body.String()), "swagger"))
}
