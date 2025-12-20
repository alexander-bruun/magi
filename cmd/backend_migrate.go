package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/filestore"
	"github.com/spf13/cobra"
)

// NewBackendMigrateCmd creates the backend-migrate command
func NewBackendMigrateCmd(dataDirectory *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backend-migrate",
		Short: "Migrate cache data between different backends",
		Long:  `Migrate all cache data from one backend to another. Supported backends: local, sftp, s3`,
	}

	var sourceBackend string
	var destBackend string
	var sourceConfig map[string]string
	var destConfig map[string]string

	cmd.Flags().StringVar(&sourceBackend, "from", "", "Source backend type (local, sftp, s3)")
	cmd.Flags().StringVar(&destBackend, "to", "", "Destination backend type (local, sftp, s3)")
	cmd.Flags().StringToStringVar(&sourceConfig, "source-config", nil, "Source backend configuration (key=value pairs)")
	cmd.Flags().StringToStringVar(&destConfig, "dest-config", nil, "Destination backend configuration (key=value pairs)")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		// Create source backend
		sourceCache, err := createBackend(sourceBackend, sourceConfig, *dataDirectory)
		if err != nil {
			cmd.PrintErrf("Failed to create source backend: %v\n", err)
			os.Exit(1)
		}

		// Create destination backend
		destCache, err := createBackend(destBackend, destConfig, *dataDirectory)
		if err != nil {
			cmd.PrintErrf("Failed to create destination backend: %v\n", err)
			os.Exit(1)
		}

		// Perform migration
		if err := migrateBackends(sourceCache, destCache, cmd); err != nil {
			cmd.PrintErrf("Migration failed: %v\n", err)
			os.Exit(1)
		}

		cmd.Println("Migration completed successfully!")
	}

	return cmd
}

func createBackend(backendType string, config map[string]string, dataDir string) (filestore.CacheBackend, error) {
	cacheConfig := &filestore.CacheConfig{BackendType: backendType}

	switch backendType {
	case "local":
		path := config["path"]
		if path == "" {
			path = filepath.Join(dataDir, "cache")
		}
		return filestore.NewLocalFileSystemAdapter(path), nil

	case "sftp":
		cacheConfig.SFTPHost = config["host"]
		cacheConfig.SFTPPort = 22
		if port := config["port"]; port != "" {
			if p, err := strconv.Atoi(port); err == nil {
				cacheConfig.SFTPPort = p
			}
		}
		cacheConfig.SFTPUsername = config["username"]
		cacheConfig.SFTPPassword = config["password"]
		cacheConfig.SFTPKeyFile = config["key_file"]
		cacheConfig.SFTPBasePath = config["base_path"]

		if err := cacheConfig.Validate(); err != nil {
			return nil, err
		}
		return filestore.NewSFTPAdapter(filestore.SFTPConfig{
			Host:     cacheConfig.SFTPHost,
			Port:     cacheConfig.SFTPPort,
			Username: cacheConfig.SFTPUsername,
			Password: cacheConfig.SFTPPassword,
			KeyFile:  cacheConfig.SFTPKeyFile,
			BasePath: cacheConfig.SFTPBasePath,
		})

	case "s3":
		cacheConfig.S3Bucket = config["bucket"]
		cacheConfig.S3Region = config["region"]
		cacheConfig.S3Endpoint = config["endpoint"]
		cacheConfig.S3BasePath = config["base_path"]

		if err := cacheConfig.Validate(); err != nil {
			return nil, err
		}
		return filestore.NewS3Adapter(filestore.S3Config{
			Bucket:   cacheConfig.S3Bucket,
			BasePath: cacheConfig.S3BasePath,
			Region:   cacheConfig.S3Region,
			Endpoint: cacheConfig.S3Endpoint,
		})

	default:
		return nil, fmt.Errorf("unsupported backend type: %s", backendType)
	}
}

func migrateBackends(source, dest filestore.CacheBackend, cmd *cobra.Command) error {
	// List all files in source backend recursively
	files, err := listAllFiles(source, "")
	if err != nil {
		return fmt.Errorf("failed to list source files: %w", err)
	}

	totalFiles := len(files)
	cmd.Printf("Found %d files to migrate\n", totalFiles)

	migrated := 0
	for _, file := range files {
		cmd.Printf("Migrating: %s\n", file)

		// Load file from source
		reader, err := source.LoadReader(file)
		if err != nil {
			return fmt.Errorf("failed to load file %s: %w", file, err)
		}

		// Save file to destination
		if err := dest.SaveReader(file, reader); err != nil {
			reader.Close()
			return fmt.Errorf("failed to save file %s: %w", file, err)
		}
		reader.Close()

		migrated++
		cmd.Printf("Progress: %d/%d files migrated\n", migrated, totalFiles)
	}

	return nil
}

func listAllFiles(backend filestore.CacheBackend, prefix string) ([]string, error) {
	var files []string

	dirs := []string{""}
	for len(dirs) > 0 {
		currentDir := dirs[0]
		dirs = dirs[1:]

		fullPath := filepath.Join(prefix, currentDir)
		if fullPath != "" && !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}

		entries, err := backend.List(fullPath)
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			entryPath := filepath.Join(fullPath, entry)
			if strings.HasSuffix(entry, "/") {
				// It's a directory
				dirs = append(dirs, strings.TrimSuffix(entryPath, "/"))
			} else {
				// It's a file
				files = append(files, entryPath)
			}
		}
	}

	return files, nil
}