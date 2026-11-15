//go:build linux

package sysinfo

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// mountPoint 从 /proc/self/mounts 里找覆盖 path 的最长挂载点。
func mountPoint(path string, _ *unix.Statfs_t) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return ""
	}
	return mountPointFromTable(abs, string(data))
}
