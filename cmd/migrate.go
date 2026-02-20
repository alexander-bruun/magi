package cmd

import (
	"fmt"
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
		newMigrateClearCmd(dataDirectory),
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
			withDB(dataDirectory, cmd, func() error {
				if err = models.MigrateUp("migrations", version); err != nil {
					return fmt.Errorf("Migration failed: %w", err)
				}

				cmd.Printf("Migration up %d completed successfully\n", version)
				return nil
			})
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
				withDB(dataDirectory, cmd, func() error {
					for v := 17; v >= 1; v-- {
						if err = models.MigrateDown("migrations", v); err != nil {
							return fmt.Errorf("Failed to rollback migration %d: %w", v, err)
						}
					}
					cmd.Println("All migrations rolled back successfully")
					return nil
				})
				return
			} else {
				version, err = strconv.Atoi(args[0])
				if err != nil {
					cmd.PrintErrf("Invalid version number: %s\n", args[0])
					os.Exit(1)
				}
			}

			// Initialize database without auto-migrations
			withDB(dataDirectory, cmd, func() error {
				if err = models.MigrateDown("migrations", version); err != nil {
					return fmt.Errorf("Migration failed: %w", err)
				}

				cmd.Printf("Migration down %d completed successfully\n", version)
				return nil
			})
		},
	}
}

func newMigrateClearCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear all migration schema versions from the database",
		Long:  "This will clear the schema_migrations table, making the system think no migrations have been applied. Use with caution!",
		Run: func(cmd *cobra.Command, args []string) {
			withDB(dataDirectory, cmd, func() error {
				if err := models.ClearMigrationVersions(); err != nil {
					return fmt.Errorf("Failed to clear migration versions: %w", err)
				}

				cmd.Println("Migration schema versions cleared successfully")
				cmd.Println("WARNING: The database schema has not been modified. Run 'migrate up all' to reapply all migrations.")
				return nil
			})
		},
	}
}
