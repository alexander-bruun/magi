//go:build windows

package models

// getDiskUsageLinux is not available on Windows
func getDiskUsageLinux() ([]DiskStats, error) {
	return []DiskStats{}, nil
}
