package repository_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMFAMigrationSchema(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	var hasFlag, hasMFAEnabled, hasMFAMethod, hasLegacyTotp, hasTOTPVerified bool
	err := db.QueryRow(`
		SELECT
			EXISTS (SELECT 1 FROM information_schema.columns
			         WHERE table_schema='global' AND table_name='contact_mechanisms'
			           AND column_name='can_receive_security_code'),
			EXISTS (SELECT 1 FROM information_schema.columns
			         WHERE table_schema='tenants' AND table_name='user_credentials'
			           AND column_name='mfa_enabled'),
			EXISTS (SELECT 1 FROM information_schema.columns
			         WHERE table_schema='tenants' AND table_name='user_credentials'
			           AND column_name='mfa_method'),
			EXISTS (SELECT 1 FROM information_schema.columns
			         WHERE table_schema='tenants' AND table_name='user_credentials'
			           AND column_name='totp_enabled'),
			EXISTS (SELECT 1 FROM information_schema.columns
			         WHERE table_schema='tenants' AND table_name='user_credentials'
			           AND column_name='totp_verified')
	`).Scan(&hasFlag, &hasMFAEnabled, &hasMFAMethod, &hasLegacyTotp, &hasTOTPVerified)
	require.NoError(t, err)
	require.True(t, hasFlag)
	require.True(t, hasMFAEnabled)
	require.True(t, hasMFAMethod)
	require.False(t, hasLegacyTotp, "totp_enabled must be renamed to mfa_enabled")
	require.True(t, hasTOTPVerified)

	var methodCheck, consistencyCheck string
	err = db.QueryRow(`
		SELECT pg_get_constraintdef(oid)
		  FROM pg_constraint
		 WHERE conname='user_credentials_mfa_method_chk'
	`).Scan(&methodCheck)
	require.NoError(t, err)
	require.Contains(t, methodCheck, "EMAIL_OTP")
	require.Contains(t, methodCheck, "TOTP")
	require.Contains(t, methodCheck, "NONE")

	err = db.QueryRow(`
		SELECT pg_get_constraintdef(oid)
		  FROM pg_constraint
		 WHERE conname='user_credentials_mfa_consistency_chk'
	`).Scan(&consistencyCheck)
	require.NoError(t, err)
	require.Contains(t, consistencyCheck, "mfa_enabled")
	require.Contains(t, consistencyCheck, "mfa_method")

	var verifiedCheck string
	err = db.QueryRow(`
		SELECT pg_get_constraintdef(oid)
		  FROM pg_constraint
		 WHERE conname='user_credentials_totp_verified_chk'
	`).Scan(&verifiedCheck)
	require.NoError(t, err)
	require.Contains(t, verifiedCheck, "totp_verified")
}

func TestContactMechanismSecurityFlagDefaultsToFalse(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	id := uuid.New()
	value := "security-flag-" + id.String() + "@example.com"
	_, err = tx.Exec(`
		INSERT INTO global.contact_mechanisms (id, type, value)
		VALUES ($1, 'EMAIL', $2)`, id, value)
	require.NoError(t, err)

	var flag bool
	err = tx.QueryRow(`SELECT can_receive_security_code FROM global.contact_mechanisms WHERE id = $1`, id).Scan(&flag)
	require.NoError(t, err)
	require.False(t, flag, "the column default must be false; the application sets the type-specific value")
}
