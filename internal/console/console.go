// Package console 是程序与控制台通信的唯一出口：标准输出/错误的写出、文本样式加工、
// 清屏、窗口标题与子进程终端接管都集中在这里。所有文本在真正写出前都会经过同一道
// 加工（transform），并带上来源标识，便于按部分做样式、转义、过滤或记录等处理。
package console

import (
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"

	"gowork/internal/sysinfo"
)

// Source 标识一段输出由程序的哪个部分产生，便于 transform 按部分做差异化处理。
type Source string

const (
	SourceDefault   Source = "default"  // 无特殊语义，也不强制纯文本。不要滥用
	SourceBanner    Source = "banner"   // logo
	SourceOnProject Source = "project"  // logo 上追加的 “on 项目名”
	SourceGreeting  Source = "greeting" // 问候语
	SourceRecent    Source = "recent"   // “最近活跃”标题
	SourceQuote     Source = "quote"    // 底部格言
	SourceRule      Source = "rule"     // 分隔线
	SourceWarning   Source = "warning"  // 前缀歧义、名称与选项冲突等提示
	SourceClone     Source = "clone"    // 克隆提示的标题
	SourceHelp      Source = "help"     // -h / --help
	SourceRaw       Source = "raw"      // 强制原样或调用方自行上色，跳过 transform。不要滥用
	SourceError     Source = "error"    // 错误信息
)

// Transform 在写出前加工文本；source 告诉加工者这段文本来自程序的哪个部分。
type Transform func(source Source, text string) string

// Console 持有控制台的两个输出流与终端状态。请用 New / NewWith 构造。
type Console struct {
	stdout    io.Writer // 标准输出流，仪表盘、帮助与子命令输出都写这里
	stderr    io.Writer // 标准错误流，只用于错误信息
	stdoutTTY bool      // stdout 是否为真实终端，决定清屏、标题与着色
	stderrTTY bool      // stderr 是否为真实终端，单独决定错误信息是否着色
	noColor   bool      // 是否禁用颜色（--no-color / 配置项）
	width     int       // 终端宽度，探测失败按 80 兜底
	accent    string    // 本次运行的主色
	transform Transform // 写出前的统一加工，nil 表示原样输出
}

// New 绑定标准输出与标准错误，并开启 Windows 控制台的 ANSI 解析与 UTF-8 代码页。
func New() *Console {
	sysinfo.EnableVT()
	c := &Console{
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		stdoutTTY: term.IsTerminal(int(os.Stdout.Fd())),
		stderrTTY: term.IsTerminal(int(os.Stderr.Fd())),
		width:     widthOf(os.Stdout),
		accent:    Palette[rand.IntN(len(Palette))],
	}
	c.transform = c.defaultTransform
	return c
}

// NewWith 用指定流构造 Console，供测试注入。两个布尔参数决定该流是否按终端处理；
// 传入 *os.File 时仍可被 Refresh 重新探测。
func NewWith(stdout, stderr io.Writer, stdoutTTY, stderrTTY bool) *Console {
	c := &Console{
		stdout:    stdout,
		stderr:    stderr,
		stdoutTTY: stdoutTTY,
		stderrTTY: stderrTTY,
		width:     80,
		accent:    Palette[0],
	}
	if f := fileOf(stdout); f != nil {
		c.width = widthOf(f)
	}
	c.transform = c.defaultTransform
	return c
}

// Refresh 重新探测终端状态（是否为终端、宽度），在窗口大小变化后调用即可热更新。
// 注入的普通流无法探测，保持原值。
func (c *Console) Refresh() {
	if f := fileOf(c.stdout); f != nil {
		c.stdoutTTY = term.IsTerminal(int(f.Fd()))
		c.width = widthOf(f)
	}
	if f := fileOf(c.stderr); f != nil {
		c.stderrTTY = term.IsTerminal(int(f.Fd()))
	}
}

// SetWidth 直接覆盖终端宽度，供无法探测或需要固定布局的场景使用。
func (c *Console) SetWidth(width int) { c.width = width }

// SetAccent 替换本次运行的主色，可在运行中切换主题。
func (c *Console) SetAccent(accent string) { c.accent = accent }

// SetNoColor 设置是否禁用 ANSI 颜色；stdout 与 stderr 共用这一开关。
func (c *Console) SetNoColor(noColor bool) { c.noColor = noColor }

// SetTransform 替换写出前的统一加工；传 nil 表示关闭加工（默认加工也会停用）。
func (c *Console) SetTransform(fn Transform) { c.transform = fn }

// Writer 返回标准输出的写入器，写出前会经过统一加工，并带上来源标识。
func (c *Console) Writer(source Source) io.Writer { return writer{c, source} }

// Print 把文本写到标准输出，写出前经过统一加工。
func (c *Console) Print(source Source, s string) {
	_ = c.write(source, c.stdout, s)
}

// Println 把若干片段拼成一行写到标准输出，末尾补换行。
func (c *Console) Println(source Source, parts ...string) {
	_ = c.write(source, c.stdout, strings.Join(parts, "")+"\n")
}

// Paint 按标准输出的颜色能力给文本着色；禁用颜色时原样返回。
// 整段颜色默认由 transform 负责，这里只用于同一段里需要多种颜色的精细调控。
// 如果一段文本经过 Paint，输出它时请将来源设为 RawSource
func (c *Console) Paint(color, s string) string {
	if !c.colorEnabled(c.stdoutTTY) {
		return s
	}
	return color + s + Reset
}

// Accent 返回本次运行随机挑选的主色。
func (c *Console) Accent() string { return c.accent }

// IsTerminal 表示标准输出是否是真实终端。
func (c *Console) IsTerminal() bool { return c.stdoutTTY }

// Width 返回构造时探测到的终端宽度。
func (c *Console) Width() int { return c.width }

// ClearScreen 清屏并清掉回滚缓冲；输出被重定向时不做任何事。
// 终端控制序列直接写出，不经过 transform，避免被样式加工破坏。
func (c *Console) ClearScreen() {
	if !c.stdoutTTY {
		return
	}
	_, _ = io.WriteString(c.stdout, "\x1b[H\x1b[2J\x1b[3J")
}

// SetTitle 设置终端窗口标题；输出被重定向时不做任何事。控制序列同样绕过 transform。
func (c *Console) SetTitle(title string) {
	if !c.stdoutTTY {
		return
	}
	_, _ = io.WriteString(c.stdout, "\x1b]0;"+title+"\x07")
}

// Attach 让子进程（git clone 等）直接接管控制台，进度条、SSH 与凭据提示都依赖真实终端。
func (c *Console) Attach(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
}

// Fail 把错误写到标准错误，然后按给定退出码结束进程。
func (c *Console) Fail(err error, code int) {
	_ = c.write(SourceError, c.stderr, err.Error()+"\n")
	os.Exit(code)
}

// defaultTransform 是缺省的写出前加工。它目前只负责整段样式，且只处理整段样式相同的
// 来源；SourceDefault 没有专属样式，落到最后的原样返回；SourceRaw 在 write 里就被跳过，
// 不会到这里；help 当前不做处理。
// transform 本身不限于颜色，转义、过滤或记录等都可以在这里扩展。
func (c *Console) defaultTransform(source Source, text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	switch source {
	case SourceError:
		if c.colorEnabled(c.stderrTTY) {
			return Red + text + Reset
		}
	case SourceBanner, SourceGreeting, SourceRecent, SourceClone:
		return c.Paint(c.accent, text)
	case SourceOnProject:
		return c.Paint(White, text)
	case SourceRule:
		return c.Paint(DarkGray, text)
	case SourceQuote:
		return c.Paint(Green, text)
	case SourceWarning:
		return c.Paint(Yellow, text)
	}
	return text
}

// colorEnabled 是颜色开关的唯一判定：该流是真实终端、未传 --no-color、NO_COLOR 为空。
func (c *Console) colorEnabled(isTTY bool) bool {
	return sysinfo.ColorEnabled(isTTY, c.noColor)
}

// write 是所有标准输出/错误写出的终点。SourceRaw 表示文本已由调用方处理完毕，
// 直接写出、不经过 transform；其余来源统一执行 transform。
func (c *Console) write(source Source, w io.Writer, s string) error {
	if source != SourceRaw && c.transform != nil {
		s = c.transform(source, s)
	}
	_, err := io.WriteString(w, s)
	return err
}

// widthOf 探测文件对应终端的宽度，取不到时按 80 列兜底。
func widthOf(f *os.File) int {
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

// fileOf 取出写入器背后的 *os.File；普通写入器（如 bytes.Buffer）返回 nil。
func fileOf(w io.Writer) *os.File {
	f, _ := w.(*os.File)
	return f
}

// writer 把 Console 包装成 io.Writer，供 help / init / complete 等直接写流的场景。
type writer struct {
	c      *Console
	source Source
}

func (w writer) Write(p []byte) (int, error) {
	if err := w.c.write(w.source, w.c.stdout, string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
