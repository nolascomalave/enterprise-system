package repository

import (
	"context"

	"enterprise-system/ent"
	"enterprise-system/ent/userapppreference"
	"enterprise-system/ent/usercredentials"
	"enterprise-system/internal/domain"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

var _ domain.UserPreferencesRepository = (*UserPreferencesRepository)(nil)

type UserPreferencesRepository struct {
	client *ent.Client
}

func NewUserPreferencesRepository(client *ent.Client) *UserPreferencesRepository {
	return &UserPreferencesRepository{client: client}
}

func (r *UserPreferencesRepository) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if r == nil {
		return errNilClient
	}
	return execute(ctx, r.client, fn)
}

// credentialsID resolves the JWT party_id to the tenant-scoped credentials row.
func credentialsID(ctx context.Context, client *ent.Client, tenantID, userID uuid.UUID) (uuid.UUID, error) {
	row, err := client.UserCredentials.Query().
		Where(
			usercredentials.TenantID(tenantID),
			usercredentials.PartyID(userID),
			usercredentials.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return uuid.Nil, domain.ErrPreferencesNotFound
		}
		return uuid.Nil, repoError(err)
	}
	return row.ID, nil
}

func (r *UserPreferencesRepository) Get(ctx context.Context, tenantID, userID, appClientID uuid.UUID) (*domain.UserPreferences, error) {
	var result *domain.UserPreferences
	err := r.withClient(ctx, func(client *ent.Client) error {
		credentialsID, err := credentialsID(ctx, client, tenantID, userID)
		if err != nil {
			return err
		}
		row, err := client.UserAppPreference.Query().
			Where(
				userapppreference.TenantID(tenantID),
				userapppreference.UserCredentialsID(credentialsID),
				userapppreference.ClientApplicationID(appClientID),
				userapppreference.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toDomainUserAppPreference(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPreferencesNotFound
	}
	return result, err
}

func (r *UserPreferencesRepository) Upsert(ctx context.Context, prefs *domain.UserPreferences) error {
	if prefs == nil {
		return domain.ErrInvalidPreferences
	}
	return r.withClient(ctx, func(client *ent.Client) error {
		credentialsID, err := credentialsID(ctx, client, prefs.TenantID, prefs.UserID)
		if err != nil {
			return err
		}
		err = client.UserAppPreference.Create().
			SetTenantID(prefs.TenantID).
			SetUserCredentialsID(credentialsID).
			SetClientApplicationID(prefs.ClientApplicationID).
			SetPreferences(prefs.Preferences).
			SetCreatedAt(prefs.CreatedAt).
			SetUpdatedAt(prefs.UpdatedAt).
			OnConflict(
				sql.ConflictColumns(
					userapppreference.FieldTenantID,
					userapppreference.FieldUserCredentialsID,
					userapppreference.FieldClientApplicationID,
				),
				sql.ConflictWhere(sql.IsNull(userapppreference.FieldDeletedAt)),
			).
			UpdateNewValues().
			Exec(ctx)
		return repoError(err)
	})
}

func toDomainUserAppPreference(e *ent.UserAppPreference) *domain.UserPreferences {
	if e == nil {
		return nil
	}
	return &domain.UserPreferences{
		ID:                  e.ID,
		TenantID:            e.TenantID,
		UserCredentialsID:   e.UserCredentialsID,
		ClientApplicationID: e.ClientApplicationID,
		Preferences:         e.Preferences,
		CreatedAt:           e.CreatedAt,
		UpdatedAt:           e.UpdatedAt,
		DeletedAt:           e.DeletedAt,
	}
}
