package repository

import (
	"errors"
	"testing"

	"enterprise-system/internal/domain"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestTranslateRelationshipCycleError(t *testing.T) {
	require.ErrorIs(t,
		translateRelationshipCycleError(&pq.Error{Code: "42501"}),
		domain.ErrUnauthorizedRelationshipAction)
	require.ErrorIs(t,
		translateRelationshipCycleError(&pq.Error{Code: "54000"}),
		domain.ErrRelationshipGraphTooDeep)

	err := translateRelationshipCycleError(errors.New("unexpected"))
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrRelationshipGraphTooDeep)
}
