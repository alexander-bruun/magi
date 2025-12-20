package filestore

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

// CacheConfig holds configuration for cache backends
type CacheConfig struct {
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

// ParseCacheConfigFromEnv parses cache configuration from environment variables
func ParseCacheConfigFromEnv() (*CacheConfig, error) {
	config := &CacheConfig{
		BackendType: getEnvOrDefault("MAGI_CACHE_BACKEND", "local"),
	}

	switch config.BackendType {
	case "local":
		config.LocalBasePath = getEnvOrDefault("MAGI_CACHE_LOCAL_PATH", "")
	case "sftp":
		config.SFTPHost = getEnvOrDefault("MAGI_CACHE_SFTP_HOST", "")
		if portStr := os.Getenv("MAGI_CACHE_SFTP_PORT"); portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid SFTP port: %w", err)
			}
			config.SFTPPort = port
		} else {
			config.SFTPPort = 22
		}
		config.SFTPUsername = getEnvOrDefault("MAGI_CACHE_SFTP_USERNAME", "")
		config.SFTPPassword = getEnvOrDefault("MAGI_CACHE_SFTP_PASSWORD", "")
		config.SFTPKeyFile = getEnvOrDefault("MAGI_CACHE_SFTP_KEY_FILE", "")
		config.SFTPHostKey = getEnvOrDefault("MAGI_CACHE_SFTP_HOST_KEY", "")
		config.SFTPBasePath = getEnvOrDefault("MAGI_CACHE_SFTP_BASE_PATH", "")
	case "s3":
		config.S3Bucket = getEnvOrDefault("MAGI_CACHE_S3_BUCKET", "")
		config.S3Region = getEnvOrDefault("MAGI_CACHE_S3_REGION", "")
		config.S3Endpoint = getEnvOrDefault("MAGI_CACHE_S3_ENDPOINT", "")
		config.S3BasePath = getEnvOrDefault("MAGI_CACHE_S3_BASE_PATH", "")
	default:
		return nil, fmt.Errorf("unsupported cache backend type: %s", config.BackendType)
	}

	return config, nil
}

// Validate validates the cache configuration
func (c *CacheConfig) Validate() error {
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
		return fmt.Errorf("unsupported cache backend type: %s", c.BackendType)
	}
	return nil
}

// CreateBackend creates a cache backend from the configuration
func (c *CacheConfig) CreateBackend() (CacheBackend, error) {
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
		return nil, fmt.Errorf("unsupported cache backend type: %s", c.BackendType)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// CacheBackend defines the interface for cache storage backends
type CacheBackend interface {
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

// CacheManager manages cache operations using different backends
type CacheManager struct {
	backend CacheBackend
}

// NewCacheManager creates a new cache manager with the specified backend
func NewCacheManager(backend CacheBackend) *CacheManager {
	return &CacheManager{
		backend: backend,
	}
}

// Save saves data to the cache
func (cm *CacheManager) Save(path string, data []byte) error {
	return cm.backend.Save(path, data)
}

// SaveReader saves data from a reader to the cache
func (cm *CacheManager) SaveReader(path string, reader io.Reader) error {
	return cm.backend.SaveReader(path, reader)
}

// Load loads data from the cache
func (cm *CacheManager) Load(path string) ([]byte, error) {
	return cm.backend.Load(path)
}

// LoadReader returns a reader for the cache data
func (cm *CacheManager) LoadReader(path string) (io.ReadCloser, error) {
	return cm.backend.LoadReader(path)
}

// Exists checks if a file exists in the cache
func (cm *CacheManager) Exists(path string) (bool, error) {
	return cm.backend.Exists(path)
}

// Delete deletes a file from the cache
func (cm *CacheManager) Delete(path string) error {
	return cm.backend.Delete(path)
}

// CreateDir creates a directory in the cache
func (cm *CacheManager) CreateDir(path string) error {
	return cm.backend.CreateDir(path)
}

// List lists files in a cache directory
func (cm *CacheManager) List(path string) ([]string, error) {
	return cm.backend.List(path)
}

// Backend returns the underlying cache backend
func (cm *CacheManager) Backend() CacheBackend {
	return cm.backend
}