// Package ansi 收录仪表盘与错误输出共用的 ANSI 前景色。
package ansi

const (
	Reset    = "\x1b[0m"
	DarkGray = "\x1b[90m"
	White    = "\x1b[97m"
	Red      = "\x1b[91m"
	Green    = "\x1b[32m"
	Cyan     = "\x1b[36m"
	Yellow   = "\x1b[33m"
	Magenta  = "\x1b[35m"
)

// Palette 是仪表盘每次运行随机挑一个的主色。
var Palette = []string{Cyan, Green, Yellow, Magenta}
