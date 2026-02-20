package cmd

import (
	"fmt"
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
			withDB(dataDirectory, cmd, func() error {
				// Verify the media exists
				if _, err := models.GetMedia(mediaSlug); err != nil {
					return fmt.Errorf("Media with slug '%s' not found: %w", mediaSlug, err)
				}

				highlight, err := models.CreateHighlight(mediaSlug, backgroundImageURL, description, displayOrder)
				if err != nil {
					return fmt.Errorf("Failed to create highlight: %w", err)
				}

				cmd.Printf("Highlight created successfully for series '%s' (ID: %d)\n", mediaSlug, highlight.ID)
				return nil
			})
		},
	}
}

func newHighlightsListCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all highlighted series",
		Run: func(cmd *cobra.Command, args []string) {
			withDB(dataDirectory, cmd, func() error {
				highlights, err := models.GetHighlights()
				if err != nil {
					return fmt.Errorf("Failed to get highlights: %w", err)
				}

				if len(highlights) == 0 {
					cmd.Println("No highlights found.")
					return nil
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
				return nil
			})
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

			withDB(dataDirectory, cmd, func() error {
				if err := models.DeleteHighlight(id); err != nil {
					return fmt.Errorf("Failed to delete highlight: %w", err)
				}

				cmd.Printf("Highlight with ID %d removed successfully\n", id)
				return nil
			})
		},
	}
}
