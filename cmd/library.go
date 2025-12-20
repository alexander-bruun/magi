package cmd

import (
	"os"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewLibraryCmd creates the library command
func NewLibraryCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Library management commands",
	}

	cmd.AddCommand(
		newLibraryListCmd(dataDirectory),
		newLibraryCreateCmd(dataDirectory),
		newLibraryUpdateCmd(dataDirectory),
		newLibraryDeleteCmd(dataDirectory),
	)

	return cmd
}

func newLibraryListCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all libraries",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			libraries, err := models.GetLibraries()
			if err != nil {
				cmd.PrintErrf("Failed to get libraries: %v\n", err)
				os.Exit(1)
			}

			if len(libraries) == 0 {
				cmd.Println("No libraries found.")
				return
			}

			cmd.Println("Libraries:")
			for _, lib := range libraries {
				cmd.Printf("  %s: %s (%s)\n", lib.Slug, lib.Name, strings.Join(lib.Folders, ", "))
			}
		},
	}
}

func newLibraryCreateCmd(dataDirectory *string) *cobra.Command {
	var description, cron string

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new library",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			library := models.Library{
				Name:        name,
				Description: description,
				Cron:        cron,
				Folders:     []string{}, // Empty folders, can be added later
			}

			err = models.CreateLibrary(library)
			if err != nil {
				cmd.PrintErrf("Failed to create library: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Library '%s' created successfully\n", name)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Library description (required)")
	cmd.Flags().StringVar(&cron, "cron", "", "Library scan cron schedule (required)")
	cmd.MarkFlagRequired("description")
	cmd.MarkFlagRequired("cron")

	return cmd
}

func newLibraryUpdateCmd(dataDirectory *string) *cobra.Command {
	var name, description, cron string

	cmd := &cobra.Command{
		Use:   "update [slug]",
		Short: "Update an existing library",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			slug := args[0]

			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			// Get existing library
			library, err := models.GetLibrary(slug)
			if err != nil {
				cmd.PrintErrf("Failed to get library: %v\n", err)
				os.Exit(1)
			}
			if library == nil {
				cmd.PrintErrf("Library '%s' not found\n", slug)
				os.Exit(1)
			}

			// Update fields if provided
			if name != "" {
				library.Name = name
			}
			if description != "" {
				library.Description = description
			}
			if cron != "" {
				library.Cron = cron
			}

			err = models.UpdateLibrary(library)
			if err != nil {
				cmd.PrintErrf("Failed to update library: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Library '%s' updated successfully\n", slug)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New library name")
	cmd.Flags().StringVar(&description, "description", "", "New library description")
	cmd.Flags().StringVar(&cron, "cron", "", "New library scan cron schedule")

	return cmd
}

func newLibraryDeleteCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [slug]",
		Short: "Delete a library",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			slug := args[0]

			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.DeleteLibrary(slug)
			if err != nil {
				cmd.PrintErrf("Failed to delete library: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Library '%s' deleted successfully\n", slug)
		},
	}
}