package cli

import (
	"fmt"
	"io"

	"gowork/internal/config"
	"gowork/internal/project"
)

// completeSubcommand 处理 `gowork complete <kind>`，为 shell 补全动态提供候选。
// 它只走读路径：解析工作区、扫描一级目录，不渲染、不克隆、不写 cwd 文件。
func completeSubcommand(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("用法: gowork complete <kind>\n可用: projects")
	}
	switch args[0] {
	case "projects":
		workspace, err := completionWorkspace()
		if err != nil {
			return err
		}
		writeProjectNames(workspace, out)
		return nil
	default:
		return fmt.Errorf("不支持的补全项：%s\n可用: projects", args[0])
	}
}

// completionWorkspace 按常规优先级解析工作区：环境变量 > 配置文件。
// 补全场景没有命令行参数，所以不需要 --workspace。
func completionWorkspace() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return cfg.ResolveWorkspace("")
}

// writeProjectNames 按 project.List 的顺序逐行输出项目名，最近修改的在前。
// 输出用换行分隔，shell 侧按行切分；项目名里的空格不受影响。
func writeProjectNames(workspace string, out io.Writer) {
	for _, p := range project.List(workspace) {
		fmt.Fprintln(out, p.Name)
	}
}
