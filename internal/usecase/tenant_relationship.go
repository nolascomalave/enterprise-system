package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// TenantRelationshipUseCase orchestrates holding/subsidiary relationship flows.
type TenantRelationshipUseCase interface {
	StartPendingRelationship(ctx context.Context, input StartPendingRelationshipInput) (*domain.TenantRelationship, error)
	EmitInviteToken(ctx context.Context, input EmitInviteTokenInput) (*EmitInviteTokenResult, error)
	CancelPendingRelationship(ctx context.Context, input CancelPendingRelationshipInput) (*domain.TenantRelationship, error)
	RejectRelationship(ctx context.Context, input RejectRelationshipInput) (*domain.TenantRelationship, error)
	ConsumeInviteToken(ctx context.Context, input ConsumeInviteTokenInput) (*domain.TenantRelationship, error)
	RevokeApprovedRelationship(ctx context.Context, input RevokeApprovedRelationshipInput) (*domain.TenantRelationship, error)
}

type TenantRelationshipService struct {
	repo domain.TenantRelationshipRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewTenantRelationshipService(
	repo domain.TenantRelationshipRepository,
	tx TransactionManager,
	now func() time.Time,
) *TenantRelationshipService {
	if now == nil {
		now = time.Now
	}
	return &TenantRelationshipService{repo: repo, tx: tx, now: now}
}

type StartPendingRelationshipInput struct {
	HoldingTenantID    uuid.UUID
	SubsidiaryTenantID uuid.UUID
	RelationshipType   string
}

// StartPendingRelationship only creates the PENDING row. It deliberately does
// not emit an invite token: the subsidiary must not hold the token that would
// approve its own request.
func (s *TenantRelationshipService) StartPendingRelationship(
	ctx context.Context,
	input StartPendingRelationshipInput,
) (*domain.TenantRelationship, error) {
	now := s.now().UTC()
	relationship, err := domain.NewPendingTenantRelationship(
		input.HoldingTenantID, input.SubsidiaryTenantID, input.RelationshipType, now,
	)
	if err != nil {
		return nil, validationError("invalid_relationship", err.Error(), err)
	}

	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		holdingActive, err := s.repo.TenantActive(ctx, input.HoldingTenantID)
		if err != nil {
			return fmt.Errorf("check holding tenant: %w", err)
		}
		if !holdingActive {
			return validationError("tenant_inactive", "holding tenant is inactive or deleted", domain.ErrTenantInactive)
		}
		subsidiaryActive, err := s.repo.TenantActive(ctx, input.SubsidiaryTenantID)
		if err != nil {
			return fmt.Errorf("check subsidiary tenant: %w", err)
		}
		if !subsidiaryActive {
			return validationError("tenant_inactive", "subsidiary tenant is inactive or deleted", domain.ErrTenantInactive)
		}
		if err := s.repo.CreateRelationship(ctx, relationship); err != nil {
			return fmt.Errorf("create tenant relationship: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return relationship, nil
}

type EmitInviteTokenInput struct {
	RelationshipID uuid.UUID
	HoldingID      uuid.UUID
	TTL            time.Duration
}

type EmitInviteTokenResult struct {
	InviteCode string
	Invite     *domain.TenantRelationshipInvite
}

// EmitInviteToken is the holding-only action that issues the invite token.
func (s *TenantRelationshipService) EmitInviteToken(
	ctx context.Context,
	input EmitInviteTokenInput,
) (*EmitInviteTokenResult, error) {
	if input.RelationshipID == uuid.Nil || input.HoldingID == uuid.Nil {
		return nil, validationError("invalid_input", "relationship_id and holding_id are required", domain.ErrInvalidRelationshipInvite)
	}
	if input.TTL <= 0 {
		input.TTL = 7 * 24 * time.Hour
	}
	now := s.now().UTC()

	var result *EmitInviteTokenResult
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		relationship, err := s.repo.LockRelationship(ctx, input.RelationshipID)
		if err != nil {
			return err
		}
		if relationship.Status != domain.RelationshipPending {
			return validationError("invalid_transition", "relationship is not PENDING", domain.ErrInvalidRelationshipTransition)
		}
		if relationship.TenantIDHolding != input.HoldingID {
			return validationError("forbidden", "only the holding can emit invites", domain.ErrUnauthorizedRelationshipAction)
		}

		inviteCode, inviteHash, err := newInviteCode()
		if err != nil {
			return fmt.Errorf("generate invite code: %w", err)
		}
		invite := &domain.TenantRelationshipInvite{
			ID:             uuid.Must(uuid.NewV7()),
			RelationshipID: relationship.ID,
			TokenHash:      inviteHash,
			ExpiresAt:      now.Add(input.TTL),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := s.repo.CreateInvite(ctx, invite); err != nil {
			return fmt.Errorf("create tenant relationship invite: %w", err)
		}
		result = &EmitInviteTokenResult{InviteCode: inviteCode, Invite: invite}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type CancelPendingRelationshipInput struct {
	RelationshipID uuid.UUID
	SubsidiaryID   uuid.UUID
	Reason         string
}

func (s *TenantRelationshipService) CancelPendingRelationship(
	ctx context.Context,
	input CancelPendingRelationshipInput,
) (*domain.TenantRelationship, error) {
	if input.RelationshipID == uuid.Nil || input.SubsidiaryID == uuid.Nil {
		return nil, validationError("invalid_input", "relationship_id and subsidiary_id are required", domain.ErrInvalidTenantRelationship)
	}
	now := s.now().UTC()

	var result *domain.TenantRelationship
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		relationship, err := s.repo.LockRelationship(ctx, input.RelationshipID)
		if err != nil {
			return err
		}
		if relationship.Status != domain.RelationshipPending {
			return validationError("invalid_transition", "relationship is not PENDING", domain.ErrInvalidRelationshipTransition)
		}
		if relationship.TenantIDSubsidiary != input.SubsidiaryID {
			return validationError("forbidden", "only the subsidiary can cancel the relationship", domain.ErrUnauthorizedRelationshipAction)
		}
		if err := relationship.Cancel(now, input.Reason); err != nil {
			return err
		}
		if err := s.repo.UpdateRelationship(ctx, relationship); err != nil {
			return fmt.Errorf("cancel relationship: %w", err)
		}
		if err := s.repo.CancelActiveInvites(ctx, relationship.ID, input.SubsidiaryID, now); err != nil {
			return fmt.Errorf("cancel relationship invites: %w", err)
		}
		result = relationship
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type RejectRelationshipInput struct {
	RelationshipID uuid.UUID
	HoldingID      uuid.UUID
	Reason         string
}

func (s *TenantRelationshipService) RejectRelationship(
	ctx context.Context,
	input RejectRelationshipInput,
) (*domain.TenantRelationship, error) {
	if input.RelationshipID == uuid.Nil || input.HoldingID == uuid.Nil {
		return nil, validationError("invalid_input", "relationship_id and holding_id are required", domain.ErrInvalidTenantRelationship)
	}
	now := s.now().UTC()

	var result *domain.TenantRelationship
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		relationship, err := s.repo.LockRelationship(ctx, input.RelationshipID)
		if err != nil {
			return err
		}
		if relationship.Status != domain.RelationshipPending {
			return validationError("invalid_transition", "relationship is not PENDING", domain.ErrInvalidRelationshipTransition)
		}
		if relationship.TenantIDHolding != input.HoldingID {
			return validationError("forbidden", "only the holding can reject the relationship", domain.ErrUnauthorizedRelationshipAction)
		}
		if err := relationship.Reject(now, input.Reason); err != nil {
			return err
		}
		if err := s.repo.UpdateRelationship(ctx, relationship); err != nil {
			return fmt.Errorf("reject relationship: %w", err)
		}
		if err := s.repo.CancelActiveInvites(ctx, relationship.ID, input.HoldingID, now); err != nil {
			return fmt.Errorf("cancel relationship invites: %w", err)
		}
		result = relationship
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type ConsumeInviteTokenInput struct {
	InviteCode   string
	SubsidiaryID uuid.UUID
}

func (s *TenantRelationshipService) ConsumeInviteToken(
	ctx context.Context,
	input ConsumeInviteTokenInput,
) (*domain.TenantRelationship, error) {
	inviteCode := strings.TrimSpace(input.InviteCode)
	if inviteCode == "" || input.SubsidiaryID == uuid.Nil {
		return nil, validationError("invalid_invite", "invite code and subsidiary_id are required", domain.ErrInvalidRelationshipInvite)
	}
	hash := sha256.Sum256([]byte(inviteCode))
	now := s.now().UTC()

	var result *domain.TenantRelationship
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		invite, err := s.repo.FindInviteByTokenHash(ctx, hash[:])
		if err != nil {
			return validationError("invite_not_active", "invite is not active", domain.ErrInviteNotActive)
		}
		relationship, err := s.repo.LockRelationship(ctx, invite.RelationshipID)
		if err != nil {
			return err
		}

		if invite.CanceledAt != nil {
			return validationError("invite_not_active", "invite was canceled", domain.ErrInviteNotActive)
		}
		if invite.ConsumedAt != nil {
			// Retry idempotency: a consumed invite for an approved relationship succeeds.
			if relationship.Status == domain.RelationshipApproved {
				result = relationship
				return nil
			}
			return validationError("invite_not_active", "invite was already consumed", domain.ErrInviteNotActive)
		}
		if relationship.Status != domain.RelationshipPending {
			return validationError("invite_not_active", "relationship is not PENDING", domain.ErrInviteNotActive)
		}
		if relationship.TenantIDSubsidiary != input.SubsidiaryID {
			return validationError("forbidden", "only the subsidiary can consume the invite", domain.ErrUnauthorizedRelationshipAction)
		}
		if !invite.ExpiresAt.After(now) {
			return validationError("invite_not_active", "invite has expired", domain.ErrInviteNotActive)
		}

		cycle, err := s.repo.WouldCreateCycle(ctx, relationship.TenantIDHolding, relationship.TenantIDSubsidiary)
		if err != nil {
			return fmt.Errorf("check relationship cycle: %w", err)
		}
		if cycle {
			return validationError("relationship_cycle", "relationship would create a cycle", domain.ErrRelationshipCycleDetected)
		}

		if err := s.repo.MarkInviteConsumed(ctx, invite.ID, now); err != nil {
			return fmt.Errorf("mark invite consumed: %w", err)
		}

		relationship.Status = domain.RelationshipApproved
		relationship.ApprovedAt = &now
		relationship.UpdatedAt = now
		if err := s.repo.UpdateRelationship(ctx, relationship); err != nil {
			return fmt.Errorf("approve relationship: %w", err)
		}

		result = relationship
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type RevokeApprovedRelationshipInput struct {
	RelationshipID uuid.UUID
	HoldingID      uuid.UUID
}

func (s *TenantRelationshipService) RevokeApprovedRelationship(
	ctx context.Context,
	input RevokeApprovedRelationshipInput,
) (*domain.TenantRelationship, error) {
	if input.RelationshipID == uuid.Nil || input.HoldingID == uuid.Nil {
		return nil, validationError("invalid_input", "relationship_id and holding_id are required", domain.ErrInvalidTenantRelationship)
	}
	now := s.now().UTC()

	var result *domain.TenantRelationship
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		relationship, err := s.repo.LockRelationship(ctx, input.RelationshipID)
		if err != nil {
			return err
		}
		if relationship.Status != domain.RelationshipApproved {
			return validationError("invalid_transition", "relationship is not APPROVED", domain.ErrInvalidRelationshipTransition)
		}
		if relationship.TenantIDHolding != input.HoldingID {
			return validationError("forbidden", "only the holding can revoke the relationship", domain.ErrUnauthorizedRelationshipAction)
		}
		relationship.Status = domain.RelationshipCanceled
		relationship.CanceledAt = &now
		relationship.EndsAt = &now
		relationship.UpdatedAt = now
		if err := s.repo.UpdateRelationship(ctx, relationship); err != nil {
			return fmt.Errorf("revoke relationship: %w", err)
		}
		if err := s.repo.CancelActiveInvites(ctx, relationship.ID, input.HoldingID, now); err != nil {
			return fmt.Errorf("cancel relationship invites: %w", err)
		}
		result = relationship
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func newInviteCode() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	code := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(code))
	return code, hash[:], nil
}
