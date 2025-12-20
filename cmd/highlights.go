package cmd

import (
	"os"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewHighlightsCmd creates the highlights command
func NewHighlightsCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "highlights",
		Short: "Manage highlighted series for the home page banner",
	}

	cmd.AddCommand(
		newHighlightsAddCmd(dataDirectory),
		newHighlightsListCmd(dataDirectory),
		newHighlightsRemoveCmd(dataDirectory),
	)

	return cmd
}

func newHighlightsAddCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "add [media-slug] [background-image-url] [description] [display-order]",
		Short: "Add a series to the highlights banner",
		Long: `Add a series to the highlights banner.

The media-slug should be the slug of an existing series.
The background-image-url should be a URL to an image to use as the banner background.
The description is optional text to display with the highlight.
The display-order determines the order of highlights (lower numbers appear first).`,
		Args: cobra.RangeArgs(2, 4),
		Run: func(cmd *cobra.Command, args []string) {
			mediaSlug := args[0]
			backgroundImageURL := args[1]
			description := ""
			displayOrder := 0

			if len(args) >= 3 {
				description = args[2]
			}
			if len(args) >= 4 {
				var err error
				displayOrder, err = strconv.Atoi(args[3])
				if err != nil {
					cmd.PrintErrf("Invalid display order: %s\n", args[3])
					os.Exit(1)
				}
			}

			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			// Verify the media exists
			_, err = models.GetMedia(mediaSlug)
			if err != nil {
				cmd.PrintErrf("Media with slug '%s' not found: %v\n", mediaSlug, err)
				os.Exit(1)
			}

			highlight, err := models.CreateHighlight(mediaSlug, backgroundImageURL, description, displayOrder)
			if err != nil {
				cmd.PrintErrf("Failed to create highlight: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Highlight created successfully for series '%s' (ID: %d)\n", mediaSlug, highlight.ID)
		},
	}
}

func newHighlightsListCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all highlighted series",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			highlights, err := models.GetHighlights()
			if err != nil {
				cmd.PrintErrf("Failed to get highlights: %v\n", err)
				os.Exit(1)
			}

			if len(highlights) == 0 {
				cmd.Println("No highlights found.")
				return
			}

			cmd.Println("Highlighted Series:")
			cmd.Println("==================")
			for _, h := range highlights {
				cmd.Printf("ID: %d\n", h.Highlight.ID)
				cmd.Printf("Series: %s (%s)\n", h.Media.Name, h.Media.Slug)
				cmd.Printf("Background: %s\n", h.Highlight.BackgroundImageURL)
				if h.Highlight.Description != "" {
					cmd.Printf("Description: %s\n", h.Highlight.Description)
				}
				cmd.Printf("Display Order: %d\n", h.Highlight.DisplayOrder)
				cmd.Println("---")
			}
		},
	}
}

func newHighlightsRemoveCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "remove [id]",
		Short: "Remove a highlight by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				cmd.PrintErrf("Invalid ID: %s\n", args[0])
				os.Exit(1)
			}

			// Initialize database without auto-migrations
			err = models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.DeleteHighlight(id)
			if err != nil {
				cmd.PrintErrf("Failed to delete highlight: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Highlight with ID %d removed successfully\n", id)
		},
	}
}