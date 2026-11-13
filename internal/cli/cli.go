// Package cli 负责命令行入口：参数解析、编排与错误退出。
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"gowork/internal/clone"
	"gowork/internal/config"
	"gowork/internal/project"
	"gowork/internal/render"
	"gowork/internal/shell"
	"gowork/internal/sysinfo"
)

// Options 是命令行解析结果 + 配置文件确定的运行选项。
type Options struct {
	Project   string
	Recent    int
	CwdFile   string
	Workspace string
	Help      bool
	Config    config.Config
	// Args 是归一化前的原始参数，用于识别“选项名与目录同名”的提示。
	Args []string
	// UsedFlags 是 pflag 实际解析到的选项（规范名）；位置参数之后的选项也在其中。
	UsedFlags map[string]bool
	// ProjectAfterTerminator 表示项目名来自 `--` 之后的第一个位置参数；
	// 此时按普通名字匹配，不把 -N 解释成索引。
	ProjectAfterTerminator bool
	// flags 是本次解析用的 FlagSet；同名提示据此读取真实的短选项与取值规则。
	flags *pflag.FlagSet
}

// registerFlags 注册全部命令行选项。pflag 负责短选项、组合写法、贴值以及
// 位置参数之后的选项；-N 改写与同名提示都从这份注册信息读取取值规则。
func registerFlags(fs *pflag.FlagSet, o *Options, cfg config.Config) {
	fs.StringVarP(&o.Project, "project", "p", "", "要跳转的项目：完整名称、唯一前缀、-N（按最近修改排序的第 N 个），或仓库地址（自动克隆后进入）")
	fs.IntVar(&o.Recent, "recent", 0, "按最近修改排序的第 N 个项目（1 起，等价于 -N）")
	fs.IntVarP(&o.Config.Indent, "indent", "i", cfg.Indent, "左侧缩进空格数")
	fs.StringVarP(&o.CwdFile, "cwd-file", "c", "", "成功后把目标目录写入该文件，供 shell 函数读取")
	fs.StringVarP(&o.Workspace, "workspace", "w", "", "工作区根目录；留空则依次读环境变量 GOWORK_WORKSPACE 与配置文件")
	fs.BoolVarP(&o.Config.NoColor, "no-color", "n", cfg.NoColor, "禁用 ANSI 颜色")
	fs.BoolVarP(&o.Help, "help", "h", false, "显示帮助并退出")
	fs.Usage = func() {
		writeHelp(fs.Output())
	}
}

// collisionIgnored 的选项不参与“选项名与目录同名”提示：
// cwd-file 由 shell 函数自动注入，help 会直接退出。
var collisionIgnored = map[string]bool{
	"cwd-file": true,
	"help":     true,
}

// RewriteIndexArgs 在解析前把裸写的 -N 改写成 --recent=N，其余参数原样保留。
// pflag 会把 -1 当成未知 shorthand，所以改写必须先于 Parse；判断某个 token
// 是否会吃掉下一个参数时直接查 pflag 的注册信息，`-p -1` 的值不会被改坏。
func RewriteIndexArgs(fs *pflag.FlagSet, args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i:]...)
			break
		}
		if project.IndexPattern.MatchString(arg) {
			arg = "--recent=" + strings.TrimPrefix(arg, "-")
		}
		out = append(out, arg)

		// `-flag value` 形式：值原样跟在后面，不能当位置参数改写。
		if optionConsumesNext(fs, args[i]) && i+1 < len(args) {
			i++
			out = append(out, args[i])
		}
	}
	return out
}

// projectAfterTerminator 判断第一个位置参数是否紧跟在 `--` 之后。
// 带值选项会先吃掉下一个参数，所以 `--workspace -- -1` 里的 `--` 是值而不是终止符。
func projectAfterTerminator(fs *pflag.FlagSet, args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return i+1 < len(args)
		}
		if optionConsumesNext(fs, arg) {
			i++
			continue
		}
		if isFlagToken(arg) {
			continue
		}
		return false
	}
	return false
}

// isFlagToken 判断 token 是否像选项；单独的 "-" 与 "--" 不算。
func isFlagToken(arg string) bool {
	return len(arg) > 1 && arg[0] == '-' && arg != "--"
}

// collisionWarnings 汇总 CLI 语义造成的同名歧义：init 子命令与选项名。
// -N 索引的歧义由 project.Resolve 负责，因为那一层本来就要解释索引。
func collisionWarnings(fs *pflag.FlagSet, projectArg string, args []string, used map[string]bool, projects []project.Project) []project.Warning {
	var warnings []project.Warning
	if w, ok := initCollisionWarning(projectArg, projects); ok {
		warnings = append(warnings, w)
	}
	warnings = append(warnings, optionCollisionWarnings(fs, args, used, projects)...)
	return warnings
}

// initCollisionWarning 在工作区里存在名为 init 的目录时说明子命令与项目的区别。
func initCollisionWarning(projectArg string, projects []project.Project) (project.Warning, bool) {
	if projectArg != "init" {
		return project.Warning{}, false
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, "init") {
			return project.Warning{
				Message: `"init" 同时是工作区里的目录名与 shell 集成子命令；通过 work 调用时按项目进入`,
				Details: []string{fmt.Sprintf("直接运行 gowork init <shell> 会输出集成代码；要明确进入该目录请写 work ./%s", p.Name)},
			}, true
		}
	}
	return project.Warning{}, false
}

// optionCollisionWarnings 找出被实际解析为选项、且与工作区目录同名的写法。
// 只统计 flag 真正用到的选项；`--` 之后的 token 与带值选项的值都不参与提示。
func optionCollisionWarnings(fs *pflag.FlagSet, args []string, used map[string]bool, projects []project.Project) []project.Warning {
	seen := make(map[string]bool)
	var warnings []project.Warning
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		spelling, names, ok := optionToken(fs, arg)
		if ok && !seen[spelling] && optionUsed(names, used) {
			seen[spelling] = true
			for _, p := range projects {
				if p.Name != spelling {
					continue
				}
				warnings = append(warnings, project.Warning{
					Message: fmt.Sprintf("%q 同时是工作区里的目录名与命令行选项；当前按选项解析", spelling),
					Details: []string{fmt.Sprintf("要进入该目录请写 work ./%s", p.Name)},
				})
			}
		}
		if optionConsumesNext(fs, arg) && i+1 < len(args) {
			i++
		}
	}
	return warnings
}

// optionConsumesNext 判断这个选项是否会吃掉下一个参数作为值。
// 取值规则来自 pflag 的 NoOptDefVal：布尔项可省略值，带值项在没有贴值时会占用下一个参数。
func optionConsumesNext(fs *pflag.FlagSet, arg string) bool {
	if !isFlagToken(arg) {
		return false
	}
	if strings.HasPrefix(arg, "--") {
		body := arg[2:]
		if strings.IndexByte(body, '=') >= 0 {
			return false
		}
		f := fs.Lookup(body)
		return f != nil && f.NoOptDefVal == ""
	}

	body := arg[1:]
	for i := 0; i < len(body); i++ {
		f := fs.ShorthandLookup(string(body[i]))
		if f == nil {
			return false
		}
		if f.NoOptDefVal != "" {
			// `-n=false` 是给布尔项赋值，不再吃下一个参数。
			if i+1 < len(body) && body[i+1] == '=' {
				return false
			}
			continue
		}
		return body[i+1:] == ""
	}
	return false
}

// optionToken 把一段选项写法拆成“用户实际写下的拼写”与规范选项名。
// 短选项组合只取到第一个带值选项为止：-ni5 → 拼写 -ni、规范名 no-color/indent。
// 单横线只表示短选项；`-no-color` 这类写法按 pflag 的规则视为短选项组合。
func optionToken(fs *pflag.FlagSet, arg string) (spelling string, names []string, ok bool) {
	if arg == "" || arg == "-" || arg == "--" {
		return "", nil, false
	}

	if strings.HasPrefix(arg, "--") {
		body := arg[2:]
		name := body
		if i := strings.IndexByte(body, '='); i >= 0 {
			name = body[:i]
		}
		f := fs.Lookup(name)
		if f == nil {
			return "", nil, false
		}
		return arg[:2+len(name)], []string{f.Name}, true
	}
	if !strings.HasPrefix(arg, "-") {
		return "", nil, false
	}

	body := arg[1:]
	end := len(body)
	for i := 0; i < len(body); i++ {
		f := fs.ShorthandLookup(string(body[i]))
		if f == nil {
			return "", nil, false
		}
		names = append(names, f.Name)
		if f.NoOptDefVal == "" {
			end = i + 1
			break
		}
		if i+1 < len(body) && body[i+1] == '=' {
			end = i + 1
			break
		}
	}
	if len(names) == 0 {
		return "", nil, false
	}
	return "-" + body[:end], names, true
}

// optionUsed 判断这组选项里是否有真正生效、且需要参与冲突提示的项。
func optionUsed(names []string, used map[string]bool) bool {
	for _, name := range names {
		if collisionIgnored[name] {
			continue
		}
		if used[name] {
			return true
		}
	}
	return false
}

// parseWith 把命令行解析结果与配置默认值合并。数值与开关先取配置文件的值，
// 命令行显式给定时再覆盖。
func parseWith(fs *pflag.FlagSet, args []string, cfg config.Config) (Options, error) {
	o := Options{Config: cfg, Args: args}
	registerFlags(fs, &o, cfg)

	// 裸 -N 必须先改写：pflag 会把 -1 当成未知 shorthand。
	normalized := RewriteIndexArgs(fs, args)
	if err := fs.Parse(normalized); err != nil {
		return o, err
	}
	o.flags = fs
	o.UsedFlags = make(map[string]bool)
	fs.Visit(func(f *pflag.Flag) { o.UsedFlags[f.Name] = true })

	// 帮助优先：同时给了冲突参数或越界值时也直接显示帮助，而不是报错。
	if o.Help {
		return o, nil
	}

	if o.UsedFlags["project"] && o.UsedFlags["recent"] {
		return o, errors.New("--project 与 --recent 不能同时使用")
	}
	positional := fs.Arg(0)
	if positional != "" && (o.UsedFlags["project"] || o.UsedFlags["recent"]) {
		return o, errors.New("项目名不能与 --project 或 --recent 同时使用")
	}
	if o.UsedFlags["recent"] && o.Recent < 1 {
		return o, fmt.Errorf("--recent=%d 小于最小值 1", o.Recent)
	}
	if o.Project == "" {
		if o.Recent > 0 {
			o.Project = fmt.Sprintf("-%d", o.Recent)
		} else {
			o.Project = positional
			o.ProjectAfterTerminator = o.Project != "" && projectAfterTerminator(fs, normalized)
		}
	}
	if o.Config.Indent < config.MinIndent {
		return o, fmt.Errorf("--indent=%d 小于最小值 %d", o.Config.Indent, config.MinIndent)
	}
	return o, nil
}

// Parse 解析命令行参数；ContinueOnError 便于测试，正式入口用 ExitOnError。
func Parse(args []string, cfg config.Config) (Options, error) {
	fs := pflag.NewFlagSet("gowork", pflag.ContinueOnError)
	return parseWith(fs, args, cfg)
}

// runSubcommand 处理 argv[1] 上的旁路子命令（init / complete），返回是否已处理。
// shell 函数会把 --cwd-file 注入到 argv[1]，所以通过 work 调用时这里永远返回 false，
// 用户写的名字会按项目名解析；子命令只支持直接运行二进制。
func runSubcommand(args []string, out io.Writer) (bool, error) {
	if len(args) < 2 {
		return false, nil
	}
	switch args[1] {
	case "init":
		return true, shell.Init(args[2:], out)
	case "complete":
		return true, completeSubcommand(args[2:], out)
	}
	return false, nil
}

// Main 是命令行的完整流程：init / complete 子命令、裸索引改写、解析与执行。
func Main(args []string) {
	// 整个程序直写控制台，先打开 ANSI 解析与 UTF-8 代码页。
	sysinfo.EnableVT()

	// init / complete 都是只读旁路，不解析项目参数。
	// shell 函数会把 --cwd-file 放在最前面，所以 `work init` / `work complete`
	// 里的名字仍是项目名，不会走到这里。
	if handled, err := runSubcommand(args, os.Stdout); handled {
		if err != nil {
			sysinfo.Fail(err, 2, false)
		}
		return
	}

	// 配置文件坏了也先解析命令行：这样 -h 仍可用，--no-color 也能作用于配置错误。
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}

	// parseWith 内部会先改写裸 -N（-1 = 最近修改的第一个）。
	// pflag 会把裸写的 -1 当成未知 shorthand；统一改写后，PowerShell 与 POSIX
	// 的 shell 函数、直接调 exe 三种入口的行为就一致了。
	// 用 ContinueOnError 自己处理错误：pflag 的 ExitOnError 会把错误打到 stdout。
	fs := pflag.NewFlagSet(args[0], pflag.ContinueOnError)
	o, err := parseWith(fs, args[1:], cfg)
	if err != nil {
		sysinfo.Fail(err, 2, o.Config.NoColor)
	}
	if o.Help {
		writeHelp(os.Stdout)
		return
	}
	if cfgErr != nil {
		sysinfo.Fail(cfgErr, 1, o.Config.NoColor)
	}
	if err := Run(o, cfg); err != nil {
		sysinfo.Fail(err, 1, o.Config.NoColor)
	}
}

// Run 解析工作区、定位目标、渲染仪表盘，成功时把目标目录写入 --cwd-file。
func Run(o Options, cfg config.Config) error {
	sysinfo.EnableVT()

	workspace, err := cfg.ResolveWorkspace(o.Workspace)
	if err != nil {
		return err
	}
	o.Config.Workspace = workspace

	opts := render.DefaultOptions()
	opts.Indent = o.Config.Indent
	opts.Workspace = workspace
	opts.NoColor = o.Config.NoColor
	opts.RecentLimit = o.Config.RecentLimit
	opts.RecentNameWidth = o.Config.RecentNameWidth
	opts.DiskWarnGB = o.Config.DiskWarnGB
	opts.ShowDisk = o.Config.ShowDisk
	opts.ShowGit = o.Config.ShowGit
	opts.ShowHoliday = o.Config.ShowHoliday
	opts.ShowRecent = o.Config.ShowRecent
	opts.ClearScreen = o.Config.ClearScreen
	opts.Quotes = o.Config.Quotes
	r := render.New(opts)
	projects := project.List(workspace)

	var (
		target   string
		warnings []project.Warning
	)

	if clone.IsURL(o.Project) {
		if target, err = clone.Project(workspace, o.Project, r); err != nil {
			return err
		}
		// 刚克隆下来的目录也要出现在仪表盘里
		projects = project.List(workspace)
	} else {
		var resolve func(string, string, []project.Project) (string, []project.Warning, error)
		resolve = project.Resolve
		if o.ProjectAfterTerminator {
			resolve = project.ResolveWithoutIndex
		}
		if target, warnings, err = resolve(workspace, o.Project, projects); err != nil {
			return err
		}
	}

	warnings = append(warnings, collisionWarnings(o.flags, o.Project, o.Args, o.UsedFlags, projects)...)
	r.Render(target, projects, warnings)

	if o.CwdFile != "" {
		if err := os.WriteFile(o.CwdFile, []byte(target), 0o644); err != nil {
			return fmt.Errorf("无法把目标目录写入 %s：%w", o.CwdFile, err)
		}
	}
	return nil
}
