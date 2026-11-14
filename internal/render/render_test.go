package render

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gowork/internal/config"
	"gowork/internal/project"
)

func testRenderer(out *bytes.Buffer) *Renderer {
	return &Renderer{
		out:             out,
		indent:          "   ",
		width:           80,
		workspace:       "/ws",
		recentLimit:     4,
		recentNameWidth: 35,
		diskWarnGB:      10,
		showDisk:        true,
		showGit:         true,
		showHoliday:     true,
		showRecent:      true,
		clearScreenOn:   true,
		quotes:          []string{"固定格言"},
	}
}

func TestRenderHonorsLayoutConfig(t *testing.T) {
	out := &bytes.Buffer{}
	r := testRenderer(out)
	r.showDisk = false
	r.showGit = false
	r.showHoliday = false
	r.recentLimit = 2
	r.recentNameWidth = 10

	projects := []project.Project{
		{Name: "very-long-project-name", Mod: time.Date(2026, 10, 31, 12, 0, 0, 0, time.Local)},
		{Name: "beta", Mod: time.Date(2026, 10, 31, 11, 0, 0, 0, time.Local)},
		{Name: "gamma", Mod: time.Date(2026, 10, 31, 10, 0, 0, 0, time.Local)},
	}
	r.Render("/ws", projects, nil)
	got := out.String()

	for _, hidden := range []string{"磁盘", "Git", "节日"} {
		if strings.Contains(got, hidden) {
			t.Errorf("关闭后不该出现 %q：\n%s", hidden, got)
		}
	}
	if !strings.Contains(got, "最近活跃") {
		t.Errorf("缺少最近活跃：\n%s", got)
	}
	if strings.Contains(got, "gamma") {
		t.Errorf("recent_limit=2 不该列出 gamma：\n%s", got)
	}
	if strings.Contains(got, "very-long-project-name") {
		t.Errorf("recent_name_width=10 应截断长名字：\n%s", got)
	}
	if !strings.Contains(got, "very-lo...") {
		t.Errorf("截断结果应保留 7 列加 ...：\n%s", got)
	}
	if !strings.Contains(got, "固定格言") {
		t.Errorf("应使用自定义格言：\n%s", got)
	}
}

func TestNewAppliesConfigOptions(t *testing.T) {
	o := Options{
		Indent:          4,
		Workspace:       "/ws",
		NoColor:         true,
		RecentLimit:     2,
		RecentNameWidth: 10,
		DiskWarnGB:      5,
		ShowDisk:        false,
		ShowGit:         false,
		ShowHoliday:     false,
		ShowRecent:      false,
		ClearScreen:     false,
		Quotes:          []string{"甲"},
	}
	r := New(o)
	if r.indent != "    " || r.workspace != "/ws" {
		t.Errorf("基础字段不对：indent=%q workspace=%q", r.indent, r.workspace)
	}
	if r.recentLimit != 2 || r.recentNameWidth != 10 || r.diskWarnGB != 5 {
		t.Errorf("布局数值没有生效：%+v", r)
	}
	if r.showDisk || r.showGit || r.showHoliday || r.showRecent || r.clearScreenOn {
		t.Errorf("开关没有生效：%+v", r)
	}
	if len(r.quotes) != 1 || r.quotes[0] != "甲" {
		t.Errorf("格言没有生效：%v", r.quotes)
	}
}

func TestDefaultOptions(t *testing.T) {
	o := DefaultOptions()
	if o.Indent != config.DefaultIndent ||
		o.RecentLimit != config.DefaultRecentLimit ||
		o.RecentNameWidth != config.DefaultRecentNameWidth ||
		o.DiskWarnGB != config.DefaultDiskWarnGB {
		t.Errorf("数值默认值应与 config 共用：%+v", o)
	}
	if !o.ShowDisk || !o.ShowGit || !o.ShowHoliday || !o.ShowRecent || !o.ClearScreen {
		t.Errorf("默认开关应全部为 true：%+v", o)
	}
}

func TestNewDefaultsOptionalFeatures(t *testing.T) {
	r := New(DefaultOptions())
	if !r.showDisk || !r.showGit || !r.showHoliday || !r.showRecent {
		t.Errorf("默认配置应显示所有行：%+v", r)
	}
	if !r.clearScreenOn {
		t.Error("默认配置应清屏")
	}
}

func TestRenderCanHideRecent(t *testing.T) {
	out := &bytes.Buffer{}
	r := testRenderer(out)
	r.showRecent = false
	r.showDisk = false
	r.showGit = false
	r.showHoliday = false

	r.Render("/ws", []project.Project{{Name: "alpha"}}, nil)
	if got := out.String(); strings.Contains(got, "最近活跃") {
		t.Errorf("show_recent=false 不该出现最近活跃：\n%s", got)
	}
}

func TestRenderWarningsBelowDashboard(t *testing.T) {
	out := &bytes.Buffer{}
	r := testRenderer(out)
	r.showDisk = false
	r.showGit = false
	r.showHoliday = false
	r.showRecent = false

	warnings := []project.Warning{
		{
			Message: "前缀 'my' 匹配到多个项目，跳转到最短匹配：",
			Details: []string{"my-app", "my-service"},
		},
		{
			Message: `"-n" 同时是工作区里的目录名与命令行选项；当前按选项解析`,
			Details: []string{"要进入该目录请写 work ./-n"},
		},
	}
	r.Render("/ws", nil, warnings)
	got := out.String()

	quoteAt := strings.Index(got, "固定格言")
	if quoteAt < 0 {
		t.Fatalf("缺少格言：\n%s", got)
	}
	for _, w := range warnings {
		at := strings.Index(got, w.Message)
		if at < 0 {
			t.Fatalf("缺少提示 %q：\n%s", w.Message, got)
		}
		if at <= quoteAt {
			t.Errorf("提示 %q 应在仪表盘下方（格言之后）：\n%s", w.Message, got)
		}
	}
	if n := strings.Count(got, "⚠"); n != len(warnings) {
		t.Errorf("每条提示应各带一个 ⚠，得到 %d 个：\n%s", n, got)
	}
	if !strings.Contains(got, "my-service\n\n") {
		t.Errorf("两条提示之间应空一行：\n%s", got)
	}
}

func TestDisplayWidth(t *testing.T) {
	cases := map[string]int{
		"":     0,
		"abc":  3,
		"时间":   4,
		"a时b":  4,
		"中文项目": 8,
	}
	for in, want := range cases {
		if got := displayWidth(in); got != want {
			t.Errorf("displayWidth(%q) = %d, 想要 %d", in, got, want)
		}
	}
}

func TestRuneWidthZeroWidth(t *testing.T) {
	// 组合用记号（Mn）、零宽连接符（Cf）、变体选择符都应记 0 列。
	for _, r := range []rune{'\u0301', '\u200d', '\ufe0f'} {
		if got := runeWidth(r); got != 0 {
			t.Errorf("runeWidth(%U) = %d, 想要 0", r, got)
		}
	}
	if got := runeWidth('中'); got != 2 {
		t.Errorf("runeWidth(中) = %d, 想要 2", got)
	}
	if got := runeWidth('a'); got != 1 {
		t.Errorf("runeWidth(a) = %d, 想要 1", got)
	}
}

func TestTruncateWidth(t *testing.T) {
	if got := truncateWidth("abc", 5); got != "abc" {
		t.Errorf("短于上限不该截：%q", got)
	}
	if got := truncateWidth("abcde", 5); got != "abcde" {
		t.Errorf("正好等于上限不该截：%q", got)
	}

	cases := []struct {
		desc string
		in   string
		max  int
	}{
		{"ASCII 超长", strings.Repeat("a", 40), config.DefaultRecentNameWidth},
		{"CJK 超长", strings.Repeat("中", 30), config.DefaultRecentNameWidth},
		{"恰好 3 列上限", "abcdef", 3},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := truncateWidth(c.in, c.max)
			if w := displayWidth(got); w > c.max {
				t.Errorf("截断后 %d 列，超过上限 %d：%q", w, c.max, got)
			}
			if !strings.HasSuffix(got, "...") {
				t.Errorf("截断后应以 ... 收尾：%q", got)
			}
			if !utf8.ValidString(got) {
				t.Errorf("截断破坏了 UTF-8：%q", got)
			}
		})
	}

	// 35 列的预算应被用满，而不是截得过狠。
	if got := truncateWidth(strings.Repeat("中", 30), config.DefaultRecentNameWidth); displayWidth(got) != config.DefaultRecentNameWidth {
		t.Errorf("截断宽度 = %d, 想要 %d：%q", displayWidth(got), config.DefaultRecentNameWidth, got)
	}
}

func TestGreeting(t *testing.T) {
	cases := []struct {
		hour int
		want string
	}{
		{0, "夜深了"}, {5, "夜深了"},
		{6, "早上好"}, {11, "早上好"},
		{12, "中午好"}, {13, "中午好"},
		{14, "下午好"}, {17, "下午好"},
		{18, "晚上好"}, {23, "晚上好"},
	}
	for _, c := range cases {
		ts := time.Date(2026, 10, 18, c.hour, 0, 0, 0, time.Local)
		if got := greeting(ts); !strings.Contains(got, c.want) {
			t.Errorf("greeting(%d 点) = %q, 想要包含 %q", c.hour, got, c.want)
		}
	}
}

func TestWeekdayZH(t *testing.T) {
	cases := map[string]string{
		"2026-10-18": "星期日",
		"2026-10-19": "星期一",
		"2026-10-24": "星期六",
	}
	for date, want := range cases {
		ts, err := time.ParseInLocation("2006-01-02", date, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		if got := weekdayZH(ts); got != want {
			t.Errorf("weekdayZH(%s) = %q, 想要 %q", date, got, want)
		}
	}
}
