package cmd

import (
	"os"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/spf13/cobra"
)

// NewSeriesCmd creates the series command
func NewSeriesCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "series",
		Short: "Series (media) management commands",
	}

	cmd.AddCommand(
		newSeriesListCmd(dataDirectory),
		newSeriesInfoCmd(dataDirectory),
		newSeriesUpdateCmd(dataDirectory),
		newSeriesDeleteCmd(dataDirectory),
	)

	return cmd
}

func newSeriesListCmd(dataDirectory *string) *cobra.Command {
	var librarySlug string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List series in a library",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize database without auto-migrations
			err := models.InitializeWithMigration(*dataDirectory, false)
			if err != nil {
				cmd.PrintErrf("Failed to connect to database: %v\n", err)
				os.Exit(1)
			}
			defer models.Close()

			var medias []models.Media

			if librarySlug != "" {
				medias, err = models.GetMediasByLibrarySlug(librarySlug)
			} else {
				// Get all medias with limit
				medias, err = models.GetTopMedias(limit)
			}

			if err != nil {
				cmd.PrintErrf("Failed to get series: %v\n", err)
				os.Exit(1)
			}

			if len(medias) == 0 {
				cmd.Println("No series found.")
				return
			}

			cmd.Printf("Series (%d):\n", len(medias))
			for _, media := range medias {
				cmd.Printf("  %s: %s (%d chapters)\n", media.Slug, media.Name, media.FileCount)
			}
		},
	}

	cmd.Flags().StringVar(&librarySlug, "library", "", "Filter by library slug")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit number of results")

	return cmd
}

func newSeriesInfoCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "info [slug]",
		Short: "Show detailed information about a series",
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

			media, err := models.GetMediaUnfiltered(slug)
			if err != nil {
				cmd.PrintErrf("Failed to get series: %v\n", err)
				os.Exit(1)
			}
			if media == nil {
				cmd.PrintErrf("Series '%s' not found\n", slug)
				os.Exit(1)
			}

			cmd.Printf("Series Information:\n")
			cmd.Printf("  Slug: %s\n", media.Slug)
			cmd.Printf("  Name: %s\n", media.Name)
			cmd.Printf("  Author: %s\n", media.Author)
			cmd.Printf("  Description: %s\n", media.Description)
			cmd.Printf("  Year: %d\n", media.Year)
			cmd.Printf("  Type: %s\n", media.Type)
			cmd.Printf("  Status: %s\n", media.Status)
			cmd.Printf("  Content Rating: %s\n", media.ContentRating)
			cmd.Printf("  Library: %s\n", media.LibrarySlug)
			cmd.Printf("  Chapters: %d\n", media.FileCount)
			cmd.Printf("  Read Count: %d\n", media.ReadCount)
			cmd.Printf("  Vote Score: %d\n", media.VoteScore)
			cmd.Printf("  Tags: %s\n", strings.Join(media.Tags, ", "))
			cmd.Printf("  Path: %s\n", media.Path)
			cmd.Printf("  Cover Art: %s\n", media.CoverArtURL)
		},
	}
}

func newSeriesUpdateCmd(dataDirectory *string) *cobra.Command {
	var name, author, description, mangaType, status, contentRating string
	var year int
	var tags []string

	cmd := &cobra.Command{
		Use:   "update [slug]",
		Short: "Update series metadata",
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

			// Get existing media
			media, err := models.GetMediaUnfiltered(slug)
			if err != nil {
				cmd.PrintErrf("Failed to get series: %v\n", err)
				os.Exit(1)
			}
			if media == nil {
				cmd.PrintErrf("Series '%s' not found\n", slug)
				os.Exit(1)
			}

			// Update fields if provided
			if name != "" {
				media.Name = name
			}
			if author != "" {
				media.Author = author
			}
			if description != "" {
				media.Description = description
			}
			if year > 0 {
				media.Year = year
			}
			if mangaType != "" {
				media.Type = mangaType
			}
			if status != "" {
				media.Status = status
			}
			if contentRating != "" {
				media.ContentRating = contentRating
			}
			if len(tags) > 0 {
				err = models.SetTagsForMedia(slug, tags)
				if err != nil {
					cmd.PrintErrf("Failed to update tags: %v\n", err)
					os.Exit(1)
				}
			}

			err = models.UpdateMedia(media)
			if err != nil {
				cmd.PrintErrf("Failed to update series: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Series '%s' updated successfully\n", slug)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Series name")
	cmd.Flags().StringVar(&author, "author", "", "Series author")
	cmd.Flags().StringVar(&description, "description", "", "Series description")
	cmd.Flags().IntVar(&year, "year", 0, "Publication year")
	cmd.Flags().StringVar(&mangaType, "type", "", "Series type (manga, manhwa, manhua, etc.)")
	cmd.Flags().StringVar(&status, "status", "", "Series status (ongoing, completed, etc.)")
	cmd.Flags().StringVar(&contentRating, "content-rating", "", "Content rating (safe, suggestive, etc.)")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Comma-separated list of tags")

	return cmd
}

func newSeriesDeleteCmd(dataDirectory *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [slug]",
		Short: "Delete a series",
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

			err = models.DeleteMedia(slug)
			if err != nil {
				cmd.PrintErrf("Failed to delete series: %v\n", err)
				os.Exit(1)
			}

			cmd.Printf("Series '%s' deleted successfully\n", slug)
		},
	}
}