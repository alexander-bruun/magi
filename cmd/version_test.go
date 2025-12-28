package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVersionCmd_Execution(t *testing.T) {
	version := "1.0.0-test"
	cmd := NewVersionCmd(version)

	// Test command properties
	assert.Equal(t, "version", cmd.Use)
	assert.Equal(t, "Print the version number", cmd.Short)

	// Test command execution
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Version: 1.0.0-test")
}
