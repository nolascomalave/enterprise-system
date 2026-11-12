package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ClientType string

const (
	ClientTypeFirstParty ClientType = "FIRST_PARTY"
	ClientTypeThirdParty ClientType = "THIRD_PARTY"
)

func (t ClientType) Valid() bool {
	return t == ClientTypeFirstParty || t == ClientTypeThirdParty
}

type ApplicationScope string

const (
	ApplicationScopePlatform       ApplicationScope = "PLATFORM"
	ApplicationScopeTenantSpecific ApplicationScope = "TENANT_SPECIFIC"
)

func (s ApplicationScope) Valid() bool {
	return s == ApplicationScopePlatform || s == ApplicationScopeTenantSpecific
}

// ClientApplication is a tenant-owned or platform OAuth 2.0 M2M client.
type ClientApplication struct {
	ID               uuid.UUID
	TenantID         *uuid.UUID
	Name             string
	ClientID         string
	ClientSecretHash string
	Scopes           []string
	ClientType       ClientType
	ApplicationScope ApplicationScope
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// ClientApplicationRepository persists OAuth client applications.
type ClientApplicationRepository interface {
	Create(ctx context.Context, application *ClientApplication) error
	FindByClientID(ctx context.Context, clientID string) (*ClientApplication, error)
}
