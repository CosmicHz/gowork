package console

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// clearNoColor 让 NO_COLOR 在本用例期间保持未设置，结束后恢复。
func clearNoColor(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("NO_COLOR")
	if had {
		_ = os.Unsetenv("NO_COLOR")
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("NO_COLOR", old)
		}
	})
}

// TestPaintHonorsColorSwitch 钉住闸门的颜色策略：只有真实终端、未禁用、NO_COLOR 为空才着色。
func TestPaintHonorsColorSwitch(t *testing.T) {
	clearNoColor(t)
	con := NewWith(io.Discard, io.Discard, true, true)

	if got := con.Paint(Red, "错"); got != Red+"错"+Reset {
		t.Errorf("终端下应着色，得到 %q", got)
	}

	con.SetNoColor(true)
	if got := con.Paint(Red, "错"); got != "错" {
		t.Errorf("--no-color 应原样返回，得到 %q", got)
	}

	con.SetNoColor(false)
	t.Setenv("NO_COLOR", "1")
	if got := con.Paint(Red, "错"); got != "错" {
		t.Errorf("NO_COLOR 非空应原样返回，得到 %q", got)
	}
}

// TestPaintNeedsTTY 确认重定向输出时不会混入转义序列。
func TestPaintNeedsTTY(t *testing.T) {
	clearNoColor(t)
	con := NewWith(io.Discard, io.Discard, false, false)
	if got := con.Paint(Red, "错"); got != "错" {
		t.Errorf("非终端不该着色，得到 %q", got)
	}
}

// TestTransformAppliesToAllOutput 钉住“写出前统一加工”：Print / Println / Writer 都要经过
// transform，并且各自带上真实的来源标识。
func TestTransformAppliesToAllOutput(t *testing.T) {
	out := &bytes.Buffer{}
	con := NewWith(out, io.Discard, false, false)
	con.SetTransform(func(source Source, s string) string {
		return string(source) + ":" + s
	})

	con.Print(SourceBanner, "a\n")
	con.Println(SourceHelp, "b", "c")
	if _, err := io.WriteString(con.Writer(SourceWarning), "d\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(con.Writer(SourceDefault), "e\n"); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "banner:a\nhelp:bc\nwarning:d\ndefault:e\n"; got != want {
		t.Errorf("transform 输出 = %q，想要 %q", got, want)
	}
}

// TestRawSourceBypassesTransform 钉住 SourceRaw 的硬性契约：文本直接写出，
// 连自定义 transform 都不会被调用。
func TestRawSourceBypassesTransform(t *testing.T) {
	out := &bytes.Buffer{}
	con := NewWith(out, io.Discard, false, false)
	con.SetTransform(func(Source, string) string { return "被加工" })

	con.Println(SourceRaw, "原样")
	if _, err := io.WriteString(con.Writer(SourceRaw), "也原样\n"); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "原样\n也原样\n"; got != want {
		t.Errorf("SourceRaw 输出 = %q，想要 %q", got, want)
	}
}

// TestTerminalSequencesOnlyOnTTY 清屏与窗口标题只在真实终端下写出，且不经过 transform。
func TestTerminalSequencesOnlyOnTTY(t *testing.T) {
	out := &bytes.Buffer{}
	con := NewWith(out, io.Discard, false, false)
	con.ClearScreen()
	con.SetTitle("WORK | /ws")
	if out.Len() != 0 {
		t.Errorf("非终端不该写终端序列：%q", out.String())
	}

	out.Reset()
	con = NewWith(out, io.Discard, true, true)
	con.SetTransform(func(Source, string) string { return "被加工" })
	con.ClearScreen()
	con.SetTitle("WORK | /ws")
	if got, want := out.String(), "\x1b[H\x1b[2J\x1b[3J\x1b]0;WORK | /ws\x07"; got != want {
		t.Errorf("终端序列 = %q，想要 %q", got, want)
	}
}

// TestLineTracksOutput 钉住 Console 维护的当前行号：所有写出路径都会推进，
// 清屏后归零；Line 从 1 开始。
func TestLineTracksOutput(t *testing.T) {
	out := &bytes.Buffer{}
	con := NewWith(out, io.Discard, false, false)

	if got := con.Line(); got != 1 {
		t.Errorf("初始行号 = %d，想要 1", got)
	}
	con.Print(SourceDefault, "a")
	if got := con.Line(); got != 1 {
		t.Errorf("没有换行时行号不该变，得到 %d", got)
	}
	con.Println(SourceDefault, "b")
	if got := con.Line(); got != 2 {
		t.Errorf("Println 后行号 = %d，想要 2", got)
	}
	con.Print(SourceDefault, "c\nd")
	if got := con.Line(); got != 3 {
		t.Errorf("一次写多个换行后行号 = %d，想要 3", got)
	}
	if _, err := io.WriteString(con.Writer(SourceDefault), "e\n"); err != nil {
		t.Fatal(err)
	}
	if got := con.Line(); got != 4 {
		t.Errorf("Writer 写出后行号 = %d，想要 4", got)
	}

	con.ClearScreen()
	if got := con.Line(); got != 1 {
		t.Errorf("清屏后行号应回到 1，得到 %d", got)
	}
}

// TestDefaultTransformColorsUniformSources 钉住默认加工的分工：整段同色的来源由
// transform 着色；SourceRaw 在 write 里就被跳过，defaultTransform 对它保持原样。
func TestDefaultTransformColorsUniformSources(t *testing.T) {
	clearNoColor(t)
	con := NewWith(io.Discard, io.Discard, true, true)

	cases := []struct {
		source Source
		text   string
		want   string
	}{
		{SourceError, "错", Red + "错" + Reset},
		{SourceGreeting, "早", con.Accent() + "早" + Reset},
		{SourceRule, "──", DarkGray + "──" + Reset},
		{SourceQuote, "格言", Green + "格言" + Reset},
		{SourceWarning, "⚠ 注意", Yellow + "⚠ 注意" + Reset},
		{SourceBanner, "logo", con.Accent() + "logo" + Reset},
		{SourceOnProject, "on alpha", White + "on alpha" + Reset},
		{SourceRecent, "最近活跃", con.Accent() + "最近活跃" + Reset},
		{SourceClone, "正在克隆仓库", con.Accent() + "正在克隆仓库" + Reset},
		{SourceHelp, "用法", "用法"},
		{SourceDefault, "普通文本", "普通文本"},
		{SourceQuote, "\n", "\n"},
	}
	for _, c := range cases {
		if got := con.defaultTransform(c.source, c.text); got != c.want {
			t.Errorf("defaultTransform(%q, %q) = %q，想要 %q", c.source, c.text, got, c.want)
		}
	}

	con.SetNoColor(true)
	if got := con.defaultTransform(SourceGreeting, "早"); got != "早" {
		t.Errorf("--no-color 应关闭着色，得到 %q", got)
	}

	// stderr 被重定向时错误信息不着色，与 stdout 的终端状态无关。
	redirected := NewWith(io.Discard, io.Discard, true, false)
	if got := redirected.defaultTransform(SourceError, "错"); got != "错" {
		t.Errorf("stderr 非终端时不该着色，得到 %q", got)
	}
}

// TestRainbowAccent 钉住小概率彩虹主色：抽中时 AccentPaint 逐字符上色；
// Paint 只认调用方给的颜色，即使传的正好是主色也不会变成彩虹；
// 禁用颜色或显式设置主色后回到普通行为。
func TestRainbowAccent(t *testing.T) {
	clearNoColor(t)
	con := NewWith(io.Discard, io.Discard, true, true)
	con.rainbow = true
	con.rainbowOS = 0

	if got := con.AccentPaint("ab"); !strings.Contains(got, "\x1b[38;") {
		t.Errorf("彩虹主色应逐字符上色，得到 %q", got)
	}
	// 同一行的所有彩色片段共用 seed 与行号相位，颜色必须一致。
	first := con.AccentPaint("ab")
	if again := con.AccentPaint("ab"); again != first {
		t.Errorf("同一行应共用相位，得到 %q 与 %q", first, again)
	}
	if value := con.ValuePaint("ab"); value != first {
		t.Errorf("行内容与主色在同一行应共用相位，得到 %q 与 %q", first, value)
	}

	// 纵向相位按当前行号错开，写完一行后颜色才会变化。
	con.Println(SourceDefault, "换行")
	if second := con.AccentPaint("ab"); first == second {
		t.Errorf("换行后的下一段应错开相位，输出却相同：%q", first)
	}
	if got, want := con.Paint(con.Accent(), "ab"), con.Accent()+"ab"+Reset; got != want {
		t.Errorf("Paint 不该因为颜色等于主色就上彩虹，得到 %q，想要 %q", got, want)
	}
	if got := con.Paint(Red, "错"); got != Red+"错"+Reset {
		t.Errorf("固定颜色不该被彩虹影响，得到 %q", got)
	}
	if got := con.defaultTransform(SourceQuote, "格言"); !strings.Contains(got, "\x1b[38;") {
		t.Errorf("彩虹模式下格言应跟随主色，得到 %q", got)
	}

	con.SetNoColor(true)
	if got := con.AccentPaint("ab"); got != "ab" {
		t.Errorf("禁用颜色时应原样返回，得到 %q", got)
	}

	con.SetNoColor(false)
	con.SetAccent(Cyan)
	if got := con.Accent(); got != Cyan {
		t.Errorf("显式设置主色后应退出彩虹模式，得到 %q", got)
	}
	if got := con.AccentPaint("ab"); got != Cyan+"ab"+Reset {
		t.Errorf("退出彩虹模式后应恢复整段单色，得到 %q", got)
	}
}

// TestLabelPaint 钉住行标签始终暗灰，抽中彩虹也不跟随。
func TestLabelPaint(t *testing.T) {
	clearNoColor(t)
	con := NewWith(io.Discard, io.Discard, true, true)

	if got, want := con.LabelPaint("时间"), DarkGray+"时间"+Reset; got != want {
		t.Errorf("普通运行标签应暗灰，得到 %q，想要 %q", got, want)
	}

	con.rainbow = true
	if got, want := con.LabelPaint("时间"), DarkGray+"时间"+Reset; got != want {
		t.Errorf("彩虹模式下标签也应保持暗灰，得到 %q，想要 %q", got, want)
	}

	con.SetNoColor(true)
	if got := con.LabelPaint("时间"); got != "时间" {
		t.Errorf("禁用颜色时应原样返回，得到 %q", got)
	}
}

// TestValuePaint 钉住行内容：普通运行白色，抽中彩虹时跟随主色。
func TestValuePaint(t *testing.T) {
	clearNoColor(t)
	con := NewWith(io.Discard, io.Discard, true, true)

	if got, want := con.ValuePaint("99"), White+"99"+Reset; got != want {
		t.Errorf("普通运行内容应白色，得到 %q，想要 %q", got, want)
	}

	con.rainbow = true
	if got := con.ValuePaint("99"); !strings.Contains(got, "\x1b[38;") {
		t.Errorf("彩虹模式下内容应跟随主色，得到 %q", got)
	}

	con.SetNoColor(true)
	if got := con.ValuePaint("99"); got != "99" {
		t.Errorf("禁用颜色时应原样返回，得到 %q", got)
	}
}

// TestNewWithDefaults 钉住注入流时的兜底宽度与固定主色，避免测试依赖真实终端。
func TestNewWithDefaults(t *testing.T) {
	con := NewWith(io.Discard, io.Discard, false, false)
	if con.Width() != 80 {
		t.Errorf("注入流时宽度应按 80 兜底，得到 %d", con.Width())
	}
	if con.Accent() != Palette[0] {
		t.Errorf("注入流时主色应固定为调色板首项，得到 %q", con.Accent())
	}
	if con.IsTerminal() {
		t.Error("注入流时不该报告为终端")
	}
}

// TestHotUpdateWidthAndAccent 钉住终端配置可在同一个 Console 上热更新；
// 注入普通流时 Refresh 没有可探测的文件，应保持已设置的值。
func TestHotUpdateWidthAndAccent(t *testing.T) {
	con := NewWith(io.Discard, io.Discard, false, false)

	con.SetWidth(120)
	con.SetAccent(Cyan)
	if con.Width() != 120 || con.Accent() != Cyan {
		t.Errorf("热更新未生效：width=%d accent=%q", con.Width(), con.Accent())
	}

	con.Refresh()
	if con.Width() != 120 || con.Accent() != Cyan {
		t.Errorf("Refresh 不该覆盖注入流的值：width=%d accent=%q", con.Width(), con.Accent())
	}
}
