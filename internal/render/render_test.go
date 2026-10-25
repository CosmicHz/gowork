package render

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

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
		{"ASCII 超长", strings.Repeat("a", 40), recentNameWidth},
		{"CJK 超长", strings.Repeat("中", 30), recentNameWidth},
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
	if got := truncateWidth(strings.Repeat("中", 30), recentNameWidth); displayWidth(got) != recentNameWidth {
		t.Errorf("截断宽度 = %d, 想要 %d：%q", displayWidth(got), recentNameWidth, got)
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
