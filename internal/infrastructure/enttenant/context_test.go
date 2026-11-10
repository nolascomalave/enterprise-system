package enttenant

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionContextRoundTrip(t *testing.T) {
	want := SessionContext{
		TenantID:  uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001"),
		UserID:    uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002"),
		SessionID: uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000003"),
	}

	got, ok := SessionContextFrom(WithSessionContext(context.Background(), want))

	require.True(t, ok)
	assert.Equal(t, want, got)
	assert.False(t, got.IsZero())
}

func TestSessionContextFromEmpty(t *testing.T) {
	got, ok := SessionContextFrom(context.Background())

	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestSessionContextIsZero(t *testing.T) {
	assert.True(t, SessionContext{}.IsZero())
	assert.False(t, SessionContext{TenantID: uuid.New()}.IsZero())
}
