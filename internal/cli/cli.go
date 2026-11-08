// Package cli 负责命令行入口：参数解析、编排与错误退出。
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	// UsedFlags 是 flag 实际解析到的选项（规范名）；重排后项目名之后的选项也在其中。
	UsedFlags map[string]bool
	// ProjectAfterTerminator 表示项目名来自 `--` 之后的第一个位置参数；
	// 此时按普通名字匹配，不把 -N 解释成索引。
	ProjectAfterTerminator bool
}

// valueFlags 是会用掉下一个参数的非布尔选项。改写 -N 时要跳过它们后面那个值，
// 否则 `--project -1` 会被误当成裸写的索引而改坏。
var valueFlags = map[string]bool{
	"-project": true, "--project": true,
	"-recent": true, "--recent": true,
	"-indent": true, "--indent": true,
	"-cwd-file": true, "--cwd-file": true,
	"-workspace": true, "--workspace": true,
}

// 短选项与长选项一一对应。h 没有注册成真正的 flag，交给 flag 包内置的
// help 处理；这里只负责把它从组合里拆出来。
var (
	shortFlags = map[byte]string{
		'c': "cwd-file",
		'h': "help",
		'i': "indent",
		'n': "no-color",
		'p': "project",
		'w': "workspace",
	}
	shortBoolFlags = map[byte]bool{
		'h': true,
		'n': true,
	}
)

// longFlags 是注册过的长选项。
var longFlags = map[string]bool{
	"project":   true,
	"recent":    true,
	"indent":    true,
	"cwd-file":  true,
	"workspace": true,
	"no-color":  true,
	"help":      true,
}

// collisionIgnored 的选项不参与“选项名与目录同名”提示：
// cwd-file 由 shell 函数自动注入，help 会直接退出。
var collisionIgnored = map[string]bool{
	"cwd-file": true,
	"help":     true,
}

// ExpandShortFlags 把类 Unix 的短选项写法展开成长选项：
// `-n` → `--no-color`，`-ni 5` / `-ni5` → `--no-color --indent 5` / `--no-color --indent=5`。
// 只有整段都由已知短选项组成时才展开。`-1` 交给后面的索引改写；`-no-color` 这类
// 单横线长选项和未知选项原样保留，由 flag 包处理（flag 同时接受单横线与双横线长选项）。
// `--` 之后的参数不再展开；但 `--workspace --` 里的 `--` 是选项值，不算终止符。
func ExpandShortFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i > 0 && valueFlags[args[i-1]] && !strings.Contains(args[i-1], "=") {
				out = append(out, arg)
				continue
			}
			out = append(out, args[i:]...)
			break
		}
		body, ok := shortBody(arg)
		if !ok {
			out = append(out, arg)
			continue
		}

		expanded, consumed, ok := expandBundle(body, args, i)
		if !ok {
			// 组合里出现未知字符，整段按原样交给 flag 包报错。
			out = append(out, arg)
			continue
		}
		out = append(out, expanded...)
		i += consumed
	}
	return out
}

// shortBody 返回单横线短选项的选项体；`--`、单独的 `-` 与 `-N` 索引不在此列。
func shortBody(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return "", false
	}
	return arg[1:], true
}

// expandBundle 展开一段短选项体，返回展开结果与额外消耗的参数个数。
// 带值选项会吃掉组合剩余部分（贴值）或下一个参数（独立值）；找不到值时就
// 只输出长选项名，让 flag 包统一报“缺少参数”。
func expandBundle(body string, args []string, i int) (out []string, consumed int, ok bool) {
	for j := 0; j < len(body); j++ {
		c := body[j]
		name, found := shortFlags[c]
		if !found {
			return nil, 0, false
		}

		if shortBoolFlags[c] {
			out = append(out, "--"+name)
			continue
		}

		if rest := body[j+1:]; rest != "" {
			return append(out, "--"+name+"="+rest), 0, true
		}
		if i+1 < len(args) {
			return append(out, "--"+name, args[i+1]), 1, true
		}
		return append(out, "--"+name), 0, true
	}
	return out, 0, true
}

// NormalizeArgs 在解析前统一处理参数：先展开短选项，再把 `--` 之前的
// 裸 -N 改写成 --recent=N，最后把位置参数之后的选项重排到前面。
func NormalizeArgs(args []string) []string {
	return ReorderArgs(RewriteIndexArgs(ExpandShortFlags(args)))
}

// RewriteIndexArgs 把裸写的 -N 改写成 --recent=N，其余参数原样保留。
func RewriteIndexArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// "--" 后的参数不改写
		if arg == "--" {
			out = append(out, args[i:]...)
			break
		}
		if project.IndexPattern.MatchString(arg) {
			arg = "--recent=" + strings.TrimPrefix(arg, "-")
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

// ReorderArgs 把位置参数之后的选项移到位置参数之前，使
// `work my-app -n` 与 `work -n my-app` 等价；`--` 之后全部视为位置参数。
// 目标选项也参与重排，这样 `项目名 + --recent` 会在解析阶段报冲突，而不是静默忽略。
// 未知的选项形态也移到前面，交给 flag 包报错，而不是在位置参数之后被吞掉。
func ReorderArgs(args []string) []string {
	pos := firstPositional(args)
	if pos < 0 {
		return args
	}
	out := make([]string, 0, len(args))
	out = append(out, args[:pos]...)
	rest := make([]string, 0, len(args)-pos)
	rest = append(rest, args[pos])
	for i := pos + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		if !isFlagToken(arg) {
			rest = append(rest, arg)
			continue
		}
		_, takesValue, known := reorderOption(arg)
		out = append(out, arg)
		if known && takesValue && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			out = append(out, args[i])
		}
	}
	return append(out, rest...)
}

// reorderOption 识别展开后的长选项，返回规范名、是否带独立值、是否已知。
func reorderOption(arg string) (name string, takesValue, known bool) {
	if !isFlagToken(arg) || arg == "--" {
		return "", false, false
	}
	name = strings.TrimPrefix(arg, "-")
	name = strings.TrimPrefix(name, "-")
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	if !longFlags[name] {
		return "", false, false
	}
	return name, valueFlags["--"+name], true
}

// firstPositional 返回第一个位置参数的下标；`--` 之前没有位置参数时返回 -1。
// 已知带值选项会跳过它的值，避免把 `--indent -5` 的 -5 当成位置参数。
func firstPositional(args []string) int {
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			return -1
		}
		_, takesValue, known := reorderOption(args[i])
		if known {
			if takesValue && !strings.Contains(args[i], "=") && i+1 < len(args) {
				i++
			}
			continue
		}
		if isFlagToken(args[i]) {
			continue
		}
		return i
	}
	return -1
}

// isFlagToken 判断 token 是否像选项；单独的 "-" 与 "--" 不算。
func isFlagToken(arg string) bool {
	return len(arg) > 1 && arg[0] == '-' && arg != "--"
}

// collisionWarnings 汇总 CLI 语义造成的同名歧义：init 子命令与选项名。
// -N 索引的歧义由 project.Resolve 负责，因为那一层本来就要解释索引。
func collisionWarnings(projectArg string, args []string, used map[string]bool, projects []project.Project) []project.Warning {
	var warnings []project.Warning
	if w, ok := initCollisionWarning(projectArg, projects); ok {
		warnings = append(warnings, w)
	}
	warnings = append(warnings, optionCollisionWarnings(args, used, projects)...)
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
func optionCollisionWarnings(args []string, used map[string]bool, projects []project.Project) []project.Warning {
	seen := make(map[string]bool)
	var warnings []project.Warning
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		spelling, names, ok := optionToken(arg)
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
		if optionConsumesNext(arg) && i+1 < len(args) {
			i++
		}
	}
	return warnings
}

// optionConsumesNext 判断这个选项是否会吃掉下一个参数作为值。
// 支持长选项、单横线长选项、短选项单独值，以及组合中最后一个带值短选项。
func optionConsumesNext(arg string) bool {
	if !isFlagToken(arg) || arg == "--" {
		return false
	}
	if _, takesValue, known := reorderOption(arg); known {
		return takesValue && !strings.Contains(arg, "=")
	}
	body, ok := shortBody(arg)
	if !ok {
		return false
	}
	for j := 0; j < len(body); j++ {
		c := body[j]
		if _, found := shortFlags[c]; !found {
			return false
		}
		if shortBoolFlags[c] {
			continue
		}
		return body[j+1:] == ""
	}
	return false
}

// optionToken 把一段选项写法拆成“用户实际写下的拼写”与规范选项名。
// 短选项组合只取到第一个带值选项为止：-ni5 → 拼写 -ni、规范名 no-color/indent。
func optionToken(arg string) (spelling string, names []string, ok bool) {
	if arg == "" || arg == "-" || arg == "--" {
		return "", nil, false
	}

	if strings.HasPrefix(arg, "--") {
		body := arg[2:]
		name := body
		if i := strings.IndexByte(body, '='); i >= 0 {
			name = body[:i]
		}
		if !longFlags[name] {
			return "", nil, false
		}
		return arg[:2+len(name)], []string{name}, true
	}
	if !strings.HasPrefix(arg, "-") {
		return "", nil, false
	}

	body := arg[1:]
	name := body
	if i := strings.IndexByte(body, '='); i >= 0 {
		name = body[:i]
	}
	if longFlags[name] {
		return "-" + name, []string{name}, true
	}

	end := len(body)
	for i := 0; i < len(body); i++ {
		n, found := shortFlags[body[i]]
		if !found {
			return "", nil, false
		}
		names = append(names, n)
		if !shortBoolFlags[body[i]] {
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
func parseWith(fs *flag.FlagSet, args []string, cfg config.Config) (Options, error) {
	o := Options{Config: cfg, Args: args}

	fs.StringVar(&o.Project, "project", "", "要跳转的项目：完整名称、唯一前缀、-N（按最近修改排序的第 N 个），或仓库地址（自动克隆后进入）")
	fs.IntVar(&o.Recent, "recent", 0, "按最近修改排序的第 N 个项目（1 起，等价于 -N）")
	fs.IntVar(&o.Config.Indent, "indent", cfg.Indent, "左侧缩进空格数")
	fs.StringVar(&o.CwdFile, "cwd-file", "", "成功后把目标目录写入该文件，供 shell 函数读取")
	fs.StringVar(&o.Workspace, "workspace", "", "工作区根目录；留空则依次读环境变量 GOWORK_WORKSPACE 与配置文件")
	fs.BoolVar(&o.Config.NoColor, "no-color", cfg.NoColor, "禁用 ANSI 颜色")
	fs.BoolVar(&o.Help, "help", false, "显示帮助并退出")
	fs.Usage = func() {
		writeHelp(fs.Output())
	}

	// 统一在解析前做参数归一化，同时保留原始 Args 供冲突提示使用。
	normalized := NormalizeArgs(args)
	if err := fs.Parse(normalized); err != nil {
		return o, err
	}
	o.UsedFlags = make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { o.UsedFlags[f.Name] = true })

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
			o.ProjectAfterTerminator = o.Project != "" && projectAfterTerminator(normalized, fs.Args())
		}
	}
	if o.Config.Indent < config.MinIndent {
		return o, fmt.Errorf("--indent=%d 小于最小值 %d", o.Config.Indent, config.MinIndent)
	}
	return o, nil
}

// projectAfterTerminator 判断第一个位置参数是否紧跟在 `--` 之后。
// flag.Args() 是 normalized 的后缀，k 指向第一个位置参数；k-1 为 `--` 才算终止符。
// `--` 也可能是上一个带值选项的值（如 `--workspace -- -1`），这时不算。
func projectAfterTerminator(normalized, remaining []string) bool {
	k := len(normalized) - len(remaining)
	if k <= 0 || k >= len(normalized) || normalized[k-1] != "--" {
		return false
	}
	if k >= 2 && valueFlags[normalized[k-2]] {
		return false
	}
	return true
}

// Parse 解析命令行参数；ContinueOnError 便于测试，正式入口用 ExitOnError。
func Parse(args []string, cfg config.Config) (Options, error) {
	fs := flag.NewFlagSet("gowork", flag.ContinueOnError)
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

	// parseWith 内部会先展开短选项、再改写裸 -N（-1 = 最近修改的第一个）。
	// Go 的 flag 包会把裸写的 -1 当成未知选项而报参数错误；统一改写后，
	// PowerShell 与 POSIX 的 shell 函数、直接调 exe 三种入口的行为就一致了。
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)
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

	warnings = append(warnings, collisionWarnings(o.Project, o.Args, o.UsedFlags, projects)...)
	r.Render(target, projects, warnings)

	if o.CwdFile != "" {
		if err := os.WriteFile(o.CwdFile, []byte(target), 0o644); err != nil {
			return fmt.Errorf("无法把目标目录写入 %s：%w", o.CwdFile, err)
		}
	}
	return nil
}
