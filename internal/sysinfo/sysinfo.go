// Package sysinfo 放跨平台的终端与系统信息辅助函数；平台相关部分在
// console_*.go 与 disk_*.go 里，用构建标签二选一。
package sysinfo

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"gowork/internal/ansi"
)

// ColorEnabled 是全局唯一的颜色开关：目标流必须是真实终端，且没有被命令行或
// NO_COLOR（按约定非空即禁用）关掉。是否终端按流各自判断——stdout 是终端时仪表盘
// 带色，stderr 被重定向时错误信息仍是纯文本。
func ColorEnabled(isTTY, noColor bool) bool {
	return isTTY && !noColor && os.Getenv("NO_COLOR") == ""
}

// Fail 把错误写到 stderr，然后按给定退出码结束进程。noColor 来自命令行开关，
// 与仪表盘共用同一套颜色策略。
func Fail(err error, code int, noColor bool) {
	msg := err.Error()
	if ColorEnabled(term.IsTerminal(int(os.Stderr.Fd())), noColor) {
		msg = ansi.Red + msg + ansi.Reset
	}
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}

// TerminalWidth 返回控制台宽度，取不到时按 80 列兜底。
func TerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}
