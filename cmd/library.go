package cmd

import (
	"fmt"
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
			withDB(dataDirectory, cmd, func() error {
				libraries, err := models.GetLibraries()
				if err != nil {
					return fmt.Errorf("Failed to get libraries: %w", err)
				}

				if len(libraries) == 0 {
					cmd.Println("No libraries found.")
					return nil
				}

				cmd.Println("Libraries:")
				for _, lib := range libraries {
					cmd.Printf("  %s: %s (%s)\n", lib.Slug, lib.Name, strings.Join(lib.Folders, ", "))
				}
				return nil
			})
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

			withDB(dataDirectory, cmd, func() error {
				library := models.Library{
					Name:        name,
					Description: description,
					Cron:        cron,
					Folders:     []string{}, // Empty folders, can be added later
				}

				if err := models.CreateLibrary(library); err != nil {
					return fmt.Errorf("Failed to create library: %w", err)
				}

				cmd.Printf("Library '%s' created successfully\n", name)
				return nil
			})
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

			withDB(dataDirectory, cmd, func() error {
				// Get existing library
				library, err := models.GetLibrary(slug)
				if err != nil {
					return fmt.Errorf("Failed to get library: %w", err)
				}
				if library == nil {
					return fmt.Errorf("Library '%s' not found", slug)
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

				if err := models.UpdateLibrary(library); err != nil {
					return fmt.Errorf("Failed to update library: %w", err)
				}

				cmd.Printf("Library '%s' updated successfully\n", slug)
				return nil
			})
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

			withDB(dataDirectory, cmd, func() error {
				if err := models.DeleteLibrary(slug); err != nil {
					return fmt.Errorf("Failed to delete library: %w", err)
				}

				cmd.Printf("Library '%s' deleted successfully\n", slug)
				return nil
			})
		},
	}
}
