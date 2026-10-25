// Command gowork —— 项目跳板 + 环境仪表盘。
//
// gowork 负责目标目录解析、仪表盘渲染与仓库克隆等全部逻辑，跨 Windows / macOS / Linux；
// 各 shell 的集成代码由 gowork init 生成，只做一件事：在当前会话里切换目录。
//
// 为什么需要 shell 集成：子进程改不了父 shell 的工作目录，所以目标目录由 --cwd-file
// 回传给 shell 侧，由它执行切换。可执行文件也能单独跑，只是不会切换目录。
package main

import (
	"os"

	"gowork/internal/cli"
)

func main() {
	cli.Main(os.Args)
}
