package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// PreferencesService manages per-user, per-client-application UI preferences.
type PreferencesService struct {
	repo domain.UserPreferencesRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewPreferencesService(repo domain.UserPreferencesRepository, tx TransactionManager, now func() time.Time) *PreferencesService {
	if now == nil {
		now = time.Now
	}
	return &PreferencesService{repo: repo, tx: tx, now: now}
}

func defaultUserPreferences() map[string]any {
	return map[string]any{
		"theme":             "system",
		"locale":            "es",
		"sidebar_collapsed": false,
	}
}

// GetUserPreferences returns stored preferences or a non-persisted default
// object when the user has not saved preferences for this client application.
func (s *PreferencesService) GetUserPreferences(ctx context.Context, tenantID, userID, appClientID uuid.UUID) (*domain.UserPreferences, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil || appClientID == uuid.Nil {
		return nil, validationError("invalid_preferences_key", "tenant_id, user_id and app_client_id are required", domain.ErrInvalidPreferences)
	}

	prefs, err := readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.UserPreferences, error) {
		return s.repo.Get(ctx, tenantID, userID, appClientID)
	})
	if errors.Is(err, domain.ErrPreferencesNotFound) {
		return &domain.UserPreferences{
			TenantID:            tenantID,
			UserID:              userID,
			ClientApplicationID: appClientID,
			Preferences:         defaultUserPreferences(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user preferences: %w", err)
	}
	return prefs, nil
}

// UpdateUserPreferences replaces the JSON preferences object for the current
// user and client application using an upsert.
func (s *PreferencesService) UpdateUserPreferences(
	ctx context.Context,
	tenantID, userID, appClientID uuid.UUID,
	preferences map[string]any,
) error {
	if tenantID == uuid.Nil || userID == uuid.Nil || appClientID == uuid.Nil {
		return validationError("invalid_preferences_key", "tenant_id, user_id and app_client_id are required", domain.ErrInvalidPreferences)
	}
	if preferences == nil {
		return validationError("invalid_preferences", "preferences object is required", domain.ErrInvalidPreferences)
	}

	now := s.now().UTC()
	prefs := &domain.UserPreferences{
		TenantID:            tenantID,
		UserID:              userID,
		ClientApplicationID: appClientID,
		Preferences:         preferences,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.repo.Upsert(ctx, prefs); err != nil {
			return fmt.Errorf("upsert user preferences: %w", err)
		}
		return nil
	})
}
