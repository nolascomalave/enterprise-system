package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSubscription(t *testing.T) {
	now := testNow()

	sub, err := NewSubscription("  Acme Corp  ", "  Acme-Corp  ", now)

	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.NotEqual(t, uuid.Nil, sub.ID)
	assert.Equal(t, uuid.Version(7), sub.ID.Version(), "el id debe ser UUIDv7")
	assert.Equal(t, "Acme Corp", sub.Name)
	assert.Equal(t, "acme-corp", sub.Slug)
	assert.Equal(t, SubscriptionStatusActive, sub.Status)
	assert.Equal(t, now, sub.CreatedAt)
	assert.Equal(t, now, sub.UpdatedAt)
	assert.Nil(t, sub.DeletedAt)
	assert.True(t, sub.IsActive())
}

func TestNewSubscriptionValidation(t *testing.T) {
	now := testNow()

	tests := []struct {
		name    string
		subName string
		slug    string
	}{
		{name: "empty name", subName: "", slug: "acme"},
		{name: "blank name", subName: "   ", slug: "acme"},
		{name: "empty slug", subName: "Acme", slug: ""},
		{name: "slug without alphanumeric", subName: "Acme", slug: "---"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := NewSubscription(tt.subName, tt.slug, now)

			require.ErrorIs(t, err, ErrInvalidSubscription)
			assert.Nil(t, sub)
		})
	}
}

func TestSubscriptionSlugNormalization(t *testing.T) {
	now := testNow()

	tests := map[string]string{
		"  Acme Corp  ": "acme-corp",
		"acme_corp":     "acme-corp",
		"ACME--CORP":    "acme-corp",
		"acme___corp__": "acme-corp",
		"acme-123":      "acme-123",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			sub, err := NewSubscription("Acme", input, now)

			require.NoError(t, err)
			assert.Equal(t, want, sub.Slug)
		})
	}
}

func TestSubscriptionStatusTransitions(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	sub, err := NewSubscription("Acme", "acme", now)
	require.NoError(t, err)

	require.NoError(t, sub.Suspend(later))
	assert.Equal(t, SubscriptionStatusSuspended, sub.Status)
	assert.Equal(t, later, sub.UpdatedAt)
	assert.False(t, sub.IsActive())

	require.ErrorIs(t, sub.Suspend(later), ErrInvalidSubscriptionTransition)

	require.NoError(t, sub.Cancel(later.Add(time.Hour)))
	assert.Equal(t, SubscriptionStatusCancelled, sub.Status)
	assert.False(t, sub.IsActive())

	require.ErrorIs(t, sub.Cancel(later.Add(2*time.Hour)), ErrInvalidSubscriptionTransition)
	require.ErrorIs(t, sub.Suspend(later.Add(2*time.Hour)), ErrInvalidSubscriptionTransition)
}

func TestSubscriptionSoftDelete(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	sub, err := NewSubscription("Acme", "acme", now)
	require.NoError(t, err)

	require.NoError(t, sub.SoftDelete(later))
	require.NotNil(t, sub.DeletedAt)
	assert.Equal(t, later, *sub.DeletedAt)
	assert.Equal(t, later, sub.UpdatedAt)
	assert.False(t, sub.IsActive())

	require.ErrorIs(t, sub.SoftDelete(later.Add(time.Hour)), ErrAlreadyDeleted)
}
