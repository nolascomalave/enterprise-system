package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	otpTTL      = 5 * time.Minute
	otpCooldown = 60 * time.Second
	otpCodeLen  = 6
)

// OTPManager stores and consumes one-time security codes in Redis. Only the
// SHA-256 hash of a code is persisted; the plaintext code is returned once to
// the caller so it can be delivered through a SecurityCodeSender.
type OTPManager struct {
	client *redis.Client
}

func NewOTPManager(client *redis.Client) *OTPManager {
	return &OTPManager{client: client}
}

func otpKey(userID uuid.UUID, action string) string {
	return "otp:" + userID.String() + ":" + action
}

func otpCooldownKey(userID uuid.UUID, action string) string {
	return "otp_cooldown:" + userID.String() + ":" + action
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func generateSixDigits() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// Generate creates a new 6-digit code, stores only its hash and applies the
// per-user/per-action cooldown. Any Redis failure fails closed.
func (m *OTPManager) Generate(ctx context.Context, userID uuid.UUID, action string) (string, error) {
	if m == nil || m.client == nil {
		return "", domain.ErrOTPServiceUnavailable
	}
	action = strings.TrimSpace(action)
	if userID == uuid.Nil || action == "" {
		return "", fmt.Errorf("otp: user id and action are required")
	}

	cooldownKey := otpCooldownKey(userID, action)
	acquired, err := m.client.SetNX(ctx, cooldownKey, "1", otpCooldown).Result()
	if err != nil {
		return "", domain.ErrOTPServiceUnavailable
	}
	if !acquired {
		return "", domain.ErrOTPCooldown
	}

	code, err := generateSixDigits()
	if err != nil {
		m.releaseCooldown(ctx, cooldownKey)
		return "", fmt.Errorf("otp: generate code: %w", err)
	}
	if err := m.client.Set(ctx, otpKey(userID, action), hashCode(code), otpTTL).Err(); err != nil {
		// A transient Redis failure must not lock the user out for a minute.
		m.releaseCooldown(ctx, cooldownKey)
		return "", domain.ErrOTPServiceUnavailable
	}
	return code, nil
}

func (m *OTPManager) releaseCooldown(ctx context.Context, key string) {
	_ = m.client.Del(context.WithoutCancel(ctx), key).Err()
}

// Verify consumes the stored code atomically with GET+DEL inside a Redis
// transaction. A wrong code also consumes the stored hash, so an attacker only
// gets a single attempt per generated code.
func (m *OTPManager) Verify(ctx context.Context, userID uuid.UUID, action, code string) error {
	if m == nil || m.client == nil {
		return domain.ErrOTPServiceUnavailable
	}
	action = strings.TrimSpace(action)
	code = strings.TrimSpace(code)
	if userID == uuid.Nil || action == "" || code == "" {
		return domain.ErrInvalidSecurityCode
	}

	key := otpKey(userID, action)
	var getCmd *redis.StringCmd
	_, err := m.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		getCmd = pipe.Get(ctx, key)
		pipe.Del(ctx, key)
		return nil
	})
	if err != nil && err != redis.Nil {
		return domain.ErrOTPServiceUnavailable
	}
	if getCmd == nil {
		return domain.ErrOTPServiceUnavailable
	}
	stored, err := getCmd.Result()
	if err != nil {
		if err == redis.Nil {
			return domain.ErrInvalidSecurityCode
		}
		return domain.ErrOTPServiceUnavailable
	}

	expected := hashCode(code)
	if subtle.ConstantTimeCompare([]byte(stored), []byte(expected)) != 1 {
		return domain.ErrInvalidSecurityCode
	}
	return nil
}

// Invalidate removes a pending code, for example after a successful password
// change or when the account changes its security preferences.
func (m *OTPManager) Invalidate(ctx context.Context, userID uuid.UUID, action string) error {
	if m == nil || m.client == nil {
		return domain.ErrOTPServiceUnavailable
	}
	action = strings.TrimSpace(action)
	if userID == uuid.Nil || action == "" {
		return nil
	}
	if err := m.client.Del(ctx, otpKey(userID, action)).Err(); err != nil {
		return domain.ErrOTPServiceUnavailable
	}
	return nil
}

var _ domain.SecurityCodeManager = (*OTPManager)(nil)
