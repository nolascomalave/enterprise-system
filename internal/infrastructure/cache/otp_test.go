package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newOTPTestManager(t *testing.T) (*OTPManager, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return NewOTPManager(client), server
}

func otpHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func TestOTPManagerStoresOnlyHashWithTTL(t *testing.T) {
	manager, server := newOTPTestManager(t)
	userID := uuid.New()

	code, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)
	require.Len(t, code, 6)

	stored, err := server.Get("otp:" + userID.String() + ":CHANGE_PASSWORD")
	require.NoError(t, err)
	require.NotEqual(t, code, stored, "plaintext code must never be stored")
	require.Equal(t, otpHash(code), stored)

	require.Equal(t, 5*time.Minute, server.TTL("otp:"+userID.String()+":CHANGE_PASSWORD"))
	require.Equal(t, 60*time.Second, server.TTL("otp_cooldown:"+userID.String()+":CHANGE_PASSWORD"))
}

func TestOTPManagerCooldownRejectsSecondRequest(t *testing.T) {
	manager, server := newOTPTestManager(t)
	userID := uuid.New()

	_, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)

	_, err = manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.ErrorIs(t, err, domain.ErrOTPCooldown)

	server.FastForward(61 * time.Second)
	_, err = manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)
}

func TestOTPManagerCooldownIsPerAction(t *testing.T) {
	manager, _ := newOTPTestManager(t)
	userID := uuid.New()

	_, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)
	_, err = manager.Generate(context.Background(), userID, "LOGIN")
	require.NoError(t, err)
}

func TestOTPManagerVerifyConsumesCodeAtomically(t *testing.T) {
	manager, _ := newOTPTestManager(t)
	userID := uuid.New()

	code, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)

	require.NoError(t, manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", code))
	require.ErrorIs(t,
		manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", code),
		domain.ErrInvalidSecurityCode,
		"the second attempt must fail because the code was consumed",
	)
}

func TestOTPManagerVerifyRejectsWrongCodeAndConsumesIt(t *testing.T) {
	manager, server := newOTPTestManager(t)
	userID := uuid.New()

	code, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	require.ErrorIs(t,
		manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", wrong),
		domain.ErrInvalidSecurityCode,
	)
	require.False(t, server.Exists("otp:"+userID.String()+":CHANGE_PASSWORD"))
	require.ErrorIs(t,
		manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", code),
		domain.ErrInvalidSecurityCode,
	)
}

func TestOTPManagerVerifyRejectsExpiredCode(t *testing.T) {
	manager, server := newOTPTestManager(t)
	userID := uuid.New()

	code, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)
	server.FastForward(5*time.Minute + time.Second)

	require.ErrorIs(t,
		manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", code),
		domain.ErrInvalidSecurityCode,
	)
}

func TestOTPManagerFailsClosedWhenRedisIsDown(t *testing.T) {
	manager, server := newOTPTestManager(t)
	userID := uuid.New()
	server.Close()

	_, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.ErrorIs(t, err, domain.ErrOTPServiceUnavailable)

	err = manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", "123456")
	require.ErrorIs(t, err, domain.ErrOTPServiceUnavailable)
}

func TestOTPManagerInvalidateRemovesCode(t *testing.T) {
	manager, _ := newOTPTestManager(t)
	userID := uuid.New()

	code, err := manager.Generate(context.Background(), userID, "CHANGE_PASSWORD")
	require.NoError(t, err)
	require.NoError(t, manager.Invalidate(context.Background(), userID, "CHANGE_PASSWORD"))

	require.ErrorIs(t,
		manager.Verify(context.Background(), userID, "CHANGE_PASSWORD", code),
		domain.ErrInvalidSecurityCode,
	)
}
