package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func (s *Server) calculateUsedSpace() int64 {
	var total int64
	_ = filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// getDiskUsage returns total bytes, free bytes, and usage percentage for the disk
func (s *Server) getDiskUsage() (total int64, free int64, usedPct int) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.basePath, &stat); err != nil {
		return 0, 0, 0
	}

	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	used := total - free

	if total > 0 {
		usedPct = int((used * 100) / total)
	}

	return total, free, usedPct
}

// checkDiskSpace returns true if there's enough disk space for the write
func (s *Server) checkDiskSpace(bytesToWrite int64) (bool, string) {
	_, free, usedPct := s.getDiskUsage()

	// Check if write would exceed max disk usage
	if usedPct >= s.maxDiskUsagePct {
		return false, fmt.Sprintf("disk usage at %d%% (max %d%%)", usedPct, s.maxDiskUsagePct)
	}

	// Check if there's enough free space (with some buffer)
	minFreeBytes := int64(100 * 1024 * 1024) // 100MB minimum
	if free-bytesToWrite < minFreeBytes {
		return false, fmt.Sprintf("insufficient disk space: %d bytes free, need %d", free, bytesToWrite+minFreeBytes)
	}

	return true, ""
}
