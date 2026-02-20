package cmd

import (
	"fmt"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewMaintenanceCmd creates the maintenance command
func NewMaintenanceCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Maintenance mode management commands",
	}

	cmd.AddCommand(
		newMaintenanceEnableCmd(dataDirectory),
		newMaintenanceDisableCmd(dataDirectory),
	)

	return cmd
}

func newMaintenanceEnableCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "enable [message]",
		Short: "Enable maintenance mode",
		Long:  `Enable maintenance mode with an optional custom message. If no message is provided, a default message will be used.`,
		Run: func(cmd *cobra.Command, args []string) {
			message := "We are currently performing maintenance. Please check back later."
			if len(args) > 0 {
				message = strings.Join(args, " ")
			}

			withDB(dataDirectory, cmd, func() error {
				if _, err := models.UpdateMaintenanceConfig(true, message); err != nil {
					return fmt.Errorf("Failed to enable maintenance mode: %w", err)
				}
				cmd.Printf("Maintenance mode enabled successfully\n")
				return nil
			})
		},
	}
}

func newMaintenanceDisableCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable maintenance mode",
		Run: func(cmd *cobra.Command, args []string) {
			withDB(dataDirectory, cmd, func() error {
				if _, err := models.UpdateMaintenanceConfig(false, ""); err != nil {
					return fmt.Errorf("Failed to disable maintenance mode: %w", err)
				}
				cmd.Printf("Maintenance mode disabled successfully\n")
				return nil
			})
		},
	}
}
