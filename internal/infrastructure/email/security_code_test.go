package email

import (
	"bytes"
	"context"
	"testing"

	"enterprise-system/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailSecurityCodeSenderSendsEmail(t *testing.T) {
	var out bytes.Buffer
	sender := NewEmailSecurityCodeSender(&out)

	err := sender.SendCode(context.Background(), "user@example.com", "123456", domain.ContactTypeEmail)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "user@example.com")
	assert.Contains(t, out.String(), "123456")
}

func TestEmailSecurityCodeSenderRejectsNonEmailChannel(t *testing.T) {
	var out bytes.Buffer
	sender := NewEmailSecurityCodeSender(&out)

	err := sender.SendCode(context.Background(), "+15555550100", "123456", domain.ContactTypePhone)
	require.ErrorIs(t, err, domain.ErrUnsupportedSecurityCodeChannel)
	assert.Empty(t, out.String())
}

func TestEmailSecurityCodeSenderRejectsEmptyDestination(t *testing.T) {
	var out bytes.Buffer
	sender := NewEmailSecurityCodeSender(&out)

	err := sender.SendCode(context.Background(), "  ", "123456", domain.ContactTypeEmail)
	require.Error(t, err)
	assert.Empty(t, out.String())
}
