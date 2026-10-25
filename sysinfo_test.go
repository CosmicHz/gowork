package main

import (
	"os"
	"testing"
)

// TestColorEnabled 钉住颜色开关的三个条件：真实终端、未传 --no-color、NO_COLOR 为空。
// fail 与 renderer 都只经这个函数判定，所以这里是唯一需要覆盖的地方。
func TestColorEnabled(t *testing.T) {
	cases := []struct {
		desc    string
		isTTY   bool
		noColor bool
		env     string
		setEnv  bool
		want    bool
	}{
		{"终端且未禁用", true, false, "", false, true},
		{"非终端", false, false, "", false, false},
		{"命令行禁用", true, true, "", false, false},
		{"NO_COLOR 非空", true, false, "1", true, false},
		{"NO_COLOR 为空等同于未设置", true, false, "", true, true},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			old, had := os.LookupEnv("NO_COLOR")
			t.Cleanup(func() {
				if had {
					_ = os.Setenv("NO_COLOR", old)
				} else {
					_ = os.Unsetenv("NO_COLOR")
				}
			})

			if c.setEnv {
				t.Setenv("NO_COLOR", c.env)
			} else {
				_ = os.Unsetenv("NO_COLOR")
			}

			if got := colorEnabled(c.isTTY, c.noColor); got != c.want {
				t.Errorf("colorEnabled(%v, %v) = %v，想要 %v", c.isTTY, c.noColor, got, c.want)
			}
		})
	}
}
