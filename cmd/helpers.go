package cmd

import (
	"os"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// withDB initializes the database connection (without auto-migration), calls fn,
// and ensures models.Close() is called afterward. If initialization or fn fails,
// the error is printed and the process exits with code 1.
func withDB(dataDirectory *string, cmd *cobra.Command, fn func() error) {
	if err := models.InitializeWithMigration(*dataDirectory, false); err != nil {
		cmd.PrintErrf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer models.Close()

	if err := fn(); err != nil {
		cmd.PrintErrf("%v\n", err)
		os.Exit(1)
	}
}
