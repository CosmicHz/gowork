// Package render 负责仪表盘的布局、配色与宽度计算。
package render

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"gowork/internal/config"
	"gowork/internal/console"
	"gowork/internal/holiday"
	"gowork/internal/project"
	"gowork/internal/sysinfo"
)

const (
	maxRuleWidth  = 100
	rowLabelWidth = 12
	// bannerSuffixGap 是标题后缀与 logo 之间的空格数。
	bannerSuffixGap = 2
)

const bannerArt = `██╗    ██╗ ██████╗ ██████╗ ██╗  ██╗
██║    ██║██╔═══██╗██╔══██╗██║ ██╔╝
██║ █╗ ██║██║   ██║██████╔╝█████╔╝
██║███╗██║██║   ██║██╔══██╗██╔═██╗
╚███╔███╔╝╚██████╔╝██║  ██║██║  ██╗
 ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝`

var quotes = []string{
	"先让它正确，再让它清晰，最后再让它更快。",
	"复杂度不会消失，只会被转移。",
	"小步提交，让问题永远可定位。",
	"能删除的代码比能优化的代码更有价值。",
	"好的工具不应该打断思路。",
	"今天不写测试，明天就有 bug。",
	"简单是可靠的先决条件。",
	"保持简单，愚蠢。",
	"过早优化是万恶之源。",
	"代码是写给人看的，顺便让机器运行。",
}

// Options 是渲染所需的配置。颜色与终端能力由 console.Console 决定。
type Options struct {
	Indent          int
	Workspace       string
	RecentLimit     int
	RecentNameWidth int
	DiskWarnGB      float64
	ShowDisk        bool
	ShowGit         bool
	ShowHoliday     bool
	ShowRecent      bool
	ClearScreen     bool
	Quotes          []string
}

// DefaultOptions 返回默认渲染配置；调用方从这里起步再覆盖需要的字段。
// 数值默认与 config.DefaultConfig 共用同一组常量，避免两份默认值漂移。
func DefaultOptions() Options {
	return Options{
		Indent:          config.DefaultIndent,
		RecentLimit:     config.DefaultRecentLimit,
		RecentNameWidth: config.DefaultRecentNameWidth,
		DiskWarnGB:      config.DefaultDiskWarnGB,
		ShowDisk:        true,
		ShowGit:         true,
		ShowHoliday:     true,
		ShowRecent:      true,
		ClearScreen:     true,
	}
}

// Renderer 输出仪表盘；所有控制台通信都经 console.Console。
type Renderer struct {
	con             *console.Console
	indent          string
	workspace       string
	recentLimit     int
	recentNameWidth int
	diskWarnGB      float64
	showDisk        bool
	showGit         bool
	showHoliday     bool
	showRecent      bool
	clearScreenOn   bool
	quotes          []string
}

// New 用给定的控制台与配置构造渲染器；宽度、颜色与主色都来自 con。
func New(o Options, con *console.Console) *Renderer {
	if o.RecentLimit < 1 {
		o.RecentLimit = config.DefaultRecentLimit
	}
	if o.RecentNameWidth < config.MinRecentNameWidth {
		o.RecentNameWidth = config.DefaultRecentNameWidth
	}
	if o.DiskWarnGB < 0 {
		o.DiskWarnGB = config.DefaultDiskWarnGB
	}
	if len(o.Quotes) == 0 {
		o.Quotes = quotes
	}

	return &Renderer{
		con:             con,
		indent:          strings.Repeat(" ", o.Indent),
		workspace:       o.Workspace,
		recentLimit:     o.RecentLimit,
		recentNameWidth: o.RecentNameWidth,
		diskWarnGB:      o.DiskWarnGB,
		showDisk:        o.ShowDisk,
		showGit:         o.ShowGit,
		showHoliday:     o.ShowHoliday,
		showRecent:      o.ShowRecent,
		clearScreenOn:   o.ClearScreen,
		quotes:          o.Quotes,
	}
}

// Render 输出一次完整的仪表盘。
func (r *Renderer) Render(target string, projects []project.Project, warnings []project.Warning) {
	now := time.Now()

	// 每次输出前重新探测终端，窗口大小变化后无需重建 Console。
	r.con.Refresh()

	// 清屏后再输出
	if r.clearScreenOn {
		r.clearScreen()
	}
	r.setTitle("WORK | " + filepath.Base(target))

	// 进入具体项目时，把 “ on 项目名” 补在标题的倒数第二行末尾；
	// 名字太长放不下就另起一行，免得折行把 logo 撑破。回工作区根目录时不加。
	name := ""
	if target != r.workspace {
		name = "on " + filepath.Base(target)
	}

	r.println(console.SourceDefault)
	art := strings.Split(bannerArt, "\n")
	placed := false
	for i, line := range art {
		logo := r.indent + line
		if name != "" && i == len(art)-2 {
			need := len(r.indent) + displayWidth(line) + bannerSuffixGap + displayWidth(name)
			if need <= r.con.Width() {
				// logo 与项目名后缀是同一条线上的两段样式，分开写出，各自由来源着色。
				r.con.Print(console.SourceBanner, logo)
				r.con.Print(console.SourceOnProject, strings.Repeat(" ", bannerSuffixGap)+name)
				r.con.Println(console.SourceBanner)
				placed = true
				continue
			}
		}
		r.println(console.SourceBanner, logo)
	}
	if name != "" && !placed {
		r.println(console.SourceOnProject, r.indent+name)
	}
	r.rule()

	r.println(console.SourceGreeting, r.indent+greeting(now))
	r.println(console.SourceDefault)

	r.row("时间", now.Format("2006-01-02")+"  "+weekdayZH(now)+"  "+now.Format("15:04:05"), console.White)
	r.row("位置", target, console.White)
	r.row("项目", fmt.Sprintf("%d 个项目", len(projects)), console.White)

	if r.showDisk {
		if free, ok := sysinfo.DiskFreeGB(target); ok {
			color := console.White
			if free < r.diskWarnGB {
				color = console.Red
			}
			r.row("磁盘", fmt.Sprintf("%.1f GB 可用", free), color)
		}
	}

	if r.showGit {
		if branch, changes, ok := project.GitInfo(target); ok {
			r.row("Git", fmt.Sprintf("%s  ·  %d 项未提交", branch, changes), r.con.Accent())
		}
	}

	if r.showHoliday {
		r.row("节日", holiday.Line(holiday.Next(now), now), console.White)
	}

	if r.showRecent {
		r.printRecent(projects)
	}

	r.println(console.SourceDefault)
	r.rule()
	r.println(console.SourceQuote, r.indent+"  "+r.quotes[rand.IntN(len(r.quotes))])

	r.printWarnings(warnings)
	r.println(console.SourceDefault)
}

// printWarnings 在仪表盘下方逐组输出提示。每组首行带 ⚠，明细缩进，
// 组与组之间空一行，这样前缀歧义与名称冲突同时出现时也不会混在一起。
func (r *Renderer) printWarnings(warnings []project.Warning) {
	for _, w := range warnings {
		r.println(console.SourceDefault)
		r.println(console.SourceWarning, r.indent+"  ⚠ "+w.Message)
		for _, d := range w.Details {
			r.println(console.SourceWarning, r.indent+"     "+d)
		}
	}
}

// CloneNotice 在克隆前清屏并打印地址与目标，让用户知道接下来的 git 输出属于谁。
func (r *Renderer) CloneNotice(url, dest string) {
	if r.clearScreenOn {
		r.clearScreen()
	}
	r.println(console.SourceDefault)
	r.println(console.SourceClone, r.indent+"正在克隆仓库")
	r.row("地址", url, console.White)
	r.row("目标", dest, console.White)
	r.println(console.SourceDefault)
}

// clearScreen 清屏并清掉回滚缓冲；输出被重定向时不做任何事，免得往文件里塞转义序列。
func (r *Renderer) clearScreen() {
	r.con.ClearScreen()
}

// setTitle 设置终端窗口标题。
func (r *Renderer) setTitle(title string) {
	r.con.SetTitle(title)
}

// rule 画分隔线。只限上界：超宽终端下拉满整行会很难看；下界不设，
func (r *Renderer) rule() {
	n := max(0, min(r.con.Width(), maxRuleWidth)-len(r.indent)*2)
	r.println(console.SourceRule, r.indent+strings.Repeat("─", n))
}

// row 输出一行「标签 + 值」，标签按终端视觉宽度对齐到 rowLabelWidth。
// 行内混有多种颜色，由 Paint 精细调控，因此按 SourceRaw 直接写出、不经过 transform。
func (r *Renderer) row(label, value, valueColor string) {
	pad := rowLabelWidth - displayWidth(label)
	if pad < 0 {
		pad = 0
	}
	r.println(console.SourceRaw,
		r.indent+
			r.con.Paint(r.con.Accent(), "   ")+
			r.con.Paint(console.DarkGray, label)+
			strings.Repeat(" ", pad)+
			r.con.Paint(valueColor, value))
}

func (r *Renderer) printRecent(projects []project.Project) {
	recent := projects
	if len(recent) > r.recentLimit {
		recent = recent[:r.recentLimit]
	}
	if len(recent) == 0 {
		return
	}

	r.println(console.SourceDefault)
	r.println(console.SourceRecent, r.indent+"  最近活跃")
	for i, p := range recent {
		name := truncateWidth(p.Name, r.recentNameWidth)
		pad := r.recentNameWidth - displayWidth(name) + 1
		r.println(console.SourceRaw,
			r.indent+
				r.con.Paint(console.DarkGray, fmt.Sprintf("%2d  ", i+1))+
				name+strings.Repeat(" ", pad)+
				r.con.Paint(console.DarkGray, p.Mod.Format("01-02 15:04")))
	}
}

func (r *Renderer) println(source console.Source, parts ...string) {
	r.con.Println(source, parts...)
}

func greeting(now time.Time) string {
	switch h := now.Hour(); {
	case h < 6:
		return "夜深了，注意休息 🌙"
	case h < 12:
		return "早上好，新的一天 ☀️"
	case h < 14:
		return "中午好，别忘了吃饭 🍜"
	case h < 18:
		return "下午好，效率正高 🚀"
	default:
		return "晚上好，收个尾吧 🌆"
	}
}

func weekdayZH(t time.Time) string {
	return [...]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}[t.Weekday()]
}

// displayWidth 估算字符串在终端里占的列数：CJK/全角/emoji 记 2 列。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// runeWidth 返回一个字符占的终端列数，用的是 UAX #11 East Asian Width 的近似：
// 零宽字符交给标准库的真实 Unicode 类别判断，宽字符区段则只能手工列举
// （Go 标准库没有 EAW 属性表）；歧义宽度一律按 1 列，因为它本就取决于终端配置。
// 宽修正让类似 `⚠️` (U+26A0 + U+FE0F) 的 emoji 从"2 列"变成"1 列"。多数 emoji
// 字体把带 VS16 的它画成 2 列，所以这一处现在偏小了一列。
// 不修它是因为要正确处理"emoji + 变体选择符"这种字素簇就得引入更复杂的逻辑。
// 而目前这里没有实际影响：节日那行的值是行尾内容，既不做列对齐也不参与截断，
// 所以 1 还是 2 只影响它自己。同理，只有项目名里出现 ⚠️ 等时 `truncateWidth`
// 才会差一列，可以忽略。
// 不过，作为一个隐患，如果今后还要在别处使用这个函数，就需要认真考虑。
func runeWidth(r rune) int {
	switch {
	case r < 0x20 || r == 0x7f:
		return 0 // 控制字符
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Cf, r):
		// 非间距标记（含变体选择符 U+FE0F）与格式字符（含零宽连接符 U+200D）
		return 0
	case r < 0x1100:
		return 1
	case r <= 0x115f, // 谚文字母
		r == 0x2329, r == 0x232a,
		r >= 0x2e80 && r <= 0xa4cf && r != 0x303f, // CJK 部首 ~ 彝文
		r >= 0xac00 && r <= 0xd7a3,                // 谚文音节
		r >= 0xf900 && r <= 0xfaff,                // CJK 兼容表意文字
		r >= 0xfe30 && r <= 0xfe6f,                // CJK 兼容形式
		r >= 0xff00 && r <= 0xff60,                // 全角形式
		r >= 0xffe0 && r <= 0xffe6,                // 全角符号
		r >= 0x1f300 && r <= 0x1faff,              // emoji
		r >= 0x20000 && r <= 0x3fffd:              // CJK 扩展 B 及以后
		return 2
	}
	return 1
}

// truncateWidth 按显示宽度截断，超出部分用 ... 收尾。
func truncateWidth(s string, max int) string {
	if displayWidth(s) <= max {
		return s
	}
	w := 0
	for i, r := range s {
		if w+runeWidth(r) > max-3 {
			return s[:i] + "..."
		}
		w += runeWidth(r)
	}
	return s
}
