package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func mkdirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			t.Fatalf("创建目录 %s: %v", n, err)
		}
	}
}

func projectNames(ps []Project) []string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
}

func TestListSkipsHiddenAndFiles(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "alpha", "beta", ".hidden")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := projectNames(List(root))
	slices.Sort(got)
	if want := []string{"alpha", "beta"}; !slices.Equal(got, want) {
		t.Errorf("List = %v, 想要 %v", got, want)
	}
}

func TestListSortsByModTimeDesc(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "old", "mid", "new")

	base := time.Now()
	for name, offset := range map[string]time.Duration{"old": -3 * time.Hour, "mid": -2 * time.Hour, "new": -1 * time.Hour} {
		ts := base.Add(offset)
		if err := os.Chtimes(filepath.Join(root, name), ts, ts); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := projectNames(List(root)), []string{"new", "mid", "old"}; !slices.Equal(got, want) {
		t.Errorf("List 顺序 = %v, 想要 %v", got, want)
	}
}

func TestListMissingRoot(t *testing.T) {
	if got := List(filepath.Join(t.TempDir(), "不存在")); got != nil {
		t.Errorf("工作区不存在时应返回 nil，得到 %v", got)
	}
}

func TestListFollowsDirLink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}

	linkDir(t, real, filepath.Join(root, "link"))
	if got, want := projectNames(List(root)), []string{"link"}; !slices.Equal(got, want) {
		t.Errorf("指向目录的链接应算项目，得到 %v", got)
	}
}

func TestResolve(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "my-app", "my-service", "other")
	projects := List(ws)

	cases := []struct {
		desc    string
		want    string
		target  string
		warns   bool
		wantErr bool
	}{
		{desc: "空参数回到工作区根", want: "", target: ws},
		{desc: "完整名", want: "my-app", target: filepath.Join(ws, "my-app")},
		{desc: "唯一前缀", want: "oth", target: filepath.Join(ws, "other")},
		{desc: "前缀大小写不敏感", want: "MY-A", target: filepath.Join(ws, "my-app")},
		{desc: "多命中取最短并告警", want: "my", target: filepath.Join(ws, "my-app"), warns: true},
		{desc: "零命中报错", want: "nope", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			target, warns, err := Resolve(ws, c.want, projects)
			if c.wantErr {
				if err == nil {
					t.Fatal("想要错误，得到 nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			if target != c.target {
				t.Errorf("target = %q, 想要 %q", target, c.target)
			}
			if gotWarns := len(warns) > 0; gotWarns != c.warns {
				t.Errorf("warnings = %v, 想要有告警=%v", warns, c.warns)
			}
		})
	}
}

func TestResolveIndex(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "a", "b", "c")
	projects := List(ws)

	target, _, err := Resolve(ws, "-1", projects)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if want := filepath.Join(ws, projects[0].Name); target != want {
		t.Errorf("-1 = %q, 想要 %q", target, want)
	}

	for _, bad := range []string{"-0", "-4", "-99"} {
		if _, _, err := Resolve(ws, bad, projects); err == nil {
			t.Errorf("%s 应报越界，得到 nil", bad)
		}
	}
}

func TestGitInfo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("未安装 git")
	}

	dir := t.TempDir()
	if _, _, ok := GitInfo(dir); ok {
		t.Error("非 git 目录不该返回 ok")
	}

	runGit(t, dir, "init")
	runGit(t, dir, "add", "-A")

	if _, changes, ok := GitInfo(dir); !ok || changes != 0 {
		t.Errorf("空仓库 changes = %d, ok = %v, 想要 0/true", changes, ok)
	}

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	branch, changes, ok := GitInfo(dir)
	if !ok || branch == "" {
		t.Fatalf("ok = %v, branch = %q", ok, branch)
	}
	if changes != 1 {
		t.Errorf("changes = %d, 想要 1", changes)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=tests", "GIT_AUTHOR_EMAIL=tests@example.com",
		"GIT_COMMITTER_NAME=tests", "GIT_COMMITTER_EMAIL=tests@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v 失败：%v\n%s", args, err, out)
	}
}
