package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// 本文件只放跨平台的终端辅助函数；平台相关的部分在
// sysinfo_windows.go 与 sysinfo_posix.go 里，用构建标签二选一。

// fail 把错误以红字写到 stderr，然后按给定退出码结束进程。
func fail(err error, code int) {
	msg := err.Error()
	if term.IsTerminal(int(os.Stderr.Fd())) {
		msg = "\x1b[91m" + msg + "\x1b[0m"
	}
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}

// terminalWidth 返回控制台宽度，取不到时按 80 列兜底。
func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}
