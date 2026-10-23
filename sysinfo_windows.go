//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING：让控制台把输出当 ANSI 转义序列解析。
	enableVirtualTerminalProcessing = 0x0004
	// CP_UTF8：保证框线字符与 emoji 不被按 GBK 解读。
	cpUTF8 = 65001
)

// enableVT 启用虚拟终端处理，让控制台把输出当 ANSI 转义序列解析。
func enableVT() {
	stdout := windows.Handle(os.Stdout.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err == nil {
		_ = windows.SetConsoleMode(stdout, mode|enableVirtualTerminalProcessing)
	}
	_ = windows.SetConsoleOutputCP(cpUTF8)
}

// diskFreeGB 返回路径所在卷对当前用户可用的空间。
func diskFreeGB(path string) (float64, bool) {
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
