package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
