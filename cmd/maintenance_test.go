package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMaintenanceCmd(t *testing.T) {
	dataDir := "/tmp/test"
	cmd := NewMaintenanceCmd(&dataDir)

	assert.Equal(t, "maintenance", cmd.Use)
	assert.Equal(t, "Maintenance mode management commands", cmd.Short)
	assert.NotNil(t, cmd.Commands())
	assert.Greater(t, len(cmd.Commands()), 0)
}

func TestNewMaintenanceEnableCmd(t *testing.T) {
	dataDir := "/tmp/test"
	cmd := newMaintenanceEnableCmd(&dataDir)

	assert.Equal(t, "enable [message]", cmd.Use)
	assert.Equal(t, "Enable maintenance mode", cmd.Short)
	assert.NotNil(t, cmd.Run)
}

func TestNewMaintenanceDisableCmd(t *testing.T) {
	dataDir := "/tmp/test"
	cmd := newMaintenanceDisableCmd(&dataDir)

	assert.Equal(t, "disable", cmd.Use)
	assert.Equal(t, "Disable maintenance mode", cmd.Short)
	assert.NotNil(t, cmd.Run)
}