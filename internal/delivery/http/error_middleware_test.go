package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func errorRouter(t *testing.T, err error) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorMiddleware())
	router.GET("/fail", func(c *gin.Context) {
		c.Error(err)
	})
	return router
}

func performErrorRequest(t *testing.T, err error) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	errorRouter(t, err).ServeHTTP(recorder, req)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder.Code, body
}

func TestErrorMiddlewareMapsAppErrorTypes(t *testing.T) {
	cause := errors.New("technical detail")

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "not found",
			err:         apperror.NewNotFound("party_not_found", "party not found", cause),
			wantStatus:  http.StatusNotFound,
			wantCode:    "party_not_found",
			wantMessage: "party not found",
		},
		{
			name:        "validation",
			err:         apperror.NewValidation("invalid_party", "invalid party", cause),
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_party",
			wantMessage: "invalid party",
		},
		{
			name:        "unauthorized",
			err:         apperror.NewUnauthorized("unauthorized", "unauthorized", cause),
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "unauthorized",
			wantMessage: "unauthorized",
		},
		{
			name:        "internal hides technical detail",
			err:         apperror.NewInternal(errors.New("pq: password=secret")),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := performErrorRequest(t, tt.err)

			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantCode, body["code"])
			assert.Equal(t, tt.wantMessage, body["error"])
			assert.NotContains(t, body, "pq: password=secret")
		})
	}
}

func TestErrorMiddlewareDetectsWrappedAppError(t *testing.T) {
	err := fmt.Errorf("usecase: %w",
		apperror.NewValidation("invalid_party", "invalid party", errors.New("bad input")))

	status, body := performErrorRequest(t, err)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_party", body["code"])
	assert.Equal(t, "invalid party", body["error"])
}

func TestErrorMiddlewareHidesUnknownErrors(t *testing.T) {
	status, body := performErrorRequest(t, errors.New("connection refused: db.internal:5432"))

	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "internal_error", body["code"])
	assert.Equal(t, "internal server error", body["error"])
	assert.NotContains(t, body, "db.internal")
}

func TestErrorMiddlewareKeepsLegacySentinels(t *testing.T) {
	tests := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{domain.ErrPartyNotFound, http.StatusNotFound, "not_found"},
		{domain.ErrAlreadyDeleted, http.StatusConflict, "conflict"},
		{domain.ErrInvalidParty, http.StatusBadRequest, "bad_request"},
		{auth.ErrInvalidToken, http.StatusUnauthorized, "unauthorized"},
	}

	for _, tt := range tests {
		status, body := performErrorRequest(t, tt.err)
		assert.Equal(t, tt.wantStatus, status)
		assert.Equal(t, tt.wantCode, body["code"])
	}
}

func TestErrorMiddlewareIgnoresRequestsWithoutErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorMiddleware())
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
}
