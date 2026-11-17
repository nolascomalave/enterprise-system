package email

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"enterprise-system/internal/domain"
)

var _ domain.SecurityCodeSender = (*EmailSecurityCodeSender)(nil)

// EmailSecurityCodeSender delivers one-time security codes through the email
// channel. It reuses the same console-style delivery used by ConsoleSender and
// can be swapped for an SMTP/provider adapter without changing the use case.
type EmailSecurityCodeSender struct {
	out io.Writer
}

func NewEmailSecurityCodeSender(out io.Writer) *EmailSecurityCodeSender {
	if out == nil {
		out = os.Stdout
	}
	return &EmailSecurityCodeSender{out: out}
}

func (s *EmailSecurityCodeSender) SendCode(ctx context.Context, destination, code string, contactType domain.ContactType) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if contactType != domain.ContactTypeEmail {
		return domain.ErrUnsupportedSecurityCodeChannel
	}
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return fmt.Errorf("email security code: destination is required")
	}
	_, err := fmt.Fprintf(s.out, "security code for %s: code=%s\n", destination, code)
	return err
}
