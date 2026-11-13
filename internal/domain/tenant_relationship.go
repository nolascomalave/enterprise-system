package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RelationshipStatus string

const (
	RelationshipPending  RelationshipStatus = "PENDING"
	RelationshipApproved RelationshipStatus = "APPROVED"
	RelationshipRejected RelationshipStatus = "REJECTED"
	RelationshipCanceled RelationshipStatus = "CANCELED"
)

func (s RelationshipStatus) Valid() bool {
	switch s {
	case RelationshipPending, RelationshipApproved, RelationshipRejected, RelationshipCanceled:
		return true
	default:
		return false
	}
}

// TenantRelationship links a holding tenant with a subsidiary.
type TenantRelationship struct {
	ID                 uuid.UUID
	TenantIDHolding    uuid.UUID
	TenantIDSubsidiary uuid.UUID
	RelationshipType   string
	Status             RelationshipStatus
	RequestedAt        time.Time
	ApprovedAt         *time.Time
	RejectedAt         *time.Time
	CanceledAt         *time.Time
	EndsAt             *time.Time
	RejectionReason    string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewPendingTenantRelationship(holding, subsidiary uuid.UUID, relationshipType string, now time.Time) (*TenantRelationship, error) {
	relationshipType = strings.TrimSpace(relationshipType)
	if holding == uuid.Nil || subsidiary == uuid.Nil || holding == subsidiary || relationshipType == "" {
		return nil, fmt.Errorf("%w: invalid holding/subsidiary/type", ErrInvalidTenantRelationship)
	}
	now = now.UTC()
	return &TenantRelationship{
		ID:                 uuid.Must(uuid.NewV7()),
		TenantIDHolding:    holding,
		TenantIDSubsidiary: subsidiary,
		RelationshipType:   relationshipType,
		Status:             RelationshipPending,
		RequestedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func (r *TenantRelationship) Cancel(at time.Time, reason string) error {
	if r.Status != RelationshipPending {
		return fmt.Errorf("%w: only PENDING can be canceled", ErrInvalidRelationshipTransition)
	}
	at = at.UTC()
	r.Status = RelationshipCanceled
	r.CanceledAt = &at
	r.EndsAt = &at
	r.UpdatedAt = at
	if strings.TrimSpace(reason) != "" {
		r.RejectionReason = strings.TrimSpace(reason)
	}
	return nil
}

func (r *TenantRelationship) Reject(at time.Time, reason string) error {
	if r.Status != RelationshipPending {
		return fmt.Errorf("%w: only PENDING can be rejected", ErrInvalidRelationshipTransition)
	}
	at = at.UTC()
	r.Status = RelationshipRejected
	r.RejectedAt = &at
	r.RejectionReason = strings.TrimSpace(reason)
	r.UpdatedAt = at
	return nil
}

// TenantRelationshipInvite is an invitation to accept a pending relationship.
type TenantRelationshipInvite struct {
	ID                 uuid.UUID
	RelationshipID     uuid.UUID
	TokenHash          []byte
	ExpiresAt          time.Time
	ConsumedAt         *time.Time
	CanceledAt         *time.Time
	CanceledByTenantID *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (i *TenantRelationshipInvite) Active(now time.Time) bool {
	return i.CanceledAt == nil && i.ConsumedAt == nil && i.ExpiresAt.After(now)
}

// TenantTransfer tracks a transfer request for a subsidiary.
type TenantTransfer struct {
	ID                  uuid.UUID
	RelationshipID      uuid.UUID
	RequestedByTenantID uuid.UUID
	Status              string
	RequestedAt         time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
	CanceledAt          *time.Time
	CanceledByTenantID  *uuid.UUID
	TransferReason      string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// TenantRelationshipRepository persists relationships, invites and transfers.
type TenantRelationshipRepository interface {
	// Tenant validation
	TenantActive(ctx context.Context, tenantID uuid.UUID) (bool, error)

	// Relationships
	CreateRelationship(ctx context.Context, relationship *TenantRelationship) error
	LockRelationship(ctx context.Context, relationshipID uuid.UUID) (*TenantRelationship, error)
	WouldCreateCycle(ctx context.Context, holdingID, subsidiaryID uuid.UUID) (bool, error)
	FindRelationship(ctx context.Context, relationshipID uuid.UUID) (*TenantRelationship, error)
	UpdateRelationship(ctx context.Context, relationship *TenantRelationship) error

	// Invites
	CreateInvite(ctx context.Context, invite *TenantRelationshipInvite) error
	FindInviteByTokenHash(ctx context.Context, tokenHash []byte) (*TenantRelationshipInvite, error)
	CancelActiveInvites(ctx context.Context, relationshipID, canceledByTenantID uuid.UUID, at time.Time) error
	MarkInviteConsumed(ctx context.Context, inviteID uuid.UUID, at time.Time) error

	// Transfers
	CreateTransfer(ctx context.Context, transfer *TenantTransfer) error
	FindTransfer(ctx context.Context, transferID uuid.UUID) (*TenantTransfer, error)
}
