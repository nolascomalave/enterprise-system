package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrBadRequest   = errors.New("bad request")
)

type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// ErrorMiddleware maps domain errors to a standard JSON error response.
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		status, code := mapError(err)
		message := err.Error()
		if status == http.StatusInternalServerError {
			message = "internal server error"
		}
		c.AbortWithStatusJSON(status, APIError{Error: message, Code: code})
	}
}

func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, auth.ErrInvalidCredentials),
		errors.Is(err, auth.ErrInvalidToken),
		errors.Is(err, auth.ErrExpiredToken),
		errors.Is(err, auth.ErrInvalidTOTP),
		errors.Is(err, auth.ErrInvalidBackupCode),
		errors.Is(err, auth.ErrSessionRevoked),
		errors.Is(err, auth.ErrSessionExpired),
		errors.Is(err, auth.ErrInvalidCurrentPassword),
		errors.Is(err, auth.Err2FARequired):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, domain.ErrPartyNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "timeout"
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_timeout"
	case errors.Is(err, domain.ErrAlreadyDeleted),
		errors.Is(err, domain.ErrPartyMergeTargetNotActive),
		errors.Is(err, domain.ErrPartyNotMergeable),
		errors.Is(err, domain.ErrPartyTenantMismatch):
		return http.StatusConflict, "conflict"
	case errors.Is(err, ErrBadRequest),
		errors.Is(err, auth.Err2FANotEnabled),
		errors.Is(err, auth.ErrWeakPassword),
		errors.Is(err, auth.ErrSamePassword),
		errors.Is(err, auth.ErrInvalidResetToken),
		errors.Is(err, domain.ErrInvalidParty),
		errors.Is(err, domain.ErrInvalidPersonProfile),
		errors.Is(err, domain.ErrInvalidOrganizationProfile),
		errors.Is(err, domain.ErrInvalidContactMechanism),
		errors.Is(err, domain.ErrInvalidAddress),
		errors.Is(err, domain.ErrInvalidEmployee),
		errors.Is(err, domain.ErrInvalidEmployeeAssignment),
		errors.Is(err, domain.ErrInvalidJob),
		errors.Is(err, domain.ErrInvalidJobFamily),
		errors.Is(err, domain.ErrInvalidDepartment),
		errors.Is(err, domain.ErrEmployeeSelfManager),
		errors.Is(err, domain.ErrInvalidPartyMerge),
		errors.Is(err, domain.ErrInvalidSubscription),
		errors.Is(err, domain.ErrInvalidRoleType),
		errors.Is(err, domain.ErrInvalidPartyRole):
		return http.StatusBadRequest, "bad_request"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// TenantMiddleware validates the Bearer access token and injects its tenant,
// user and session claims into the request context. Requests without a valid
// token are rejected before reaching any handler.
func TenantMiddleware(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" || manager == nil {
			c.Error(fmt.Errorf("%w: bearer access token is required", ErrUnauthorized))
			c.Abort()
			return
		}

		claims, err := manager.ParseAccessToken(raw)
		if err != nil {
			c.Error(fmt.Errorf("%w: invalid access token", ErrUnauthorized))
			c.Abort()
			return
		}

		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			c.Error(fmt.Errorf("%w: invalid tenant claim", ErrUnauthorized))
			c.Abort()
			return
		}
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.Error(fmt.Errorf("%w: invalid user claim", ErrUnauthorized))
			c.Abort()
			return
		}
		sessionID, err := uuid.Parse(claims.SessionID)
		if err != nil {
			c.Error(fmt.Errorf("%w: invalid session claim", ErrUnauthorized))
			c.Abort()
			return
		}

		sessionContext := enttenant.SessionContext{
			TenantID: tenantID, UserID: userID, SessionID: sessionID,
		}
		ctx := enttenant.WithSessionContext(c.Request.Context(), sessionContext)
		c.Request = c.Request.WithContext(ctx)
		c.Set("session_context", sessionContext)
		c.Next()
	}
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
