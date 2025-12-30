package cmd

import (
	"os"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewMigrateCmd creates the migrate command
func NewMigrateCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
	}

	cmd.AddCommand(
		newMigrateUpCmd(dataDirectory),
		newMigrateDownCmd(dataDirectory),
	)

	return cmd
}

func newMigrateUpCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "up [version|all]",
		Short: "Run migrations up to a specific version or all pending migrations",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var version int
			var err error

			if args[0] == "all" {
				// Run all pending migrations
				err = models.InitializeWithMigration(*dataDirectory, true)
				if err != nil {
					cmd.PrintErrf("Failed to apply migrations: %v\n", err)
					os.Exit(1)
				}
				cmd.Println("All pending migrations applied successfully")
				return
			} else {
				version, err = strconv.Atoi(args[0])
				if err != nil {
					cmd.PrintErrf("Invalid version number: %s\n", args[0])
					os.Exit(1)
				}
			}

			// Initialize database without auto-migrations
			err = models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.MigrateUp("migrations", version)
			if err != nil {
				cmd.PrintErrf("Migration failed: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Migration up %d completed successfully\n", version)
		},
	}
}

func newMigrateDownCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "down [version|all]",
		Short: "Rollback migrations down to a specific version or rollback all migrations",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var version int
			var err error

			if args[0] == "all" {
				// Rollback all migrations
				err = models.InitializeWithMigration(*dataDirectory, false)
				if err != nil {
					cmd.PrintErrf("Failed to connect to database: %v\n", err)
					os.Exit(1)
				}
				defer models.Close()

				// Get all applied migrations and rollback them in reverse order
				// For simplicity, rollback from highest to lowest
				for v := 17; v >= 1; v-- {
					err = models.MigrateDown("migrations", v)
					if err != nil {
						cmd.PrintErrf("Failed to rollback migration %d: %v\n", v, err)
						os.Exit(1)
					}
				}
				cmd.Println("All migrations rolled back successfully")
				return
			} else {
				version, err = strconv.Atoi(args[0])
				if err != nil {
					cmd.PrintErrf("Invalid version number: %s\n", args[0])
					os.Exit(1)
				}
			}

			// Initialize database without auto-migrations
			err = models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.MigrateDown("migrations", version)
			if err != nil {
				cmd.PrintErrf("Migration failed: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Migration down %d completed successfully\n", version)
		},
	}
}
