package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{999, "999 B"},
		{1000, "1000 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},      // 1024 * 1024
		{1073741824, "1.0 GB"},   // 1024 * 1024 * 1024
		{1099511627776, "1.0 TB"}, // 1024^4
		{1125899906842624, "1.0 PB"}, // 1024^5
		{1152921504606846976, "1.0 EB"}, // 1024^6
	}

	for _, test := range tests {
		result := formatFileSize(test.size)
		assert.Equal(t, test.expected, result, "formatFileSize(%d)", test.size)
	}
}

func TestNewVersionCmd(t *testing.T) {
	version := "1.2.3"
	cmd := NewVersionCmd(version)

	assert.Equal(t, "version", cmd.Use)
	assert.Equal(t, "Print the version number", cmd.Short)
	assert.NotNil(t, cmd.Run)
}

func TestNewUserCmd(t *testing.T) {
	dataDir := "/tmp/test"
	cmd := NewUserCmd(&dataDir)

	assert.Equal(t, "user", cmd.Use)
	assert.Equal(t, "User management commands", cmd.Short)
	assert.NotNil(t, cmd.Commands())
	assert.Greater(t, len(cmd.Commands()), 0)
}

func TestNewMigrateCmd(t *testing.T) {
	dataDir := "/tmp/test"
	cmd := NewMigrateCmd(&dataDir)

	assert.Equal(t, "migrate", cmd.Use)
	assert.Equal(t, "Run database migrations", cmd.Short)
	assert.NotNil(t, cmd.Commands())
	assert.Greater(t, len(cmd.Commands()), 0)
}

func TestNewBackupCmd(t *testing.T) {
	dataDir := "/tmp/test"
	backupDir := "/tmp/backup"
	cmd := NewBackupCmd(&dataDir, &backupDir)

	assert.Equal(t, "backup", cmd.Use)
	assert.Equal(t, "Database backup and restore commands", cmd.Short)
	assert.NotNil(t, cmd.Commands())
	assert.Greater(t, len(cmd.Commands()), 0)
}