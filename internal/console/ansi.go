package console

// 仪表盘、错误输出与提示共用的 ANSI 前景色。颜色是否真正生效由 Console 的
// tty 判定与 noColor 开关决定，这里只提供转义序列。
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
