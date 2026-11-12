package repository

import (
	"context"
	"database/sql"
	"errors"

	"enterprise-system/ent"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

var _ domain.ClientApplicationRepository = (*ClientApplicationRepository)(nil)

type ClientApplicationRepository struct {
	client *ent.Client
}

func NewClientApplicationRepository(client *ent.Client) *ClientApplicationRepository {
	return &ClientApplicationRepository{client: client}
}

// Create uses raw SQL because scopes is a PostgreSQL text[] and Ent's slice
// field uses JSON semantics. pq.Array handles the array encoding.
func (r *ClientApplicationRepository) Create(ctx context.Context, application *domain.ClientApplication) error {
	if application == nil {
		return errors.New("repository: nil client application")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		var tenantID any
		if application.TenantID != nil {
			tenantID = *application.TenantID
		}
		_, err := client.ExecContext(ctx, `
			INSERT INTO global.client_applications
				(id, tenant_id, name, client_id, client_secret_hash, scopes,
				 client_type, application_scope, is_active, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			application.ID, tenantID, application.Name, application.ClientID,
			application.ClientSecretHash, pq.Array(application.Scopes),
			string(application.ClientType), string(application.ApplicationScope),
			application.IsActive, application.CreatedAt, application.UpdatedAt, application.DeletedAt,
		)
		return repoError(err)
	})
}

func (r *ClientApplicationRepository) FindByClientID(ctx context.Context, clientID string) (*domain.ClientApplication, error) {
	var result *domain.ClientApplication
	err := execute(ctx, r.client, func(client *ent.Client) error {
		rows, err := client.QueryContext(ctx, `
			SELECT id, tenant_id, name, client_id, client_secret_hash, scopes,
			       client_type, application_scope, is_active, created_at, updated_at, deleted_at
			  FROM global.client_applications
			 WHERE client_id = $1 AND is_active AND deleted_at IS NULL`, clientID)
		if err != nil {
			return repoError(err)
		}
		defer rows.Close()
		if !rows.Next() {
			return sql.ErrNoRows
		}
		var scopes pq.StringArray
		var tenant sql.NullString
		var clientType, applicationScope string
		result = &domain.ClientApplication{}
		if err := rows.Scan(
			&result.ID, &tenant, &result.Name, &result.ClientID,
			&result.ClientSecretHash, &scopes, &clientType, &applicationScope,
			&result.IsActive, &result.CreatedAt, &result.UpdatedAt, &result.DeletedAt,
		); err != nil {
			return repoError(err)
		}
		result.Scopes = []string(scopes)
		result.ClientType = domain.ClientType(clientType)
		result.ApplicationScope = domain.ApplicationScope(applicationScope)
		if tenant.Valid {
			parsed, err := uuid.Parse(tenant.String)
			if err != nil {
				return repoError(err)
			}
			result.TenantID = &parsed
		}
		return rows.Err()
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrClientApplicationNotFound
	}
	return result, err
}
