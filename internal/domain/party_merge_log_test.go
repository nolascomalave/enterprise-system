package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPartyMergeLog(t *testing.T) {
	now := testNow()
	sourceID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001")
	targetID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002")
	actorID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000003")

	entry, err := NewPartyMergeLog(testTenantID, sourceID, targetID, &actorID, now)

	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, uuid.Version(7), entry.ID.Version())
	assert.Equal(t, testTenantID, entry.TenantID)
	assert.Equal(t, sourceID, entry.SourcePartyID)
	assert.Equal(t, targetID, entry.TargetPartyID)
	assert.Equal(t, now, entry.MergedAt)
	require.NotNil(t, entry.MergedBy)
	assert.Equal(t, actorID, *entry.MergedBy)
}

func TestPartyMergeLogValidation(t *testing.T) {
	now := testNow()
	sourceID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001")
	targetID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002")

	tests := []struct {
		name     string
		tenantID uuid.UUID
		sourceID uuid.UUID
		targetID uuid.UUID
		mergedBy *uuid.UUID
	}{
		{name: "nil tenant", tenantID: uuid.Nil, sourceID: sourceID, targetID: targetID},
		{name: "nil source", tenantID: testTenantID, sourceID: uuid.Nil, targetID: targetID},
		{name: "nil target", tenantID: testTenantID, sourceID: sourceID, targetID: uuid.Nil},
		{name: "same source and target", tenantID: testTenantID, sourceID: sourceID, targetID: sourceID},
		{name: "nil merged_by uuid", tenantID: testTenantID, sourceID: sourceID, targetID: targetID, mergedBy: &uuid.Nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := NewPartyMergeLog(tt.tenantID, tt.sourceID, tt.targetID, tt.mergedBy, now)

			require.ErrorIs(t, err, ErrInvalidPartyMerge)
			assert.Nil(t, entry)
		})
	}
}
