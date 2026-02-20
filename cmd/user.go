package cmd

import (
	"fmt"

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

			withDB(dataDirectory, cmd, func() error {
				if err := models.ResetUserPassword(username, newPassword); err != nil {
					return fmt.Errorf("Failed to reset password for user '%s': %w", username, err)
				}
				cmd.Printf("Password reset successfully for user '%s'\n", username)
				return nil
			})
		},
	}
}
