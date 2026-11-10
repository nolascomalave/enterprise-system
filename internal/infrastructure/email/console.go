package email

import (
	"context"
	"fmt"
	"io"
	"os"

	"enterprise-system/internal/auth"
)

var _ auth.EmailSender = (*ConsoleSender)(nil)

// ConsoleSender is a development-only EmailSender. It prints the reset token to
// stdout so local flows can be tested without an email provider.
type ConsoleSender struct {
	out io.Writer
}

func NewConsoleSender(out io.Writer) *ConsoleSender {
	if out == nil {
		out = os.Stdout
	}
	return &ConsoleSender{out: out}
}

func (s *ConsoleSender) SendPasswordReset(ctx context.Context, to, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(s.out, "password reset for %s: token=%s\n", to, token)
	return err
}
