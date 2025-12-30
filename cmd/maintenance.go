package cmd

import (
	"os"
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

			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			_, err = models.UpdateMaintenanceConfig(true, message)
			if err != nil {
				cmd.PrintErrf("Failed to enable maintenance mode: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Maintenance mode enabled successfully\n")
		},
	}
}

func newMaintenanceDisableCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable maintenance mode",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			_, err = models.UpdateMaintenanceConfig(false, "")
			if err != nil {
				cmd.PrintErrf("Failed to disable maintenance mode: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Maintenance mode disabled successfully\n")
		},
	}
}
