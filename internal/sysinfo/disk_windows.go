//go:build windows

package sysinfo

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// DiskUsage 返回路径所在卷的空间信息与卷标识（如 C:）。
func DiskUsage(path string) (DiskSpace, bool) {
	volume := filepath.VolumeName(path)
	if volume == "" {
		return DiskSpace{}, false
	}
	// 盘符统一成大写，与 Windows 的显示习惯一致。
	if len(volume) == 2 && volume[1] == ':' {
		volume = strings.ToUpper(volume[:1]) + ":"
	}
	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return DiskSpace{}, false
	}

	var freeToCaller, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(root, &freeToCaller, &total, &totalFree); err != nil {
		return DiskSpace{}, false
	}
	return DiskSpace{
		FreeGB:  float64(freeToCaller) / (1 << 30),
		TotalGB: float64(total) / (1 << 30),
		Volume:  volume,
	}, true
}
