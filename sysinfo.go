package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
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

// fail 把错误以红字写到 stderr，然后按给定退出码结束进程。
func fail(err error, code int) {
	msg := err.Error()
	if term.IsTerminal(int(os.Stderr.Fd())) {
		msg = "\x1b[91m" + msg + "\x1b[0m"
	}
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}

func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
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
