package repository

import (
	"context"

	"enterprise-system/ent"
	"enterprise-system/ent/party"
	"github.com/google/uuid"
)

// DocumentOriginChecker validates document origins against the tenant.
type DocumentOriginChecker struct {
	client *ent.Client
}

func NewDocumentOriginChecker(client *ent.Client) *DocumentOriginChecker {
	return &DocumentOriginChecker{client: client}
}

// OriginBelongsToTenant currently validates the parties origin table. Other
// origin tables keep their previous behavior until they receive an explicit
// ownership rule.
func (c *DocumentOriginChecker) OriginBelongsToTenant(
	ctx context.Context,
	tenantID uuid.UUID,
	originTable string,
	originID uuid.UUID,
) (bool, error) {
	if originTable != "parties" {
		return true, nil
	}
	var exists bool
	err := execute(ctx, c.client, func(client *ent.Client) error {
		ok, err := client.Party.Query().
			Where(
				party.ID(originID),
				party.TenantID(tenantID),
				party.DeletedAtIsNil(),
			).
			Exist(ctx)
		if err != nil {
			return repoError(err)
		}
		exists = ok
		return nil
	})
	return exists, err
}
