package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/common/apperror"
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
// It is the single place where errors become HTTP status codes; handlers only
// call c.Error(err). Technical details are logged, never returned to clients.
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		status, code, message := mapError(err)
		c.AbortWithStatusJSON(status, APIError{Error: message, Code: code})
	}
}

func mapError(err error) (int, string, string) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		switch appErr.Type {
		case apperror.TypeNotFound:
			return http.StatusNotFound, fallback(appErr.Code, "not_found"), fallback(appErr.Message, "not found")
		case apperror.TypeValidation:
			return http.StatusBadRequest, fallback(appErr.Code, "bad_request"), fallback(appErr.Message, "bad request")
		case apperror.TypeUnauthorized:
			return http.StatusUnauthorized, fallback(appErr.Code, "unauthorized"), fallback(appErr.Message, "unauthorized")
		default:
			slog.Error("http: internal error", "error", err)
			return http.StatusInternalServerError, "internal_error", "internal server error"
		}
	}

	status, code := mapSentinelError(err)
	if status == http.StatusInternalServerError {
		slog.Error("http: unclassified error", "error", err)
	}
	return status, code, safeMessage(status)
}

// mapSentinelError is the compatibility layer for domain/auth sentinels that
// have not been classified with apperror yet.
func mapSentinelError(err error) (int, string) {
	switch {
	case errors.Is(err, auth.ErrInsufficientScope):
		return http.StatusForbidden, "forbidden"
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
	case errors.Is(err, domain.ErrPartyNotFound),
		errors.Is(err, domain.ErrContactMechanismNotFound),
		errors.Is(err, domain.ErrPartyContactNotFound),
		errors.Is(err, domain.ErrRoleTypeNotFound),
		errors.Is(err, domain.ErrCorporateGroupNotFound),
		errors.Is(err, domain.ErrPartyNameNotFound),
		errors.Is(err, domain.ErrPartyNameValidityNotFound),
		errors.Is(err, domain.ErrNameTypeNotFound),
		errors.Is(err, domain.ErrLocationNotFound),
		errors.Is(err, domain.ErrIdentityDocumentNotFound),
		errors.Is(err, domain.ErrIdentityDocumentTypeNotFound),
		errors.Is(err, domain.ErrDepartmentContactNotFound),
		errors.Is(err, domain.ErrDepartmentAddressNotFound),
		errors.Is(err, domain.ErrAddressValidityNotFound),
		errors.Is(err, domain.ErrSystemFunctionNotFound),
		errors.Is(err, domain.ErrMenuTemplateNotFound),
		errors.Is(err, domain.ErrMenuCategoryNotFound),
		errors.Is(err, domain.ErrMenuOptionNotFound),
		errors.Is(err, domain.ErrTenantRoleNotFound),
		errors.Is(err, domain.ErrRoleAssignmentNotFound),
		errors.Is(err, domain.ErrPermissionNotFound),
		errors.Is(err, domain.ErrMenuOptionFunctionNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "timeout"
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_timeout"
	case errors.Is(err, domain.ErrAlreadyDeleted),
		errors.Is(err, domain.ErrTenantSlugTaken),
		errors.Is(err, domain.ErrTenantAdminEmailTaken),
		errors.Is(err, domain.ErrPartyMergeTargetNotActive),
		errors.Is(err, domain.ErrPartyNotMergeable),
		errors.Is(err, auth.ErrConcurrentModification),
		errors.Is(err, domain.ErrIdentityDocumentExists),
		errors.Is(err, domain.ErrCorporateGroupTenantMismatch),
		errors.Is(err, domain.ErrDepartmentTenantMismatch),
		errors.Is(err, domain.ErrPartyTenantMismatch):
		return http.StatusConflict, "conflict"
	case errors.Is(err, ErrBadRequest),
		errors.Is(err, domain.ErrInvalidTenantRegistration),
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
		errors.Is(err, domain.ErrInvalidCorporateGroup),
		errors.Is(err, domain.ErrInvalidPartyName),
		errors.Is(err, domain.ErrInvalidLocation),
		errors.Is(err, domain.ErrLocationCountryMismatch),
		errors.Is(err, domain.ErrInvalidNationality),
		errors.Is(err, domain.ErrInvalidIdentityDocument),
		errors.Is(err, domain.ErrJurisdictionCountryMismatch),
		errors.Is(err, domain.ErrInvalidDepartmentContact),
		errors.Is(err, domain.ErrInvalidDepartmentAddress),
		errors.Is(err, domain.ErrEmployeeSelfManager),
		errors.Is(err, domain.ErrInvalidPartyMerge),
		errors.Is(err, domain.ErrInvalidSubscription),
		errors.Is(err, domain.ErrInvalidRoleType),
		errors.Is(err, domain.ErrInvalidPartyRole),
		errors.Is(err, domain.ErrInvalidSystemFunction),
		errors.Is(err, domain.ErrInvalidMenuTemplate),
		errors.Is(err, domain.ErrInvalidMenuCategory),
		errors.Is(err, domain.ErrInvalidMenuOption),
		errors.Is(err, domain.ErrInvalidTenantRole),
		errors.Is(err, domain.ErrInvalidRoleAssignment),
		errors.Is(err, domain.ErrInvalidPermission),
		errors.Is(err, domain.ErrInvalidMenuOptionFunction):
		return http.StatusBadRequest, "bad_request"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func safeMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestTimeout:
		return "request timed out"
	case http.StatusGatewayTimeout:
		return "timeout"
	default:
		return "internal server error"
	}
}

// TenantMiddleware validates the Bearer access token and injects its tenant,
// user and session claims into the request context. Client-credentials tokens
// also inject their tenant but must satisfy the route's OAuth scope.
func TenantMiddleware(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" || manager == nil {
			c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("bearer access token is required")))
			c.Abort()
			return
		}

		if claims, err := manager.ParseAccessToken(raw); err == nil {
			tenantID, parseErr := uuid.Parse(claims.TenantID)
			if parseErr != nil {
				c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("invalid tenant claim")))
				c.Abort()
				return
			}
			userID, parseErr := uuid.Parse(claims.UserID)
			if parseErr != nil {
				c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("invalid user claim")))
				c.Abort()
				return
			}
			sessionID, parseErr := uuid.Parse(claims.SessionID)
			if parseErr != nil {
				c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("invalid session claim")))
				c.Abort()
				return
			}
			bindTenantSession(c, tenantID, userID, sessionID)
			c.Next()
			return
		}

		clientClaims, err := manager.ParseClientToken(raw)
		if err != nil {
			c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("invalid access token")))
			c.Abort()
			return
		}
		tenantID := uuid.Nil
		if clientClaims.ApplicationScope == "TENANT_SPECIFIC" {
			parsed, parseErr := uuid.Parse(clientClaims.TenantID)
			if parseErr != nil {
				c.Error(apperror.NewUnauthorized("unauthorized", "unauthorized", fmt.Errorf("invalid tenant claim")))
				c.Abort()
				return
			}
			tenantID = parsed
		}
		fullPath := c.FullPath()
		if fullPath == "" {
			fullPath = c.Request.URL.Path
		}
		scope, ok := requiredScope(c.Request.Method, fullPath)
		if !ok || !auth.HasScope(clientClaims.Scopes, scope) {
			c.Error(fmt.Errorf("%w: client credentials cannot access this route", auth.ErrInsufficientScope))
			c.Abort()
			return
		}
		bindTenantSession(c, tenantID, uuid.Nil, uuid.Nil)
		c.Next()
	}
}

func bindTenantSession(c *gin.Context, tenantID, userID, sessionID uuid.UUID) {
	sessionContext := enttenant.SessionContext{
		TenantID: tenantID, UserID: userID, SessionID: sessionID,
	}
	ctx := enttenant.WithSessionContext(c.Request.Context(), sessionContext)
	c.Request = c.Request.WithContext(ctx)
	c.Set("session_context", sessionContext)
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
