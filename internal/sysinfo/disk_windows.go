//go:build windows

package sysinfo

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// DiskFreeGB 返回路径所在卷对当前用户可用的空间。
func DiskFreeGB(path string) (float64, bool) {
	volume := filepath.VolumeName(path)
	if volume == "" {
		return 0, false
	}
	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return 0, false
	}

	var freeToCaller, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(root, &freeToCaller, &total, &totalFree); err != nil {
		return 0, false
	}
	return float64(freeToCaller) / (1 << 30), true
}
