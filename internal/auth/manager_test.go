package auth

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testManager(t *testing.T, now func() time.Time) *Manager {
	t.Helper()
	m, err := NewManager(Config{
		Secret:     bytes.Repeat([]byte("s"), 32),
		Issuer:     "enterprise-system",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		TempTTL:    5 * time.Minute,
		BcryptCost: 4,
		Now:        now,
	})
	require.NoError(t, err)
	return m
}

func TestManagerRequiresStrongSecret(t *testing.T) {
	_, err := NewManager(Config{Secret: []byte("short"), Issuer: "x"})
	require.Error(t, err)
}

func TestAccessTokenRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	m := testManager(t, func() time.Time { return now })
	tenantID, userID, sessionID := uuid.New(), uuid.New(), uuid.New()

	raw, err := m.IssueAccessToken(tenantID, userID, sessionID)
	require.NoError(t, err)

	claims, err := m.ParseAccessToken(raw)
	require.NoError(t, err)
	assert.Equal(t, tenantID.String(), claims.TenantID)
	assert.Equal(t, userID.String(), claims.UserID)
	assert.Equal(t, sessionID.String(), claims.SessionID)
	assert.Equal(t, TokenTypeAccess, claims.TokenType)
}

func TestAccessTokenExpires(t *testing.T) {
	current := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	m := testManager(t, func() time.Time { return current })
	raw, err := m.IssueAccessToken(uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)

	current = current.Add(16 * time.Minute)
	_, err = m.ParseAccessToken(raw)
	require.ErrorIs(t, err, ErrExpiredToken)
}

func TestAccessTokenRejectsTampering(t *testing.T) {
	m := testManager(t, time.Now)
	raw, err := m.IssueAccessToken(uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)

	_, err = m.ParseAccessToken(raw + "x")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTempTokenRoundTrip(t *testing.T) {
	m := testManager(t, time.Now)
	tenantID, userID := uuid.New(), uuid.New()

	raw, err := m.IssueTempToken(tenantID, userID)
	require.NoError(t, err)

	claims, err := m.ParseTempToken(raw)
	require.NoError(t, err)
	assert.Equal(t, tenantID.String(), claims.TenantID)
	assert.Equal(t, userID.String(), claims.UserID)
	assert.Equal(t, TokenTypeTemp, claims.TokenType)
}

func TestRefreshTokenGenerateParseAndHash(t *testing.T) {
	m := testManager(t, time.Now)
	tenantID, sessionID := uuid.New(), uuid.New()

	raw, hash, err := m.GenerateRefreshToken(tenantID, sessionID)
	require.NoError(t, err)
	assert.Len(t, hash, 32)

	gotTenant, gotSession, err := m.ParseRefreshToken(raw)
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenant)
	assert.Equal(t, sessionID, gotSession)
	assert.True(t, m.RefreshTokenMatches(raw, hash))
	assert.False(t, m.RefreshTokenMatches(raw+"tamper", hash))
}

func TestRefreshTokenRejectsMalformed(t *testing.T) {
	m := testManager(t, time.Now)
	_, _, err := m.ParseRefreshToken("not-a-token")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestPasswordHashAndVerify(t *testing.T) {
	m := testManager(t, time.Now)

	hash, err := m.HashPassword("correct horse battery staple")
	require.NoError(t, err)
	assert.NotEqual(t, "correct horse battery staple", hash)
	assert.True(t, m.VerifyPassword(hash, "correct horse battery staple"))
	assert.False(t, m.VerifyPassword(hash, "wrong password"))

	_, err = m.HashPassword("short")
	require.ErrorIs(t, err, ErrWeakPassword)
}

func TestTOTPGenerateAndValidate(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	m := testManager(t, func() time.Time { return now })

	secret, url, err := m.GenerateTOTPSecret("enterprise-system", "user@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, url, "otpauth://totp/")

	code, err := totp.GenerateCode(secret, now)
	require.NoError(t, err)
	assert.True(t, m.ValidateTOTP(secret, code))
	assert.False(t, m.ValidateTOTP(secret, "000000"))
}

func TestBackupCodesAreHashedAndOneTime(t *testing.T) {
	m := testManager(t, time.Now)

	plain, hashes, err := m.GenerateBackupCodes(3)
	require.NoError(t, err)
	require.Len(t, plain, 3)
	require.Len(t, hashes, 3)
	assert.NotEqual(t, plain[0], hashes[0])

	remaining, ok := m.VerifyBackupCode(hashes, plain[0])
	require.True(t, ok)
	assert.Len(t, remaining, 2)

	_, ok = m.VerifyBackupCode(remaining, plain[0])
	assert.False(t, ok)
}

func TestResetTokenGenerateParseAndHash(t *testing.T) {
	m := testManager(t, time.Now)
	tenantID, partyID := uuid.New(), uuid.New()

	raw, hash, err := m.GenerateResetToken(tenantID, partyID)
	require.NoError(t, err)
	assert.Len(t, hash, 32)

	gotTenant, gotParty, err := m.ParseResetToken(raw)
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenant)
	assert.Equal(t, partyID, gotParty)
	assert.True(t, m.ResetTokenMatches(raw, hash))
	assert.False(t, m.ResetTokenMatches(raw+"tamper", hash))
}

func TestResetTokenRejectsMalformed(t *testing.T) {
	m := testManager(t, time.Now)
	_, _, err := m.ParseResetToken("not-a-token")
	require.ErrorIs(t, err, ErrInvalidToken)
}
