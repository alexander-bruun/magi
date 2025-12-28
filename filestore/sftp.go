package filestore

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SFTPAdapter implements DataBackend for SFTP storage
type SFTPAdapter struct {
	client   *sftp.Client
	basePath string
}

// SFTPConfig holds SFTP connection configuration
type SFTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string // or use KeyFile
	KeyFile  string
	HostKey  string // SSH host public key for verification
	BasePath string
}

// NewSFTPAdapter creates a new SFTP data adapter
func NewSFTPAdapter(config SFTPConfig) (*SFTPAdapter, error) {
	var auth []ssh.AuthMethod

	if config.KeyFile != "" {
		keyBytes, err := os.ReadFile(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		key, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else if config.Password != "" {
		auth = []ssh.AuthMethod{ssh.Password(config.Password)}
	} else {
		return nil, fmt.Errorf("either password or key file must be provided")
	}

	var hostKeyCallback ssh.HostKeyCallback
	var err error
	if config.HostKey != "" {
		// Parse the provided host key for pinning
		hostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(config.HostKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse host key: %w", err)
		}
		hostKeyCallback = ssh.FixedHostKey(hostKey)
	} else {
		// Fall back to checking system's known_hosts file
		hostKeyCallback, err = knownhosts.New(os.ExpandEnv("$HOME/.ssh/known_hosts"))
		if err != nil {
			return nil, fmt.Errorf("failed to load known_hosts: %w", err)
		}
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return &SFTPAdapter{
		client:   sftpClient,
		basePath: config.BasePath,
	}, nil
}

// Save saves data to the specified path
func (s *SFTPAdapter) Save(path string, data []byte) error {
	return s.SaveReader(path, bytes.NewReader(data))
}

// SaveReader saves data from a reader to the specified path
func (s *SFTPAdapter) SaveReader(path string, reader io.Reader) error {
	fullPath := filepath.Join(s.basePath, path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := s.client.MkdirAll(dir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := s.client.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", fullPath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

// Load loads data from the specified path
func (s *SFTPAdapter) Load(path string) ([]byte, error) {
	reader, err := s.LoadReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// LoadReader returns a reader for the specified path
func (s *SFTPAdapter) LoadReader(path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, path)
	return s.client.Open(fullPath)
}

// Exists checks if a file exists at the specified path
func (s *SFTPAdapter) Exists(path string) (bool, error) {
	fullPath := filepath.Join(s.basePath, path)
	_, err := s.client.Stat(fullPath)
	if err != nil {
		if err == os.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete deletes a file at the specified path
func (s *SFTPAdapter) Delete(path string) error {
	fullPath := filepath.Join(s.basePath, path)
	return s.client.Remove(fullPath)
}

// CreateDir creates a directory at the specified path
func (s *SFTPAdapter) CreateDir(path string) error {
	fullPath := filepath.Join(s.basePath, path)
	return s.client.MkdirAll(fullPath)
}

// List lists files in the specified directory
func (s *SFTPAdapter) List(path string) ([]string, error) {
	fullPath := filepath.Join(s.basePath, path)
	entries, err := s.client.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// Close closes the SFTP connection
func (s *SFTPAdapter) Close() error {
	return s.client.Close()
}
