package main

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCSV(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Equal(t, []string{"a", "b", "c"}, splitCSV(" a, b ,, c "))
}

func TestShutdownTimeout(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "")
		assert.Equal(t, 10*time.Second, shutdownTimeout())
	})
	t.Run("configured", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "3s")
		assert.Equal(t, 3*time.Second, shutdownTimeout())
	})
	t.Run("invalid falls back", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "not-a-duration")
		assert.Equal(t, 10*time.Second, shutdownTimeout())
	})
}

func TestValidatePlatformAdminKey(t *testing.T) {
	assert.Error(t, validatePlatformAdminKey(""))
	assert.Error(t, validatePlatformAdminKey("change-me"))
	assert.Error(t, validatePlatformAdminKey("short"))
	assert.Error(t, validatePlatformAdminKey(strings.Repeat("a", 31)))
	require.NoError(t, validatePlatformAdminKey(strings.Repeat("a", 32)))
	require.NoError(t, validatePlatformAdminKey("  "+strings.Repeat("b", 32)+"  "))
}
