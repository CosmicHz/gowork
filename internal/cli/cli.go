// Package cli 负责命令行入口：参数解析、编排与错误退出。
package cli

import (
	"flag"
	"fmt"
	"os"

	"gowork/internal/clone"
	"gowork/internal/config"
	"gowork/internal/project"
	"gowork/internal/render"
	"gowork/internal/shell"
	"gowork/internal/sysinfo"
)

const (
	defaultIndent = 3
	minIndent     = 3
)

// Options 是命令行解析结果。
type Options struct {
	Project   string
	Indent    int
	CwdFile   string
	Workspace string
	NoColor   bool
}

// valueFlags 是会用掉下一个参数的非布尔选项。改写 -N 时要跳过它们后面那个值，
// 否则 `--project -1` 会被误当成裸写的索引而改坏。
var valueFlags = map[string]bool{
	"-project": true, "--project": true,
	"-indent": true, "--indent": true,
	"-cwd-file": true, "--cwd-file": true,
	"-workspace": true, "--workspace": true,
}

// RewriteIndexArgs 把裸写的 -N 改写成 --project=-N，其余参数原样保留。
func RewriteIndexArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if project.IndexPattern.MatchString(arg) {
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

// Parse 解析命令行参数。
func Parse() (Options, error) {
	var o Options
	flag.StringVar(&o.Project, "project", "", "要跳转的项目：完整名称、唯一前缀、-N（按最近修改排序的第 N 个），或仓库地址（自动克隆后进入）")
	flag.IntVar(&o.Indent, "indent", defaultIndent, "左侧缩进空格数")
	flag.StringVar(&o.CwdFile, "cwd-file", "", "成功后把目标目录写入该文件，供 shell 函数读取")
	flag.StringVar(&o.Workspace, "workspace", "", "工作区根目录；留空则依次读环境变量 GOWORK_WORKSPACE 与配置文件")
	flag.BoolVar(&o.NoColor, "no-color", false, "禁用 ANSI 颜色")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "用法: gowork [选项] [项目]\n       gowork init <shell>  输出 shell 集成代码\n\n选项:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if o.Project == "" {
		o.Project = flag.Arg(0)
	}
	if o.Indent < minIndent {
		return o, fmt.Errorf("--indent 需要不小于 %d", minIndent)
	}
	return o, nil
}

// Main 是命令行的完整流程：init 子命令、裸索引改写、解析与执行。
func Main(args []string) {
	// 整个程序直写控制台，先打开 ANSI 解析与 UTF-8 代码页。
	sysinfo.EnableVT()

	// init 子命令只打印 shell 集成代码，不解析项目参数。
	if len(args) > 1 && args[1] == "init" {
		if err := shell.Init(args[2:], os.Stdout); err != nil {
			sysinfo.Fail(err, 2, false)
		}
		return
	}

	// `-N` 是本工具的项目索引写法（-1 = 最近修改的第一个），但 Go 的 flag 包会把
	// 裸写的 -1 当成未知选项而报参数错误。先把它改写成 --project=-N，这样 PowerShell
	// 与 POSIX 的 shell 函数、直接调 exe 三种入口的行为就一致了。
	os.Args = append([]string{args[0]}, RewriteIndexArgs(args[1:])...)

	o, err := Parse()
	if err != nil {
		// Parse 出错时也会返回解析结果，--no-color 依然生效。
		sysinfo.Fail(err, 2, o.NoColor)
	}
	if err := Run(o); err != nil {
		sysinfo.Fail(err, 1, o.NoColor)
	}
}

// Run 解析工作区、定位目标、渲染仪表盘，成功时把目标目录写入 --cwd-file。
func Run(o Options) error {
	sysinfo.EnableVT()

	workspace, err := config.Resolve(o.Workspace)
	if err != nil {
		return err
	}
	o.Workspace = workspace

	r := render.New(render.Options{Indent: o.Indent, Workspace: o.Workspace, NoColor: o.NoColor})
	projects := project.List(o.Workspace)

	var (
		target   string
		warnings []string
	)

	if clone.IsURL(o.Project) {
		if target, err = clone.Project(o.Workspace, o.Project, r); err != nil {
			return err
		}
		// 刚克隆下来的目录也要出现在仪表盘里
		projects = project.List(o.Workspace)
	} else {
		if target, warnings, err = project.Resolve(o.Workspace, o.Project, projects); err != nil {
			return err
		}
	}

	r.Render(target, projects, warnings)

	if o.CwdFile != "" {
		if err := os.WriteFile(o.CwdFile, []byte(target), 0o644); err != nil {
			return fmt.Errorf("无法把目标目录写入 %s：%w", o.CwdFile, err)
		}
	}
	return nil
}
