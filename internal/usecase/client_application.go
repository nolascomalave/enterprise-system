package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type CreateClientApplicationInput struct {
	TenantID         *uuid.UUID
	Name             string
	Scopes           []string
	ClientType       domain.ClientType
	ApplicationScope domain.ApplicationScope
}

type CreatedClientApplication struct {
	Application  *domain.ClientApplication
	ClientSecret string
}

type ClientToken struct {
	AccessToken string
	ExpiresIn   int
	Scopes      []string
}

type ClientApplicationService struct {
	repo    domain.ClientApplicationRepository
	manager *auth.Manager
	tx      TransactionManager
	now     func() time.Time
}

func NewClientApplicationService(
	repo domain.ClientApplicationRepository,
	manager *auth.Manager,
	tx TransactionManager,
	now func() time.Time,
) *ClientApplicationService {
	if now == nil {
		now = time.Now
	}
	return &ClientApplicationService{repo: repo, manager: manager, tx: tx, now: now}
}

func (s *ClientApplicationService) Create(ctx context.Context, in CreateClientApplicationInput) (*CreatedClientApplication, error) {
	if s == nil || s.repo == nil || s.manager == nil || s.tx == nil {
		return nil, errors.New("client applications: nil dependencies")
	}
	name := strings.TrimSpace(in.Name)
	clientType := in.ClientType
	if clientType == "" {
		clientType = domain.ClientTypeThirdParty
	}
	applicationScope := in.ApplicationScope
	if applicationScope == "" {
		applicationScope = domain.ApplicationScopeTenantSpecific
	}
	if !clientType.Valid() || !applicationScope.Valid() || name == "" || len(in.Scopes) == 0 {
		return nil, validationError("invalid_client_application", "invalid client application", domain.ErrInvalidClientApplication)
	}
	switch applicationScope {
	case domain.ApplicationScopeTenantSpecific:
		if in.TenantID == nil || *in.TenantID == uuid.Nil {
			return nil, validationError("invalid_client_application", "tenant_id is required for TENANT_SPECIFIC", domain.ErrInvalidClientApplication)
		}
	case domain.ApplicationScopePlatform:
		if clientType != domain.ClientTypeFirstParty || in.TenantID != nil {
			return nil, validationError("invalid_client_application", "PLATFORM apps must be FIRST_PARTY without tenant", domain.ErrInvalidClientApplication)
		}
	}

	scopes := make([]string, 0, len(in.Scopes))
	seen := make(map[string]struct{}, len(in.Scopes))
	for _, scope := range in.Scopes {
		scope = strings.TrimSpace(scope)
		if !auth.IsKnownScope(scope) {
			return nil, validationError("invalid_scope", "unknown scope "+scope, domain.ErrInvalidClientApplication)
		}
		if auth.IsPlatformScope(scope) &&
			(clientType != domain.ClientTypeFirstParty || applicationScope != domain.ApplicationScopePlatform) {
			return nil, validationError("invalid_scope", "platform scope requires FIRST_PARTY/PLATFORM", domain.ErrInvalidClientApplication)
		}
		if applicationScope == domain.ApplicationScopePlatform && !auth.IsPlatformScope(scope) {
			return nil, validationError("invalid_scope", "PLATFORM apps can only use platform scopes", domain.ErrInvalidClientApplication)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}

	clientID, clientSecret, err := auth.GenerateClientCredentials()
	if err != nil {
		return nil, internalError(err)
	}
	secretHash, err := s.manager.HashPassword(clientSecret)
	if err != nil {
		return nil, internalError(fmt.Errorf("hash client secret: %w", err))
	}
	now := s.now().UTC()
	applicationID, err := uuid.NewV7()
	if err != nil {
		return nil, internalError(fmt.Errorf("generate client application id: %w", err))
	}
	application := &domain.ClientApplication{
		ID: applicationID, TenantID: in.TenantID, Name: name,
		ClientID: clientID, ClientSecretHash: secretHash, Scopes: scopes,
		ClientType: clientType, ApplicationScope: applicationScope,
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.Create(ctx, application)
	}); err != nil {
		return nil, fmt.Errorf("create client application: %w", err)
	}
	return &CreatedClientApplication{Application: application, ClientSecret: clientSecret}, nil
}

func (s *ClientApplicationService) IssueToken(ctx context.Context, clientID, clientSecret string) (*ClientToken, error) {
	if s == nil || s.repo == nil || s.manager == nil {
		return nil, errors.New("client applications: nil dependencies")
	}
	clientID = strings.TrimSpace(clientID)
	application, err := s.repo.FindByClientID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrClientApplicationNotFound) {
			s.manager.DummyVerify(clientSecret)
			return nil, auth.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find client application: %w", err)
	}
	if !application.IsActive || !s.manager.VerifyPassword(application.ClientSecretHash, clientSecret) {
		return nil, auth.ErrInvalidCredentials
	}
	token, err := s.manager.IssueClientToken(
		application.TenantID, application.ClientID,
		string(application.ClientType), string(application.ApplicationScope),
		application.Scopes,
	)
	if err != nil {
		return nil, fmt.Errorf("issue client token: %w", err)
	}
	return &ClientToken{
		AccessToken: token,
		ExpiresIn:   int(s.manager.ClientTTL().Seconds()),
		Scopes:      application.Scopes,
	}, nil
}
