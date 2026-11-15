// Package sysinfo 放跨平台的系统信息辅助函数；平台相关部分在
// console_*.go 与 disk_*.go 里，用构建标签二选一。
// 控制台的读写与终端状态由 internal/console 负责，这里只提供它需要的判定与数据。
package sysinfo

import "os"

// ColorEnabled 是全局唯一的颜色开关：目标流必须是真实终端，且没有被命令行或
// NO_COLOR（按约定非空即禁用）关掉。是否终端按流各自判断——stdout 是终端时仪表盘
// 带色，stderr 被重定向时错误信息仍是纯文本。
func ColorEnabled(isTTY, noColor bool) bool {
	return isTTY && !noColor && os.Getenv("NO_COLOR") == ""
}
