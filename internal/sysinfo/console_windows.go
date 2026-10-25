//go:build windows

package sysinfo

import (
	"os"

	"golang.org/x/sys/windows"
)

const (
	// enableVirtualTerminalProcessing：让控制台把输出当 ANSI 转义序列解析。
	enableVirtualTerminalProcessing = 0x0004
	// cpUTF8：保证框线字符与 emoji 不被按 GBK 解读。
	cpUTF8 = 65001
)

// EnableVT 启用虚拟终端处理，让控制台把输出当 ANSI 转义序列解析。
func EnableVT() {
	stdout := windows.Handle(os.Stdout.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err == nil {
		_ = windows.SetConsoleMode(stdout, mode|enableVirtualTerminalProcessing)
	}
	_ = windows.SetConsoleOutputCP(cpUTF8)
}
