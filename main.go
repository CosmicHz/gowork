// Command gowork —— 项目跳板 + 环境仪表盘。
//
// gowork.exe 负责目标目录解析、仪表盘渲染与仓库克隆等全部逻辑；
// bin\work.ps1 是 shell 入口，只做一件事：在当前会话里执行 Set-Location。
//
// 为什么需要那个启动器：子进程改不了父 shell 的工作目录，所以目标目录由 --cwd-file
// 回传给 shell 侧，由它执行切换。exe 也能单独跑，只是不会切换目录。
package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	defaultIndent   = 3
	minIndent       = 3
	maxRuleWidth    = 100
	rowLabelWidth   = 12
	recentLimit     = 4
	recentNameWidth = 35
)

func main() {
	// 整个程序直写控制台，先打开 ANSI 解析与 UTF-8 代码页。
	enableVT()

	opts, err := parseFlags()
	if err != nil {
		fail(err, 2)
	}
	if err := run(opts); err != nil {
		fail(err, 1)
	}
}

type options struct {
	project   string
	indent    int
	cwdFile   string
	workspace string
	noColor   bool
}

func parseFlags() (options, error) {
	var o options
	flag.StringVar(&o.project, "project", "", "要跳转的项目：完整名称、唯一前缀、-N（按最近修改排序的第 N 个），或仓库地址（自动克隆后进入）")
	flag.IntVar(&o.indent, "indent", defaultIndent, "左侧缩进空格数")
	flag.StringVar(&o.cwdFile, "cwd-file", "", "成功后把目标目录写入该文件，供 shell 启动器读取")
	flag.StringVar(&o.workspace, "workspace", "", "工作区根目录；留空则依次读环境变量 "+workspaceEnv+" 与配置文件")
	flag.BoolVar(&o.noColor, "no-color", false, "禁用 ANSI 颜色")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "用法: gowork [选项] [项目]\n\n选项:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if o.project == "" {
		o.project = flag.Arg(0)
	}
	if o.indent < minIndent {
		return o, fmt.Errorf("--indent 需要不小于 %d", minIndent)
	}
	return o, nil
}

func run(o options) error {
	enableVT()

	workspace, err := resolveWorkspace(o.workspace)
	if err != nil {
		return err
	}
	o.workspace = workspace

	r := newRenderer(o)
	projects := listProjects(o.workspace)

	var (
		target   string
		warnings []string
	)

	if repoURL(o.project) {
		if target, err = cloneProject(o, r); err != nil {
			return err
		}
		// 刚克隆下来的目录也要出现在仪表盘里
		projects = listProjects(o.workspace)
	} else {
		if target, warnings, err = resolveTarget(o.workspace, o.project, projects); err != nil {
			return err
		}
	}

	r.render(target, projects, warnings)

	if o.cwdFile != "" {
		if err := os.WriteFile(o.cwdFile, []byte(target), 0o644); err != nil {
			return fmt.Errorf("无法把目标目录写入 %s：%w", o.cwdFile, err)
		}
	}
	return nil
}
