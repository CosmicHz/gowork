package cli

import (
	"fmt"
	"io"
)

// writeHelp 输出 gowork 的帮助。选项部分手写而不是交给 pflag.FlagUsages：
// 它会把布尔项写成 [=true|false]、用 Go 类型当占位符，也描述不了
// 短选项别名、组合写法、索引短选项和 init 子命令。
// 左列保持纯 ASCII，才能用 len 对齐；中文说明放在右列，不参与列宽计算。
func writeHelp(w io.Writer) {
	fmt.Fprint(w, `用法: gowork [选项] [项目]
      gowork init <SHELL>

进入工作区内的项目并显示仪表盘。项目可写完整名称、唯一前缀或仓库地址。

选项:
`)
	writeHelpRows(w, [][2]string{
		{"-p, --project <PROJECT>", "要跳转的项目：完整名称、唯一前缀或仓库地址"},
		{"-1..-9, --recent <N>", "按最近修改排序的第 N 个项目"},
		{"-i, --indent <N>", "左侧缩进空格数（默认 3，最小 3）"},
		{"-w, --workspace <DIR>", "本次使用的工作区"},
		{"-n, --no-color", "禁用 ANSI 颜色"},
		{"-h, --help", "显示帮助并退出"},
	})
	fmt.Fprint(w, "\n子命令:\n")
	writeHelpRows(w, [][2]string{
		{"init <SHELL>", "输出 shell 集成代码：powershell、zsh、sh、fish"},
	})
	fmt.Fprint(w, `
只允许用一种方式指定一个项目。
通过 work 调用时 init 或 complete 等子命令被当作项目名，这些子命令需直接运行 gowork。
`)
}

// writeHelpRows 按 ASCII 左列宽度对齐输出两列表格。
func writeHelpRows(w io.Writer, rows [][2]string) {
	width := 0
	for _, row := range rows {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	width += 2
	for _, row := range rows {
		fmt.Fprintf(w, "  %-*s%s\n", width, row[0], row[1])
	}
}
