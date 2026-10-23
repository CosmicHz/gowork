//go:build !windows

package main

import "golang.org/x/sys/unix"

// enableVT 在 POSIX 终端上不需要额外开关：ANSI 转义序列本来就是默认协议，
// 终端编码也由 locale 决定，不像 Windows 控制台那样要显式切到 UTF-8 代码页。
func enableVT() {}

// diskFreeGB 用 statfs 取路径所在文件系统对当前用户可用的空间。
// macOS 与 Linux 的 Statfs_t 字段类型不同，但 Bavail / Bsize 都在，取浮点相乘即可。
func diskFreeGB(path string) (float64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, false
	}
	// Bavail 是非特权用户可用的块数，Bsize 是每块字节数。
	free := float64(st.Bavail) * float64(st.Bsize)
	return free / (1 << 30), true
}
