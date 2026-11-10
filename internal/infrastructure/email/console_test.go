package email

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleSenderPrintsToken(t *testing.T) {
	var out bytes.Buffer
	sender := NewConsoleSender(&out)

	require.NoError(t, sender.SendPasswordReset(context.Background(), "user@example.com", "token-123"))
	assert.Contains(t, out.String(), "user@example.com")
	assert.Contains(t, out.String(), "token-123")
}
