//go:build !windows

package sysinfo

import "golang.org/x/sys/unix"

// DiskUsage 用 statfs 取路径所在文件系统的空间信息与挂载点。
func DiskUsage(path string) (DiskSpace, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return DiskSpace{}, false
	}
	// Bavail 是非特权用户可用的块数，Blocks 是总块数，Bsize 是每块字节数。
	return DiskSpace{
		FreeGB:  float64(st.Bavail) * float64(st.Bsize) / (1 << 30),
		TotalGB: float64(st.Blocks) * float64(st.Bsize) / (1 << 30),
		Volume:  mountPoint(path, &st),
	}, true
}
