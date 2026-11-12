package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ClientApplication is a tenant-owned OAuth 2.0 machine-to-machine client.
type ClientApplication struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Name             string
	ClientID         string
	ClientSecretHash string
	Scopes           []string
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
