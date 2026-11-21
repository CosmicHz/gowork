package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gowork/internal/console"
)

func mkdirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("创建目录 %s: %v", name, err)
		}
	}
}

func TestWriteProjectNames(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "alpha", "beta", ".hidden")
	if err := os.WriteFile(filepath.Join(ws, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := time.Now()
	for name, offset := range map[string]time.Duration{"alpha": -2 * time.Hour, "beta": -1 * time.Hour} {
		ts := base.Add(offset)
		if err := os.Chtimes(filepath.Join(ws, name), ts, ts); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	writeProjectNames(ws, &out)
	if got, want := out.String(), "beta\nalpha\n"; got != want {
		t.Errorf("writeProjectNames = %q, 想要 %q", got, want)
	}
}

func TestWriteProjectNamesMissingWorkspace(t *testing.T) {
	var out bytes.Buffer
	writeProjectNames(filepath.Join(t.TempDir(), "missing"), &out)
	if out.Len() != 0 {
		t.Errorf("工作区不存在时不该有输出：%q", out.String())
	}
}

func TestCompleteSubcommandRejectsBadArgs(t *testing.T) {
	cases := []struct {
		desc string
		args []string
		want string
	}{
		{"缺少补全项", nil, "用法"},
		{"多余参数", []string{"projects", "extra"}, "用法"},
		{"未知补全项", []string{"bogus"}, "bogus"},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var out bytes.Buffer
			err := completeSubcommand(c.args, &out)
			if err == nil {
				t.Fatal("想要错误，得到 nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应包含 %q：%v", c.want, err)
			}
			if out.Len() != 0 {
				t.Errorf("出错时不该有输出：%q", out.String())
			}
		})
	}
}

func TestCompleteSubcommandProjects(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "my-app")
	t.Setenv("GOWORK_WORKSPACE", ws)
	// 尽量把缓存目录指到临时位置，避免读到本机真实配置。
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	var out bytes.Buffer
	if err := completeSubcommand([]string{"projects"}, &out); err != nil {
		t.Fatalf("补全项目失败：%v", err)
	}
	if got, want := out.String(), "my-app\n"; got != want {
		t.Errorf("输出 = %q, 想要 %q", got, want)
	}
}

// TestRunSubcommand 钉住子命令只在直接运行时生效：shell 标记出现时全部失效；
// 选项可以写在子命令前面，但只有第一个位置参数是 init / complete 才算。
func TestRunSubcommand(t *testing.T) {
	cases := []struct {
		desc    string
		args    []string
		handled bool
	}{
		{"init 直连", []string{"gowork", "init", "sh"}, true},
		{"complete 直连", []string{"gowork", "complete", "bogus"}, true},
		{"init 后面没有参数", []string{"gowork", "init"}, true},
		{"complete 后面没有参数", []string{"gowork", "complete"}, true},
		{"选项可以写在前面", []string{"gowork", "-n", "init", "sh"}, true},
		{"cwd-file 之后的 complete", []string{"gowork", "--cwd-file=/tmp/x", "complete"}, true},
		{"项目名之后的 init 不算", []string{"gowork", "my-app", "init"}, false},
		{"项目名之后的 complete 不算", []string{"gowork", "my-app", "complete"}, false},
		{"选项取值是 init 不算", []string{"gowork", "-p", "init"}, false},
		{"--project 的值是 init 不算", []string{"gowork", "--project", "init"}, false},
		{"-- 之后的 init 不算", []string{"gowork", "--", "init"}, false},
		{"shell 标记让 init 失效", []string{"gowork", "--" + shellMarker, "init", "sh"}, false},
		{"shell 标记在任何位置都生效", []string{"gowork", "my-app", "--" + shellMarker, "complete"}, false},
		{"普通项目", []string{"gowork", "my-app"}, false},
		{"没有参数", []string{"gowork"}, false},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var out bytes.Buffer
			con := console.NewWith(&out, io.Discard, false, false)
			if handled, _ := runSubcommand(c.args, con); handled != c.handled {
				t.Errorf("runSubcommand(%v) handled = %v, 想要 %v", c.args, handled, c.handled)
			}
		})
	}
}

// TestRunSubcommandRejectsSelectors 钉住子命令前后的项目选择器都会报错，而不是被丢掉。
func TestRunSubcommandRejectsSelectors(t *testing.T) {
	cases := [][]string{
		{"gowork", "--recent", "2", "init", "sh"},
		{"gowork", "--project", "foo", "complete", "projects"},
		{"gowork", "-1", "init", "sh"},
		{"gowork", "init", "sh", "--recent", "2"},
		{"gowork", "complete", "projects", "--project", "foo"},
		{"gowork", "init", "-1"},
	}
	for _, args := range cases {
		var out bytes.Buffer
		con := console.NewWith(&out, io.Discard, false, false)
		handled, err := runSubcommand(args, con)
		if !handled || err == nil {
			t.Errorf("runSubcommand(%v) = %v, %v；想要 handled 且报错", args, handled, err)
		}
	}
}

// TestRunSubcommandHelpBefore 钉住子命令前的 -h 仍然显示帮助。
func TestRunSubcommandHelpBefore(t *testing.T) {
	var out bytes.Buffer
	con := console.NewWith(&out, io.Discard, false, false)
	handled, err := runSubcommand([]string{"gowork", "-h", "init", "sh"}, con)
	if !handled || err != nil {
		t.Fatalf("runSubcommand = %v, %v；想要 handled 且无错误", handled, err)
	}
	if !strings.Contains(out.String(), "用法: gowork") {
		t.Errorf("应输出帮助，得到 %q", out.String())
	}
}
