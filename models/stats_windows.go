//go:build windows

package models

// getDiskUsageLinux is not available on Windows
func getDiskUsageLinux() ([]DiskStats, error) {
	return []DiskStats{}, nil
}

// getDiskUsageMacOS is not available on Windows
func getDiskUsageMacOS(path string) (DiskStats, error) {
	return DiskStats{}, nil
}

// getDiskUsageUnix is not available on Windows
func getDiskUsageUnix(path string) (DiskStats, error) {
	return DiskStats{}, nil
}