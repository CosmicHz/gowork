package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestRunSubcommand 钉住子命令只在 shellArgIndex 位置生效，且只看位置、不看注入参数的名字。
// shell 函数总会在这个位置注入一个内部参数（当前是 --cwd-file），
// 所以 work 里的 init / complete 是项目名。
func TestRunSubcommand(t *testing.T) {
	cases := []struct {
		desc    string
		args    []string
		handled bool
	}{
		{"init 直连", []string{"gowork", "init", "sh"}, true},
		{"complete 直连", []string{"gowork", "complete", "bogus"}, true},
		{"cwd-file 之后的 complete 是项目名", []string{"gowork", "--cwd-file=/tmp/x", "complete"}, false},
		{"cwd-file 之后的 init 是项目名", []string{"gowork", "--cwd-file=/tmp/x", "init"}, false},
		{"换一个内部参数名也一样", []string{"gowork", "--internal", "init"}, false},
		{"短选项作内部参数也一样", []string{"gowork", "-x", "complete"}, false},
		{"第二位置参数不是子命令", []string{"gowork", "my-app", "complete"}, false},
		{"普通项目", []string{"gowork", "my-app"}, false},
		{"没有参数", []string{"gowork"}, false},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var out bytes.Buffer
			if handled, _ := runSubcommand(c.args, &out); handled != c.handled {
				t.Errorf("runSubcommand(%v) handled = %v, 想要 %v", c.args, handled, c.handled)
			}
		})
	}
}
