package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// 本文件只放跨平台的终端辅助函数；平台相关的部分在
// sysinfo_windows.go 与 sysinfo_posix.go 里，用构建标签二选一。

// colorEnabled 是全局唯一的颜色开关：目标流必须是真实终端，且没有被命令行或
// NO_COLOR（按约定非空即禁用）关掉。是否终端按流各自判断——stdout 是终端时仪表盘
// 带色，stderr 被重定向时错误信息仍是纯文本。
func colorEnabled(isTTY, noColor bool) bool {
	return isTTY && !noColor && os.Getenv("NO_COLOR") == ""
}

// fail 把错误写到 stderr，然后按给定退出码结束进程。noColor 来自命令行开关，
// 与仪表盘共用同一套颜色策略。
func fail(err error, code int, noColor bool) {
	msg := err.Error()
	if colorEnabled(term.IsTerminal(int(os.Stderr.Fd())), noColor) {
		msg = ansiRed + msg + ansiReset
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
