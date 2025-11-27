package cmd

import (
	"os"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewUserCmd creates the user command
func NewUserCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	cmd.AddCommand(
		newResetPasswordCmd(dataDirectory),
	)

	return cmd
}

func newResetPasswordCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "reset-password [username] [new-password]",
		Short: "Reset a user's password",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			username := args[0]
			newPassword := args[1]

			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.ResetUserPassword(username, newPassword)
			if err != nil {
				cmd.PrintErrf("Failed to reset password for user '%s': %v\n", username, err)
				os.Exit(1)
			}

			cmd.Printf("Password reset successfully for user '%s'\n", username)
		},
	}
}