// Command gowork —— 项目跳板 + 环境仪表盘。
//
// gowork 负责目标目录解析、仪表盘渲染与仓库克隆等全部逻辑，跨 Windows / macOS / Linux；
// 各 shell 的集成代码由 gowork init 生成，只做一件事：在当前会话里切换目录。
//
// 为什么需要 shell 集成：子进程改不了父 shell 的工作目录，所以目标目录由 --cwd-file
// 回传给 shell 侧，由它执行切换。可执行文件也能单独跑，只是不会切换目录。
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

	// init 子命令只打印 shell 集成代码，不解析项目参数。
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := initShell(os.Args[2:], os.Stdout); err != nil {
			fail(err, 2, false)
		}
		return
	}

	// `-N` 是本工具的项目索引写法（-1 = 最近修改的第一个），但 Go 的 flag 包会把
	// 裸写的 -1 当成未知选项而报参数错误。先把它改写成 --project=-N，这样 PowerShell
	// 与 POSIX 的 shell 函数、直接调 exe 三种入口的行为就一致了。
	os.Args = append([]string{os.Args[0]}, rewriteIndexArgs(os.Args[1:])...)

	opts, err := parseFlags()
	if err != nil {
		// parseFlags 出错时也会返回解析结果，--no-color 依然生效。
		fail(err, 2, opts.noColor)
	}
	if err := run(opts); err != nil {
		fail(err, 1, opts.noColor)
	}
}

// valueFlags 是会用掉下一个参数的非布尔选项。改写 -N 时要跳过它们后面那个值，
// 否则 `--project -1` 会被误当成裸写的索引而改坏。
var valueFlags = map[string]bool{
	"-project": true, "--project": true,
	"-indent": true, "--indent": true,
	"-cwd-file": true, "--cwd-file": true,
	"-workspace": true, "--workspace": true,
}

// rewriteIndexArgs 把裸写的 -N 改写成 --project=-N，其余参数原样保留。
func rewriteIndexArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if indexPattern.MatchString(arg) {
			arg = "--project=" + arg
		}
		out = append(out, arg)

		// `-flag value` 形式：值原样跟在后面，不能当位置参数改写。
		if valueFlags[args[i]] && i+1 < len(args) {
			i++
			out = append(out, args[i])
		}
	}
	return out
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
	flag.StringVar(&o.cwdFile, "cwd-file", "", "成功后把目标目录写入该文件，供 shell 函数读取")
	flag.StringVar(&o.workspace, "workspace", "", "工作区根目录；留空则依次读环境变量 "+workspaceEnv+" 与配置文件")
	flag.BoolVar(&o.noColor, "no-color", false, "禁用 ANSI 颜色")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "用法: gowork [选项] [项目]\n       gowork init <shell>  输出 shell 集成代码\n\n选项:\n")
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
