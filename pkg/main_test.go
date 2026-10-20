package main

import (
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRestoreHomeDir(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)

	t.Run("sets HOME from the OS account when unset", func(t *testing.T) {
		t.Setenv("HOME", "")
		restoreHomeDir()
		require.Equal(t, u.HomeDir, os.Getenv("HOME"))
	})

	t.Run("keeps HOME when set", func(t *testing.T) {
		t.Setenv("HOME", "/custom/home")
		restoreHomeDir()
		require.Equal(t, "/custom/home", os.Getenv("HOME"))
	})
}
