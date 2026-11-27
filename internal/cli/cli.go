// Package cli 负责命令行入口：参数解析、编排与错误退出。
package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"gowork/internal/clone"
	"gowork/internal/config"
	"gowork/internal/console"
	"gowork/internal/holiday"
	"gowork/internal/project"
	"gowork/internal/render"
	"gowork/internal/shell"
)

// Options 是命令行解析结果 + 配置文件确定的运行选项。
type Options struct {
	Project   string
	Recent    int
	CwdFile   string
	Workspace string
	Help      bool
	Config    config.Config
	// Args 是原始参数，用于识别“选项名与目录同名”的提示。
	Args []string
	// UsedFlags 是 pflag 实际解析到的选项（规范名）
	UsedFlags map[string]bool
	// ProjectAfterTerminator 表示项目名来自 `--` 之后；
	// 此时按普通名字匹配，不把 -N 解释成索引。
	ProjectAfterTerminator bool
	// flags 是本次解析用的 FlagSet；同名提示据此读取真实的短选项与取值规则。
	flags *pflag.FlagSet
	// index 记录本次用了哪几个 -N 开关；下标即序号，0 不用。
	index [maxIndexShorthand + 1]bool
}

// maxIndexShorthand 是 -1…-9 这组索引短选项的上限，更靠后的项目用 --recent N 指定。
// 不要将其改为大于 9 的数
const maxIndexShorthand = 9

// registerFlags 注册全部命令行选项。pflag 负责短选项、组合写法、贴值。
// 索引短选项与同名提示都从这份注册信息读取取值规则。
func registerFlags(fs *pflag.FlagSet, o *Options, cfg config.Config) {
	fs.StringVarP(&o.Project, "project", "p", "", "要跳转的项目：完整名称、唯一前缀、-1…-9 或仓库地址")
	fs.IntVar(&o.Recent, "recent", 0, "按最近修改排序的第 N 个项目（1 起，等价于 -1…-9）")
	fs.IntVarP(&o.Config.Indent, "indent", "i", cfg.Indent, "左侧缩进空格数")
	fs.StringVarP(&o.CwdFile, "cwd-file", "c", "", "成功后把目标目录写入该文件，供 shell 函数读取")
	fs.StringVarP(&o.Workspace, "workspace", "w", "", "工作区根目录；留空则依次读环境变量 GOWORK_WORKSPACE 与配置文件")
	fs.BoolVarP(&o.Config.NoColor, "no-color", "n", cfg.NoColor, "禁用 ANSI 颜色")
	fs.BoolVarP(&o.Help, "help", "h", false, "显示帮助并退出")

	// shell 函数注入的隐藏标记：出现时忽略所有子命令。
	// 名字刻意取得不会与工作区目录重名。
	fs.Bool(shellMarker, false, "内部使用：由 shell 函数注入，出现时忽略子命令")
	_ = fs.MarkHidden(shellMarker)

	// -1…-9 是不带值的开关：pflag 对 bool 选项自动设 NoOptDefVal="true"，
	// 所以 -1 不会去抢下一个参数。长名字只为满足注册要求，帮助里隐藏。
	for d := 1; d <= maxIndexShorthand; d++ {
		name := fmt.Sprintf("index%d", d)
		fs.BoolVarP(&o.index[d], name, strconv.Itoa(d), false, fmt.Sprintf("按最近修改排序的第 %d 个项目", d))
		_ = fs.MarkHidden(name)
	}
}

// ignoredOption 判断选项是否不参与“选项名与目录同名”提示：
// cwd-file 由 shell 函数自动注入，help 会直接退出；
// 索引短选项的歧义由 project.Resolve 负责，否则会重复提示。
func ignoredOption(name string) bool {
	switch name {
	case "cwd-file", "help", shellMarker:
		return true
	}
	rest, ok := strings.CutPrefix(name, "index")
	if !ok {
		return false
	}
	d, err := strconv.Atoi(rest)
	return err == nil && d >= 1 && d <= maxIndexShorthand
}

// checkIndexArgs 在解析前拦下短选项表达不了的索引写法。
// pflag 会把 -12 拆成 -1 与 -2 两个开关，把 -11 / -22 这类重复数字静默当成第 1 / 2 个，
// -10 则会因 -0 未注册而报难懂的 “unknown shorthand”。
// 带参选项的值（如 `-p -12`）不会被拦下。
func checkIndexArgs(fs *pflag.FlagSet, args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return nil
		}
		if m := project.LongIndexPattern.FindStringSubmatch(arg); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n < 1 {
				return errors.New("项目索引从 1 开始")
			}
			return fmt.Errorf("%s 可能是笔误：请写 --recent %d", arg, n)
		}
		if m := project.IndexValuePattern.FindStringSubmatch(arg); m != nil {
			return fmt.Errorf("索引 -%s 不接受取值", m[1])
		}
		// -n1=5 这类组合里的索引赋值同样不接受。
		if spelling, names, ok := optionToken(fs, arg); ok &&
			len(arg) > len(spelling) && arg[len(spelling)] == '=' &&
			strings.HasPrefix(names[len(names)-1], "index") {
			return fmt.Errorf("索引 %s 不接受取值", "-"+spelling[len(spelling)-1:])
		}
		if m := project.IndexPattern.FindStringSubmatch(arg); m != nil {
			n, _ := strconv.Atoi(m[1])
			switch {
			case n < 1:
				return errors.New("项目索引从 1 开始")
			case n > maxIndexShorthand:
				return fmt.Errorf("%s 可能是笔误：短选项只支持 -1 到 -%d，第 %d 个请写 --recent %d",
					arg, maxIndexShorthand, n, n)
			}
		}
		if optionConsumesNext(fs, arg) && i+1 < len(args) {
			i++
		}
	}
	return nil
}

// isFlagToken 判断 token 是否像选项；单独的 "-" 与 "--" 不算。
func isFlagToken(arg string) bool {
	return len(arg) > 1 && arg[0] == '-' && arg != "--"
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
		if ignoredOption(name) {
			continue
		}
		if used[name] {
			return true
		}
	}
	return false
}

// projectSelectors 按固定顺序列出本次用到的项目选择器，供冲突提示使用：
// 直接写的项目名、--project、--recent、-N 都算。
// 同时返回用到的索引序号（没用 -N 时为 0）。
func projectSelectors(positional string, used map[string]bool, index [maxIndexShorthand + 1]bool) ([]string, int) {
	var selectors []string
	if positional != "" {
		selectors = append(selectors, "项目名 "+positional)
	}
	if used["project"] {
		selectors = append(selectors, "--project")
	}
	if used["recent"] {
		selectors = append(selectors, "--recent")
	}

	chosen := 0
	for d := 1; d <= maxIndexShorthand; d++ {
		if !index[d] {
			continue
		}
		if chosen == 0 {
			chosen = d
		}
		selectors = append(selectors, fmt.Sprintf("索引 -%d", d))
	}
	return selectors, chosen
}

// joinWords 把列表拼成一句：两项用“和”，更多项用顿号。
func joinWords(items []string) string {
	if len(items) == 2 {
		return items[0] + " 和 " + items[1]
	}
	return strings.Join(items, "、")
}

// parseWith 把命令行解析结果与配置默认值合并。数值与开关先取配置文件的值，
// 命令行显式给定时再覆盖。
func parseWith(fs *pflag.FlagSet, args []string, cfg config.Config) (Options, error) {
	o := Options{Config: cfg, Args: args}
	registerFlags(fs, &o, cfg)
	fs.Usage = func() {
		writeHelp(fs.Output(), !InvokeByShell(args))
	}

	// -10、-12、-0 这类写法 pflag 处理不了或会静默出错，先拦下来。
	if err := checkIndexArgs(fs, args); err != nil {
		return o, err
	}
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	o.flags = fs
	o.UsedFlags = make(map[string]bool)
	fs.Visit(func(f *pflag.Flag) { o.UsedFlags[f.Name] = true })

	// 帮助优先：同时给了冲突参数或越界值时也直接显示帮助，而不是报错。
	if o.Help {
		return o, nil
	}

	// 同一方式写了两次：用户指定了多个项目。
	if rest := fs.Args(); len(rest) > 1 {
		return o, fmt.Errorf("只能指定一个项目，多写了：%s", joinWords(rest[1:]))
	}

	// 用多种方式指定了项目

	positional := fs.Arg(0)
	selectors, index := projectSelectors(positional, o.UsedFlags, o.index)
	if len(selectors) > 1 {
		return o, fmt.Errorf("只能指定一个项目，这次同时用了 %s", joinWords(selectors))
	}
	if o.UsedFlags["recent"] && o.Recent < 1 {
		return o, fmt.Errorf("--recent=%d 小于最小值 1", o.Recent)
	}
	if o.Project == "" {
		switch {
		case index > 0:
			o.Project = fmt.Sprintf("-%d", index)
		case o.Recent > 0:
			o.Project = fmt.Sprintf("-%d", o.Recent)
		default:
			o.Project = positional
			// ArgsLenAtDash 是 `--` 之前的参数个数；为 0 说明项目名来自 `--` 之后。
			o.ProjectAfterTerminator = positional != "" && fs.ArgsLenAtDash() == 0
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

// shellMarker 是 shell 函数注入的隐藏参数，用来区分「经 work 调用」与「直接运行
// gowork」。名字刻意取得不可能与工作区目录重名。
const shellMarker = "__i-am-a-shell-function"

// InvokeByShell 判断是不是 shell 函数发起的调用：用户参数（不含程序名）里
// 出现 shell 标记就是。`--` 之后是项目名，不再当作选项，所以提前结束。
func InvokeByShell(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--"+shellMarker {
			return true
		}
	}
	return false
}

// runSubcommand 处理用户参数里的旁路子命令（init / complete）。
// 经 shell 函数调用时忽略所有子命令；输出经 Console 并带上各自的来源标识，
// 便于 transform 区分。
func runSubcommand(args []string, con *console.Console) (bool, error) {
	if len(args) < 2 {
		return false, nil
	}
	rest := args[1:]
	if InvokeByShell(rest) {
		return false, nil
	}
	// 选项可以写在子命令前面（如 -n），但不能指定项目。
	// 这里只借注册信息判断选项形状，值由下面的 parseOptions 解析。
	fs := pflag.NewFlagSet("gowork", pflag.ContinueOnError)
	registerFlags(fs, &Options{}, config.DefaultConfig())

	i := findSubcommand(fs, rest)
	if i < 0 {
		return false, nil
	}
	// 子命令前后都可能写了选项（i == 0 / i+1 == len(rest) 时是空切片），
	// 统一交给 parseOptions 校验，项目选择器一律报错。
	leading, err := parseOptions(rest[:i])
	if err != nil {
		return true, err
	}
	if leading.Help {
		writeHelp(con.Writer(console.SourceHelp), true)
		return true, nil
	}
	if err := rejectSelectors("前", leading); err != nil {
		return true, err
	}
	trailing, err := parseOptions(rest[i+1:])
	if err != nil {
		return true, err
	}
	if err := rejectSelectors("后", trailing); err != nil {
		return true, err
	}
	switch rest[i] {
	case "init":
		// 输出会被 shell eval，必须原样写出。
		return true, shell.Init(rest[i+1:], con.Writer(console.SourceRaw))
	case "complete":
		// 输出会被 shell 解析，必须原样写出。
		return true, completeSubcommand(rest[i+1:], con.Writer(console.SourceRaw))
	case "prepare":
		return true, prepareSubcommand(rest[i+1:], con)
	}
	return false, nil
}

// exitCodeError 让子命令指定退出码；没有指定时按参数错误（2）处理。
type exitCodeError struct {
	code int
	err  error
}

func (e exitCodeError) Error() string { return e.err.Error() }

func (e exitCodeError) Unwrap() error { return e.err }

// prepareSubcommand 检查节假日缓存，快用完时提前抓取，避免下次运行请求接口。
func prepareSubcommand(args []string, con *console.Console) error {
	if len(args) > 0 {
		return errors.New("prepare 不接受参数")
	}
	now := time.Now()
	h, refreshed, err := holiday.Prepare(now)
	if err != nil {
		return exitCodeError{code: 1, err: fmt.Errorf("准备失败：%w", err)}
	}
	if refreshed {
		con.Println(console.SourceDefault, "已更新节假日缓存：", holiday.Line(h, now))
		return nil
	}
	con.Println(console.SourceDefault, "节假日缓存仍然可用：", holiday.Line(h, now))
	return nil
}

// findSubcommand 返回子命令在用户参数里的位置；没有则返回 -1。
// 跳过选项及其取值，`--` 之后不看；只有第一个位置参数是 init / complete / prepare 才算，
// 所以 `gowork -n init sh` 可用，而 `gowork my-app init` 不会当成子命令。
func findSubcommand(fs *pflag.FlagSet, args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return -1
		}
		if !isFlagToken(arg) {
			if arg == "init" || arg == "complete" || arg == "prepare" {
				return i
			}
			return -1
		}
		if optionConsumesNext(fs, arg) && i+1 < len(args) {
			i++
		}
	}
	return -1
}

// parseOptions 只解析并校验一段选项，位置参数（shell 名、补全项）不参与。
func parseOptions(args []string) (Options, error) {
	fs := pflag.NewFlagSet("gowork", pflag.ContinueOnError)
	o := Options{Config: config.DefaultConfig()}
	registerFlags(fs, &o, o.Config)
	if err := checkIndexArgs(fs, args); err != nil {
		return o, err
	}
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	o.UsedFlags = make(map[string]bool)
	fs.Visit(func(f *pflag.Flag) { o.UsedFlags[f.Name] = true })
	return o, nil
}

// rejectSelectors 拒绝子命令前后的项目选择器；位置参数不参与判断。
// position 填 "前" 或 "后"。用于错误提示。
func rejectSelectors(position string, o Options) error {
	selectors, _ := projectSelectors("", o.UsedFlags, o.index)
	if len(selectors) == 0 {
		return nil
	}
	return fmt.Errorf("子命令%s不能指定项目，多写了：%s", position, joinWords(selectors))
}

// Main 是命令行的完整流程：init / complete 子命令、索引校验、解析与执行。
func Main(args []string) {
	// 所有控制台输出都走这个闸门；构造时同时开启 Windows 的 ANSI 解析与 UTF-8 代码页。
	con := console.New()

	// init / complete 都是只读旁路，不解析项目参数。
	// shell 函数会注入内部参数与 shell 标记，所以 `work init` / `work complete`
	// 里的名字仍是项目名，不会走到这里。
	if handled, err := runSubcommand(args, con); handled {
		if err != nil {
			code := 2
			var ec exitCodeError
			if errors.As(err, &ec) { // 子命令要求指定退出码
				code = ec.code
			}
			con.Fail(err, code)
		}
		return
	}

	// 配置文件坏了也先解析命令行：这样 -h 仍可用，--no-color 也能作用于配置错误。
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}

	// parseWith 内部会先校验索引写法（-10 及以上要改用 --recent），再交给 pflag。
	// 用 ContinueOnError 自己处理错误：pflag 的 ExitOnError 会把错误打到 stdout。
	fs := pflag.NewFlagSet(args[0], pflag.ContinueOnError)
	o, err := parseWith(fs, args[1:], cfg)
	// 控制台配置在这里一次应用；Run 只负责解析与渲染。
	con.NoColor = o.Config.NoColor
	con.SetRainbow(console.RainbowOptions{
		Chance:    o.Config.RainbowChance,
		Freq:      o.Config.RainbowFreq,
		Spread:    o.Config.RainbowSpread,
		PhaseStep: o.Config.RainbowPhaseStep,
	})
	if err != nil {
		con.Fail(err, 2)
	}
	if o.Help {
		writeHelp(con.Writer(console.SourceHelp), !InvokeByShell(args[1:]))
		return
	}
	if cfgErr != nil {
		con.Fail(cfgErr, 1)
	}
	if err := Run(o, cfg, con); err != nil {
		con.Fail(err, 1)
	}
}

// Run 解析工作区、定位目标、渲染仪表盘，成功时把目标目录写入 --cwd-file。
func Run(o Options, cfg config.Config, con *console.Console) error {
	workspace, err := cfg.ResolveWorkspace(o.Workspace)
	if err != nil {
		return err
	}
	o.Config.Workspace = workspace

	opts := render.DefaultOptions()
	opts.Indent = o.Config.Indent
	opts.Workspace = workspace
	opts.RecentLimit = o.Config.RecentLimit
	opts.RecentNameWidth = o.Config.RecentNameWidth
	opts.DiskWarnGB = o.Config.DiskWarnGB
	opts.ShowDisk = o.Config.ShowDisk
	opts.ShowGit = o.Config.ShowGit
	opts.ShowHoliday = o.Config.ShowHoliday
	opts.ShowRecent = o.Config.ShowRecent
	opts.ClearScreen = o.Config.ClearScreen
	opts.Quotes = o.Config.Quotes
	r := render.New(opts, con)
	projects := project.List(workspace)

	var (
		target   string
		warnings []project.Warning
	)

	if clone.IsURL(o.Project) {
		if target, err = clone.Project(workspace, o.Project, r, con); err != nil {
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

	warnings = append(warnings, optionCollisionWarnings(o.flags, o.Args, o.UsedFlags, projects)...)
	r.Render(target, projects, warnings)

	if o.CwdFile != "" {
		if err := os.WriteFile(o.CwdFile, []byte(target), 0o644); err != nil {
			return fmt.Errorf("无法把目标目录写入 %s：%w", o.CwdFile, err)
		}
	}
	return nil
}
