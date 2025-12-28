package filestore

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

// DataConfig holds configuration for data backends
type DataConfig struct {
	BackendType string // "local", "sftp", "s3"

	// Local backend config
	LocalBasePath string

	// SFTP backend config
	SFTPHost     string
	SFTPPort     int
	SFTPUsername string
	SFTPPassword string
	SFTPKeyFile  string
	SFTPHostKey  string
	SFTPBasePath string

	// S3 backend config
	S3Bucket   string
	S3Region   string
	S3Endpoint string
	S3BasePath string
}

// ParseDataConfigFromEnv parses data configuration from environment variables
func ParseDataConfigFromEnv() (*DataConfig, error) {
	config := &DataConfig{
		BackendType: getEnvOrDefault("MAGI_DATA_BACKEND", "local"),
	}

	switch config.BackendType {
	case "local":
		config.LocalBasePath = getEnvOrDefault("MAGI_DATA_LOCAL_PATH", "")
	case "sftp":
		config.SFTPHost = getEnvOrDefault("MAGI_DATA_SFTP_HOST", "")
		if portStr := os.Getenv("MAGI_DATA_SFTP_PORT"); portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid SFTP port: %w", err)
			}
			config.SFTPPort = port
		} else {
			config.SFTPPort = 22
		}
		config.SFTPUsername = getEnvOrDefault("MAGI_DATA_SFTP_USERNAME", "")
		config.SFTPPassword = getEnvOrDefault("MAGI_DATA_SFTP_PASSWORD", "")
		config.SFTPKeyFile = getEnvOrDefault("MAGI_DATA_SFTP_KEY_FILE", "")
		config.SFTPHostKey = getEnvOrDefault("MAGI_DATA_SFTP_HOST_KEY", "")
		config.SFTPBasePath = getEnvOrDefault("MAGI_DATA_SFTP_BASE_PATH", "")
	case "s3":
		config.S3Bucket = getEnvOrDefault("MAGI_DATA_S3_BUCKET", "")
		config.S3Region = getEnvOrDefault("MAGI_DATA_S3_REGION", "")
		config.S3Endpoint = getEnvOrDefault("MAGI_DATA_S3_ENDPOINT", "")
		config.S3BasePath = getEnvOrDefault("MAGI_DATA_S3_BASE_PATH", "")
	default:
		return nil, fmt.Errorf("unsupported data backend type: %s", config.BackendType)
	}

	return config, nil
}

// Validate validates the data configuration
func (c *DataConfig) Validate() error {
	switch c.BackendType {
	case "local":
		if c.LocalBasePath == "" {
			return fmt.Errorf("local base path is required for local backend")
		}
	case "sftp":
		if c.SFTPHost == "" {
			return fmt.Errorf("SFTP host is required")
		}
		if c.SFTPUsername == "" {
			return fmt.Errorf("SFTP username is required")
		}
		if c.SFTPPassword == "" && c.SFTPKeyFile == "" {
			return fmt.Errorf("either SFTP password or key file is required")
		}
	case "s3":
		if c.S3Bucket == "" {
			return fmt.Errorf("S3 bucket is required")
		}
		if c.S3Region == "" {
			return fmt.Errorf("S3 region is required")
		}
	default:
		return fmt.Errorf("unsupported data backend type: %s", c.BackendType)
	}
	return nil
}

// CreateBackend creates a data backend from the configuration
func (c *DataConfig) CreateBackend() (DataBackend, error) {
	switch c.BackendType {
	case "local":
		return NewLocalFileSystemAdapter(c.LocalBasePath), nil
	case "sftp":
		sftpConfig := SFTPConfig{
			Host:     c.SFTPHost,
			Port:     c.SFTPPort,
			Username: c.SFTPUsername,
			Password: c.SFTPPassword,
			KeyFile:  c.SFTPKeyFile,
			HostKey:  c.SFTPHostKey,
			BasePath: c.SFTPBasePath,
		}
		return NewSFTPAdapter(sftpConfig)
	case "s3":
		s3Config := S3Config{
			Bucket:   c.S3Bucket,
			Region:   c.S3Region,
			Endpoint: c.S3Endpoint,
			BasePath: c.S3BasePath,
		}
		return NewS3Adapter(s3Config)
	default:
		return nil, fmt.Errorf("unsupported data backend type: %s", c.BackendType)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// DataBackend defines the interface for data storage backends
type DataBackend interface {
	// Save saves data to the specified path
	Save(path string, data []byte) error

	// SaveReader saves data from a reader to the specified path
	SaveReader(path string, reader io.Reader) error

	// Load loads data from the specified path
	Load(path string) ([]byte, error)

	// LoadReader returns a reader for the specified path
	LoadReader(path string) (io.ReadCloser, error)

	// Exists checks if a file exists at the specified path
	Exists(path string) (bool, error)

	// Delete deletes a file at the specified path
	Delete(path string) error

	// CreateDir creates a directory at the specified path
	CreateDir(path string) error

	// List lists files in the specified directory
	List(path string) ([]string, error)
}

// DataManager manages data operations using different backends
type DataManager struct {
	backend DataBackend
}

// NewDataManager creates a new data manager with the specified backend
func NewDataManager(backend DataBackend) *DataManager {
	return &DataManager{
		backend: backend,
	}
}

// Save saves data to the data store
func (dm *DataManager) Save(path string, data []byte) error {
	return dm.backend.Save(path, data)
}

// SaveReader saves data from a reader to the data store
func (dm *DataManager) SaveReader(path string, reader io.Reader) error {
	return dm.backend.SaveReader(path, reader)
}

// Load loads data from the data store
func (dm *DataManager) Load(path string) ([]byte, error) {
	return dm.backend.Load(path)
}

// LoadReader returns a reader for the data store data
func (dm *DataManager) LoadReader(path string) (io.ReadCloser, error) {
	return dm.backend.LoadReader(path)
}

// Exists checks if a file exists in the data store
func (dm *DataManager) Exists(path string) (bool, error) {
	return dm.backend.Exists(path)
}

// Delete deletes a file from the data store
func (dm *DataManager) Delete(path string) error {
	return dm.backend.Delete(path)
}

// CreateDir creates a directory in the data store
func (dm *DataManager) CreateDir(path string) error {
	return dm.backend.CreateDir(path)
}

// List lists files in a data store directory
func (dm *DataManager) List(path string) ([]string, error) {
	return dm.backend.List(path)
}

// Backend returns the underlying data backend
func (dm *DataManager) Backend() DataBackend {
	return dm.backend
}
