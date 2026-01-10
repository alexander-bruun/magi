package filestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Adapter implements DataBackend for AWS S3 storage
type S3Adapter struct {
	client   *s3.Client
	bucket   string
	basePath string
}

// S3Config holds S3 connection configuration
type S3Config struct {
	Bucket   string
	BasePath string
	Region   string
	Endpoint string // for S3-compatible services like MinIO
}

// NewS3Adapter creates a new S3 data adapter
func NewS3Adapter(s3Config S3Config) (*S3Adapter, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(s3Config.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if s3Config.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(s3Config.Endpoint)
	}

	client := s3.NewFromConfig(cfg)

	return &S3Adapter{
		client:   client,
		bucket:   s3Config.Bucket,
		basePath: s3Config.BasePath,
	}, nil
}

// Save saves data to the specified path
func (s *S3Adapter) Save(path string, data []byte) error {
	return s.SaveReader(path, bytes.NewReader(data))
}

// SaveReader saves data from a reader to the specified path
func (s *S3Adapter) SaveReader(path string, reader io.Reader) error {
	key := s.getKey(path)

	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	})

	return err
}

// Load loads data from the specified path
func (s *S3Adapter) Load(path string) ([]byte, error) {
	reader, err := s.LoadReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// LoadReader returns a reader for the specified path
func (s *S3Adapter) LoadReader(path string) (io.ReadCloser, error) {
	key := s.getKey(path)

	resp, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// Exists checks if a file exists at the specified path
func (s *S3Adapter) Exists(path string) (bool, error) {
	key := s.getKey(path)

	_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if it's a "not found" error
		var notFoundErr *types.NotFound
		if errors.As(err, &notFoundErr) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Delete deletes a file at the specified path
func (s *S3Adapter) Delete(path string) error {
	key := s.getKey(path)

	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	return err
}

// CreateDir creates a directory at the specified path (S3 doesn't have directories, but we can create a placeholder)
func (s *S3Adapter) CreateDir(path string) error {
	// S3 doesn't have directories, but we can create an empty object to represent the directory
	key := s.getKey(path)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte{}),
	})

	return err
}

// List lists files in the specified directory
func (s *S3Adapter) List(path string) ([]string, error) {
	prefix := s.getKey(path)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	resp, err := s.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	var files []string
	for _, obj := range resp.Contents {
		key := aws.ToString(obj.Key)
		// Remove the prefix
		if after, ok := strings.CutPrefix(key, prefix); ok {
			name := after
			// Skip directory markers (keys ending with /)
			if name != "" && !strings.HasSuffix(name, "/") {
				files = append(files, name)
			}
		}
	}

	return files, nil
}

// getKey constructs the full S3 key from the path
func (s *S3Adapter) getKey(path string) string {
	return strings.TrimPrefix(filepath.Join(s.basePath, path), "/")
}
