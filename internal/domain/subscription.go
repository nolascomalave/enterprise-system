package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "ACTIVE"
	SubscriptionStatusSuspended SubscriptionStatus = "SUSPENDED"
	SubscriptionStatusCancelled SubscriptionStatus = "CANCELLED"
)

func (s SubscriptionStatus) Valid() bool {
	switch s {
	case SubscriptionStatusActive, SubscriptionStatusSuspended, SubscriptionStatusCancelled:
		return true
	default:
		return false
	}
}

var subscriptionSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Subscription represents a tenant. It lives in the global schema and is the
// parent of all tenant-scoped data.
type Subscription struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Status    SubscriptionStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewSubscription(name, slug string, now time.Time) (*Subscription, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate subscription id: %w", err)
	}

	now = now.UTC()
	sub := &Subscription{
		ID:        id,
		Name:      strings.TrimSpace(name),
		Slug:      normalizeSlug(slug),
		Status:    SubscriptionStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := sub.Validate(); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *Subscription) Validate() error {
	if s == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidSubscription)
	}
	if s.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidSubscription)
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidSubscription)
	}
	if !subscriptionSlugPattern.MatchString(s.Slug) {
		return fmt.Errorf("%w: slug must match %s", ErrInvalidSubscription, subscriptionSlugPattern.String())
	}
	if !s.Status.Valid() {
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidSubscription, s.Status)
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrInvalidSubscription)
	}
	if s.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrInvalidSubscription)
	}
	if s.UpdatedAt.Before(s.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidSubscription)
	}
	if s.DeletedAt != nil && s.DeletedAt.Before(s.CreatedAt) {
		return fmt.Errorf("%w: deleted_at cannot be before created_at", ErrInvalidSubscription)
	}
	return nil
}

func (s *Subscription) IsActive() bool {
	return s != nil && s.Status == SubscriptionStatusActive && s.DeletedAt == nil
}

func (s *Subscription) Suspend(now time.Time) error {
	if s.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	if s.Status != SubscriptionStatusActive {
		return fmt.Errorf("%w: cannot suspend a subscription in status %s", ErrInvalidSubscriptionTransition, s.Status)
	}
	s.Status = SubscriptionStatusSuspended
	s.UpdatedAt = now.UTC()
	return nil
}

func (s *Subscription) Cancel(now time.Time) error {
	if s.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	if s.Status == SubscriptionStatusCancelled {
		return fmt.Errorf("%w: subscription is already cancelled", ErrInvalidSubscriptionTransition)
	}
	s.Status = SubscriptionStatusCancelled
	s.UpdatedAt = now.UTC()
	return nil
}

func (s *Subscription) SoftDelete(now time.Time) error {
	if s.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	s.DeletedAt = &now
	s.UpdatedAt = now
	return nil
}

func normalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))

	var b strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen && b.Len() > 0:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}
