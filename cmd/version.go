package cmd

import (
	"github.com/spf13/cobra"
)

// NewVersionCmd creates the version command
func NewVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("Version: %s\n", version)
		},
	}
}