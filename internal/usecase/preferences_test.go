package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePreferencesRepository struct {
	getResult   *domain.UserPreferences
	getErr      error
	upsertErr   error
	upserted    *domain.UserPreferences
	getCalls    int
	upsertCalls int
}

func (r *fakePreferencesRepository) Get(ctx context.Context, tenantID, userID, appClientID uuid.UUID) (*domain.UserPreferences, error) {
	r.getCalls++
	return r.getResult, r.getErr
}

func (r *fakePreferencesRepository) Upsert(ctx context.Context, prefs *domain.UserPreferences) error {
	r.upsertCalls++
	r.upserted = prefs
	return r.upsertErr
}

func preferencesTestService(repo *fakePreferencesRepository) *PreferencesService {
	return NewPreferencesService(repo, newFakeTxManager(), func() time.Time {
		return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	})
}

func TestGetUserPreferencesReturnsDefaultsWhenNotFound(t *testing.T) {
	repo := &fakePreferencesRepository{getErr: domain.ErrPreferencesNotFound}
	service := preferencesTestService(repo)
	tenantID, userID, appClientID := uuid.New(), uuid.New(), uuid.New()

	prefs, err := service.GetUserPreferences(context.Background(), tenantID, userID, appClientID)

	require.NoError(t, err)
	require.NotNil(t, prefs)
	assert.Equal(t, tenantID, prefs.TenantID)
	assert.Equal(t, appClientID, prefs.ClientApplicationID)
	assert.Equal(t, "system", prefs.Preferences["theme"])
	assert.Equal(t, "es", prefs.Preferences["locale"])
	assert.Equal(t, false, prefs.Preferences["sidebar_collapsed"])
	assert.Equal(t, 1, repo.getCalls)
	assert.Equal(t, 0, repo.upsertCalls, "defaults must not be persisted")
}

func TestGetUserPreferencesReturnsStoredPreferences(t *testing.T) {
	stored := &domain.UserPreferences{
		ID: uuid.New(), TenantID: uuid.New(), UserCredentialsID: uuid.New(),
		ClientApplicationID: uuid.New(),
		Preferences:         map[string]any{"theme": "dark"},
	}
	repo := &fakePreferencesRepository{getResult: stored}
	service := preferencesTestService(repo)

	prefs, err := service.GetUserPreferences(context.Background(), stored.TenantID, uuid.New(), stored.ClientApplicationID)

	require.NoError(t, err)
	assert.Equal(t, "dark", prefs.Preferences["theme"])
}

func TestUpdateUserPreferencesRejectsNilPreferences(t *testing.T) {
	service := preferencesTestService(&fakePreferencesRepository{})

	err := service.UpdateUserPreferences(context.Background(), uuid.New(), uuid.New(), uuid.New(), nil)

	require.ErrorIs(t, err, domain.ErrInvalidPreferences)
}

func TestUpdateUserPreferencesUpserts(t *testing.T) {
	repo := &fakePreferencesRepository{}
	service := preferencesTestService(repo)
	tenantID, userID, appClientID := uuid.New(), uuid.New(), uuid.New()
	values := map[string]any{"theme": "dark", "sidebar_collapsed": true}

	err := service.UpdateUserPreferences(context.Background(), tenantID, userID, appClientID, values)

	require.NoError(t, err)
	require.Equal(t, 1, repo.upsertCalls)
	require.NotNil(t, repo.upserted)
	assert.Equal(t, tenantID, repo.upserted.TenantID)
	assert.Equal(t, appClientID, repo.upserted.ClientApplicationID)
	assert.Equal(t, values, repo.upserted.Preferences)
}

func TestUpdateUserPreferencesPropagatesRepositoryError(t *testing.T) {
	repo := &fakePreferencesRepository{upsertErr: errors.New("database down")}
	service := preferencesTestService(repo)

	err := service.UpdateUserPreferences(context.Background(), uuid.New(), uuid.New(), uuid.New(), map[string]any{"theme": "dark"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database down")
}
