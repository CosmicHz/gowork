//go:build !windows

package sysinfo

import "golang.org/x/sys/unix"

// DiskFreeGB 用 statfs 取路径所在文件系统对当前用户可用的空间。
// macOS 与 Linux 的 Statfs_t 字段类型不同，但 Bavail / Bsize 都在，取浮点相乘即可。
func DiskFreeGB(path string) (float64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, false
	}
	// Bavail 是非特权用户可用的块数，Bsize 是每块字节数。
	free := float64(st.Bavail) * float64(st.Bsize)
	return free / (1 << 30), true
}
