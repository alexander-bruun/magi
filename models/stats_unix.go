//go:build !windows

package models

import (
	"os"
	"strings"
	"syscall"
)

// getDiskUsageLinux retrieves disk usage for Linux
func getDiskUsageLinux() ([]DiskStats, error) {
	var disks []DiskStats

	// Read mount points from /proc/mounts
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	seenDevices := make(map[string]bool)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]

		// Skip if we've already processed this device
		if seenDevices[device] {
			continue
		}

		// Skip pseudo filesystems and non-main mount points
		if strings.HasPrefix(device, "tmpfs") || 
			strings.HasPrefix(device, "devtmpfs") ||
			strings.HasPrefix(device, "none") ||
			strings.HasPrefix(device, "proc") ||
			strings.HasPrefix(device, "sysfs") ||
			strings.HasPrefix(device, "cgroup") ||
			strings.HasPrefix(device, "sunrpc") {
			continue
		}

		// Skip certain mount points
		if strings.HasPrefix(mountPoint, "/sys") || 
			strings.HasPrefix(mountPoint, "/proc") || 
			strings.HasPrefix(mountPoint, "/dev") ||
			strings.HasPrefix(mountPoint, "/run") ||
			strings.HasPrefix(mountPoint, "/boot/efi") ||
			strings.HasPrefix(mountPoint, "/snap") {
			continue
		}

		seenDevices[device] = true

		// Get disk stats using syscall.Statfs
		var statfs syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &statfs); err == nil {
			// Calculate disk statistics
			blockSize := uint64(statfs.Bsize)
			totalBlocks := statfs.Blocks
			availableBlocks := statfs.Bavail
			usedBlocks := totalBlocks - availableBlocks

			totalGB := float64(totalBlocks*blockSize) / (1024 * 1024 * 1024)
			usedGB := float64(usedBlocks*blockSize) / (1024 * 1024 * 1024)
			availableGB := float64(availableBlocks*blockSize) / (1024 * 1024 * 1024)

			usagePercent := 0.0
			if totalBlocks > 0 {
				usagePercent = (float64(usedBlocks) / float64(totalBlocks)) * 100
			}

			// Use device name for display if available
			displayName := device
			if !strings.HasPrefix(device, "/dev/") {
				displayName = mountPoint
			}

			disk := DiskStats{
				Path:         displayName,
				UsedGB:       usedGB,
				TotalGB:      totalGB,
				AvailableGB:  availableGB,
				UsagePercent: usagePercent,
			}
			disks = append(disks, disk)
		}
	}

	return disks, nil
}

// getDiskUsageMacOS retrieves disk usage for macOS
func getDiskUsageMacOS(path string) (DiskStats, error) {
	stats := DiskStats{Path: path}
	
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(path, &statfs); err != nil {
		return stats, err
	}

	blockSize := uint64(statfs.Bsize)
	totalBlocks := statfs.Blocks
	availableBlocks := statfs.Bavail
	usedBlocks := totalBlocks - availableBlocks

	totalGB := float64(totalBlocks*blockSize) / (1024 * 1024 * 1024)
	usedGB := float64(usedBlocks*blockSize) / (1024 * 1024 * 1024)
	availableGB := float64(availableBlocks*blockSize) / (1024 * 1024 * 1024)

	usagePercent := 0.0
	if totalBlocks > 0 {
		usagePercent = (float64(usedBlocks) / float64(totalBlocks)) * 100
	}

	stats.UsedGB = usedGB
	stats.TotalGB = totalGB
	stats.AvailableGB = availableGB
	stats.UsagePercent = usagePercent

	return stats, nil
}

// getDiskUsageUnix is a fallback for other Unix-like systems
func getDiskUsageUnix(path string) (DiskStats, error) {
	stats := DiskStats{Path: path}

	var statfs syscall.Statfs_t
	if err := syscall.Statfs(path, &statfs); err != nil {
		return stats, err
	}

	blockSize := uint64(statfs.Bsize)
	totalBlocks := statfs.Blocks
	availableBlocks := statfs.Bavail
	usedBlocks := totalBlocks - availableBlocks

	totalGB := float64(totalBlocks*blockSize) / (1024 * 1024 * 1024)
	usedGB := float64(usedBlocks*blockSize) / (1024 * 1024 * 1024)
	availableGB := float64(availableBlocks*blockSize) / (1024 * 1024 * 1024)

	usagePercent := 0.0
	if totalBlocks > 0 {
		usagePercent = (float64(usedBlocks) / float64(totalBlocks)) * 100
	}

	stats.UsedGB = usedGB
	stats.TotalGB = totalGB
	stats.AvailableGB = availableGB
	stats.UsagePercent = usagePercent

	return stats, nil
}