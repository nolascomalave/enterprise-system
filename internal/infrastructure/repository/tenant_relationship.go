package repository

import (
	"context"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/subscription"
	"enterprise-system/ent/tenantrelationship"
	"enterprise-system/ent/tenantrelationshipinvite"
	"enterprise-system/ent/tenanttransfer"
	"enterprise-system/internal/domain"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

var _ domain.TenantRelationshipRepository = (*TenantRelationshipRepository)(nil)

type TenantRelationshipRepository struct {
	client *ent.Client
}

func NewTenantRelationshipRepository(client *ent.Client) *TenantRelationshipRepository {
	return &TenantRelationshipRepository{client: client}
}

func (r *TenantRelationshipRepository) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if r == nil {
		return errNilClient
	}
	return execute(ctx, r.client, fn)
}

func (r *TenantRelationshipRepository) TenantActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	var active bool
	err := r.withClient(ctx, func(client *ent.Client) error {
		ok, err := client.Subscription.Query().
			Where(subscription.ID(tenantID), subscription.DeletedAtIsNil()).
			Exist(ctx)
		if err != nil {
			return repoError(err)
		}
		active = ok
		return nil
	})
	return active, err
}

func (r *TenantRelationshipRepository) CreateRelationship(ctx context.Context, relationship *domain.TenantRelationship) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.TenantRelationship.Create().
			SetID(relationship.ID).
			SetTenantIDHolding(relationship.TenantIDHolding).
			SetTenantIDSubsidiary(relationship.TenantIDSubsidiary).
			SetRelationshipType(relationship.RelationshipType).
			SetStatus(tenantrelationship.Status(relationship.Status)).
			SetRequestedAt(relationship.RequestedAt).
			SetCreatedAt(relationship.CreatedAt).
			SetUpdatedAt(relationship.UpdatedAt)
		if relationship.RejectionReason != "" {
			create.SetRejectionReason(relationship.RejectionReason)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *TenantRelationshipRepository) LockRelationship(ctx context.Context, relationshipID uuid.UUID) (*domain.TenantRelationship, error) {
	var result *domain.TenantRelationship
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.TenantRelationship.Query().
			Where(tenantrelationship.ID(relationshipID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrTenantRelationshipNotFound
			}
			return repoError(err)
		}
		result = toDomainTenantRelationship(row)
		return nil
	})
	return result, err
}

// WouldCreateCycle reports whether adding holding -> subsidiary to the
// APPROVED graph would close a cycle. The recursive CTE is bounded to depth 20.
func (r *TenantRelationshipRepository) WouldCreateCycle(ctx context.Context, holdingID, subsidiaryID uuid.UUID) (bool, error) {
	var cycle bool
	err := r.withClient(ctx, func(client *ent.Client) error {
		exists, err := client.TenantRelationship.Query().
			Where(func(s *sql.Selector) {
				s.Where(sql.ExprP(`EXISTS (
					WITH RECURSIVE descendants AS (
						SELECT tenant_id_subsidiary AS tenant_id, 1 AS depth
						  FROM global.tenant_relationships
						 WHERE tenant_id_holding = $1 AND status = 'APPROVED'
						UNION ALL
						SELECT r.tenant_id_subsidiary, d.depth + 1
						  FROM global.tenant_relationships r
						  JOIN descendants d ON r.tenant_id_holding = d.tenant_id
						 WHERE r.status = 'APPROVED' AND d.depth < 20
					)
					SELECT 1 FROM descendants WHERE tenant_id = $2
				)`, subsidiaryID, holdingID))
			}).
			Exist(ctx)
		if err != nil {
			return repoError(err)
		}
		cycle = exists
		return nil
	})
	return cycle, err
}

func (r *TenantRelationshipRepository) FindRelationship(ctx context.Context, relationshipID uuid.UUID) (*domain.TenantRelationship, error) {
	var result *domain.TenantRelationship
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.TenantRelationship.Get(ctx, relationshipID)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrTenantRelationshipNotFound
			}
			return repoError(err)
		}
		result = toDomainTenantRelationship(row)
		return nil
	})
	return result, err
}

func (r *TenantRelationshipRepository) UpdateRelationship(ctx context.Context, relationship *domain.TenantRelationship) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.TenantRelationship.UpdateOneID(relationship.ID).
			SetStatus(tenantrelationship.Status(relationship.Status)).
			SetUpdatedAt(relationship.UpdatedAt)
		if relationship.ApprovedAt != nil {
			update.SetApprovedAt(*relationship.ApprovedAt)
		} else {
			update.ClearApprovedAt()
		}
		if relationship.RejectedAt != nil {
			update.SetRejectedAt(*relationship.RejectedAt)
		} else {
			update.ClearRejectedAt()
		}
		if relationship.CanceledAt != nil {
			update.SetCanceledAt(*relationship.CanceledAt)
		} else {
			update.ClearCanceledAt()
		}
		if relationship.EndsAt != nil {
			update.SetEndsAt(*relationship.EndsAt)
		} else {
			update.ClearEndsAt()
		}
		if relationship.RejectionReason != "" {
			update.SetRejectionReason(relationship.RejectionReason)
		} else {
			update.ClearRejectionReason()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *TenantRelationshipRepository) CreateInvite(ctx context.Context, invite *domain.TenantRelationshipInvite) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.TenantRelationshipInvite.Create().
			SetID(invite.ID).
			SetRelationshipID(invite.RelationshipID).
			SetTokenHash(invite.TokenHash).
			SetExpiresAt(invite.ExpiresAt).
			SetCreatedAt(invite.CreatedAt).
			SetUpdatedAt(invite.UpdatedAt)
		if invite.CanceledAt != nil {
			create.SetCanceledAt(*invite.CanceledAt)
		}
		if invite.CanceledByTenantID != nil {
			create.SetCanceledByTenantID(*invite.CanceledByTenantID)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *TenantRelationshipRepository) FindInviteByTokenHash(ctx context.Context, tokenHash []byte) (*domain.TenantRelationshipInvite, error) {
	var result *domain.TenantRelationshipInvite
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.TenantRelationshipInvite.Query().
			Where(tenantrelationshipinvite.TokenHash(tokenHash)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrRelationshipInviteNotFound
			}
			return repoError(err)
		}
		result = toDomainTenantRelationshipInvite(row)
		return nil
	})
	return result, err
}

func (r *TenantRelationshipRepository) CancelActiveInvites(ctx context.Context, relationshipID, canceledByTenantID uuid.UUID, at time.Time) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.TenantRelationshipInvite.Update().
			Where(
				tenantrelationshipinvite.RelationshipID(relationshipID),
				tenantrelationshipinvite.CanceledAtIsNil(),
				tenantrelationshipinvite.ConsumedAtIsNil(),
			).
			SetCanceledAt(at).
			SetCanceledByTenantID(canceledByTenantID).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
}

func (r *TenantRelationshipRepository) MarkInviteConsumed(ctx context.Context, inviteID uuid.UUID, at time.Time) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.TenantRelationshipInvite.UpdateOneID(inviteID).
			SetConsumedAt(at).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
}

func (r *TenantRelationshipRepository) CreateTransfer(ctx context.Context, transfer *domain.TenantTransfer) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.TenantTransfer.Create().
			SetID(transfer.ID).
			SetRelationshipID(transfer.RelationshipID).
			SetRequestedByTenantID(transfer.RequestedByTenantID).
			SetStatus(tenanttransfer.Status(transfer.Status)).
			SetRequestedAt(transfer.RequestedAt).
			SetCreatedAt(transfer.CreatedAt).
			SetUpdatedAt(transfer.UpdatedAt)
		if transfer.TransferReason != "" {
			create.SetTransferReason(transfer.TransferReason)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *TenantRelationshipRepository) FindTransfer(ctx context.Context, transferID uuid.UUID) (*domain.TenantTransfer, error) {
	var result *domain.TenantTransfer
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.TenantTransfer.Get(ctx, transferID)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrTenantTransferNotFound
			}
			return repoError(err)
		}
		result = toDomainTenantTransfer(row)
		return nil
	})
	return result, err
}

func toDomainTenantRelationship(row *ent.TenantRelationship) *domain.TenantRelationship {
	result := &domain.TenantRelationship{
		ID:                 row.ID,
		TenantIDHolding:    row.TenantIDHolding,
		TenantIDSubsidiary: row.TenantIDSubsidiary,
		RelationshipType:   row.RelationshipType,
		Status:             domain.RelationshipStatus(row.Status),
		RequestedAt:        row.RequestedAt,
		RejectionReason:    row.RejectionReason,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
	if row.ApprovedAt != nil {
		result.ApprovedAt = row.ApprovedAt
	}
	if row.RejectedAt != nil {
		result.RejectedAt = row.RejectedAt
	}
	if row.CanceledAt != nil {
		result.CanceledAt = row.CanceledAt
	}
	if row.EndsAt != nil {
		result.EndsAt = row.EndsAt
	}
	return result
}

func toDomainTenantRelationshipInvite(row *ent.TenantRelationshipInvite) *domain.TenantRelationshipInvite {
	return &domain.TenantRelationshipInvite{
		ID:                 row.ID,
		RelationshipID:     row.RelationshipID,
		TokenHash:          row.TokenHash,
		ExpiresAt:          row.ExpiresAt,
		ConsumedAt:         row.ConsumedAt,
		CanceledAt:         row.CanceledAt,
		CanceledByTenantID: row.CanceledByTenantID,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func toDomainTenantTransfer(row *ent.TenantTransfer) *domain.TenantTransfer {
	return &domain.TenantTransfer{
		ID:                  row.ID,
		RelationshipID:      row.RelationshipID,
		RequestedByTenantID: row.RequestedByTenantID,
		Status:              string(row.Status),
		RequestedAt:         row.RequestedAt,
		StartedAt:           row.StartedAt,
		CompletedAt:         row.CompletedAt,
		CanceledAt:          row.CanceledAt,
		CanceledByTenantID:  row.CanceledByTenantID,
		TransferReason:      row.TransferReason,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}
