package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	TokenTypeAccess            = "access"
	TokenTypeTemp              = "2fa"
	TokenTypeClientCredentials = "client_credentials"
)

type Config struct {
	Secret     []byte
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	TempTTL    time.Duration
	ResetTTL   time.Duration
	ClientTTL  time.Duration
	BcryptCost int
	Now        func() time.Time
}

type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	tempTTL    time.Duration
	resetTTL   time.Duration
	clientTTL  time.Duration
	bcryptCost int
	dummyHash  []byte
	now        func() time.Time
}

type AccessClaims struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type TempClaims struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type ClientClaims struct {
	TenantID  string   `json:"tenant_id"`
	ClientID  string   `json:"client_id"`
	Scopes    []string `json:"scopes"`
	TokenType string   `json:"typ"`
	jwt.RegisteredClaims
}

func NewManager(cfg Config) (*Manager, error) {
	if len(cfg.Secret) < 32 {
		return nil, errors.New("auth: JWT secret must be at least 32 bytes")
	}
	if cfg.Issuer == "" {
		return nil, errors.New("auth: issuer is required")
	}
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	if cfg.TempTTL <= 0 {
		cfg.TempTTL = 5 * time.Minute
	}
	if cfg.ResetTTL <= 0 {
		cfg.ResetTTL = time.Hour
	}
	if cfg.ClientTTL <= 0 {
		cfg.ClientTTL = 5 * time.Minute
	}
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = 12
	}
	if cfg.BcryptCost < bcrypt.MinCost || cfg.BcryptCost > bcrypt.MaxCost {
		return nil, errors.New("auth: invalid bcrypt cost")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("enterprise-system-dummy-password"), cfg.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("auth: prepare dummy password hash: %w", err)
	}
	return &Manager{
		secret:     cfg.Secret,
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		tempTTL:    cfg.TempTTL,
		resetTTL:   cfg.ResetTTL,
		clientTTL:  cfg.ClientTTL,
		bcryptCost: cfg.BcryptCost,
		dummyHash:  dummyHash,
		now:        cfg.Now,
	}, nil
}

func (m *Manager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }
func (m *Manager) ResetTTL() time.Duration   { return m.resetTTL }
func (m *Manager) ClientTTL() time.Duration  { return m.clientTTL }

func (m *Manager) IssueAccessToken(tenantID, userID, sessionID uuid.UUID) (string, error) {
	now := m.now().UTC()
	claims := AccessClaims{
		TenantID:  tenantID.String(),
		UserID:    userID.String(),
		SessionID: sessionID.String(),
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) ParseAccessToken(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, m.keyFunc,
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid || claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.TenantID); err != nil {
		return nil, ErrInvalidToken
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) IssueClientToken(tenantID uuid.UUID, clientID string, scopes []string) (string, error) {
	now := m.now().UTC()
	claims := ClientClaims{
		TenantID:  tenantID.String(),
		ClientID:  clientID,
		Scopes:    scopes,
		TokenType: TokenTypeClientCredentials,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   clientID,
			ExpiresAt: jwt.NewNumericDate(now.Add(m.clientTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) ParseClientToken(raw string) (*ClientClaims, error) {
	claims := &ClientClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, m.keyFunc,
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid || claims.TokenType != TokenTypeClientCredentials ||
		strings.TrimSpace(claims.ClientID) == "" {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.TenantID); err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// GenerateClientCredentials creates an opaque client_id and a high-entropy
// client_secret. The caller must hash the secret before persisting it.
func GenerateClientCredentials() (clientID, clientSecret string, err error) {
	clientID, err = randomToken(24)
	if err != nil {
		return "", "", fmt.Errorf("generate client_id: %w", err)
	}
	clientSecret, err = randomToken(32)
	if err != nil {
		return "", "", fmt.Errorf("generate client_secret: %w", err)
	}
	return clientID, clientSecret, nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (m *Manager) IssueTempToken(tenantID, userID uuid.UUID) (string, error) {
	now := m.now().UTC()
	claims := TempClaims{
		TenantID:  tenantID.String(),
		UserID:    userID.String(),
		TokenType: TokenTypeTemp,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.tempTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) ParseTempToken(raw string) (*TempClaims, error) {
	claims := &TempClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, m.keyFunc,
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid || claims.TokenType != TokenTypeTemp {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.TenantID); err != nil {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) keyFunc(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodHS256 {
		return nil, ErrInvalidToken
	}
	return m.secret, nil
}

func (m *Manager) GenerateRefreshToken(tenantID, sessionID uuid.UUID) (string, []byte, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, fmt.Errorf("auth: generate refresh nonce: %w", err)
	}
	raw := fmt.Sprintf("%s.%s.%s", tenantID, sessionID, base64.RawURLEncoding.EncodeToString(nonce))
	return raw, m.HashRefreshToken(raw), nil
}

func (m *Manager) ParseRefreshToken(raw string) (uuid.UUID, uuid.UUID, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	tenantID, err := uuid.Parse(parts[0])
	if err != nil || tenantID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	sessionID, err := uuid.Parse(parts[1])
	if err != nil || sessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(nonce) < 16 {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	return tenantID, sessionID, nil
}

func (m *Manager) HashRefreshToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func (m *Manager) RefreshTokenMatches(raw string, hash []byte) bool {
	return subtle.ConstantTimeCompare(m.HashRefreshToken(raw), hash) == 1
}

func (m *Manager) GenerateResetToken(tenantID, partyID uuid.UUID) (string, []byte, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, fmt.Errorf("auth: generate reset nonce: %w", err)
	}
	raw := fmt.Sprintf("%s.%s.%s", tenantID, partyID, base64.RawURLEncoding.EncodeToString(nonce))
	return raw, m.HashResetToken(raw), nil
}

func (m *Manager) ParseResetToken(raw string) (uuid.UUID, uuid.UUID, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	tenantID, err := uuid.Parse(parts[0])
	if err != nil || tenantID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	partyID, err := uuid.Parse(parts[1])
	if err != nil || partyID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(nonce) < 16 {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	return tenantID, partyID, nil
}

func (m *Manager) HashResetToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func (m *Manager) ResetTokenMatches(raw string, hash []byte) bool {
	return subtle.ConstantTimeCompare(m.HashResetToken(raw), hash) == 1
}

func (m *Manager) HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), m.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(hash), nil
}

func (m *Manager) VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// DummyVerify spends a comparable amount of time when a login identifier does
// not exist, reducing user-enumeration by timing.
func (m *Manager) DummyVerify(password string) {
	_ = bcrypt.CompareHashAndPassword(m.dummyHash, []byte(password))
}

func (m *Manager) GenerateTOTPSecret(issuer, account string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: issuer, AccountName: account})
	if err != nil {
		return "", "", fmt.Errorf("auth: generate TOTP secret: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

func (m *Manager) ValidateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	valid, err := totp.ValidateCustom(code, secret, m.now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}

func (m *Manager) GenerateBackupCodes(count int) ([]string, []string, error) {
	if count <= 0 {
		return nil, nil, errors.New("auth: backup code count must be positive")
	}
	plain := make([]string, 0, count)
	hashes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		buf := make([]byte, 5)
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, fmt.Errorf("auth: generate backup code: %w", err)
		}
		code := hex.EncodeToString(buf)
		hash, err := bcrypt.GenerateFromPassword([]byte(code), m.bcryptCost)
		if err != nil {
			return nil, nil, fmt.Errorf("auth: hash backup code: %w", err)
		}
		plain = append(plain, code)
		hashes = append(hashes, string(hash))
	}
	return plain, hashes, nil
}

func (m *Manager) VerifyBackupCode(hashes []string, code string) ([]string, bool) {
	code = strings.TrimSpace(code)
	if code == "" {
		return hashes, false
	}
	for i, hash := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			remaining := make([]string, 0, len(hashes)-1)
			remaining = append(remaining, hashes[:i]...)
			remaining = append(remaining, hashes[i+1:]...)
			return remaining, true
		}
	}
	return hashes, false
}
