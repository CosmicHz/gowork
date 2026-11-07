package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func hasWarning(warnings []Warning, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w.Message, substr) {
			return true
		}
	}
	return false
}

func TestResolveWarnsOnIndexNameCollision(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "-1", "other")
	projects := List(ws)

	target, warnings, err := Resolve(ws, "-1", projects)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if want := filepath.Join(ws, projects[0].Name); target != want {
		t.Errorf("-1 = %q, 想要 %q", target, want)
	}
	if len(warnings) != 1 || !hasWarning(warnings, "-1") {
		t.Fatalf("应提示 -1 目录与索引同名：%+v", warnings)
	}
	if len(warnings[0].Details) == 0 || !strings.Contains(strings.Join(warnings[0].Details, ""), "./-1") {
		t.Errorf("应给出 ./-1 的消歧写法：%+v", warnings[0])
	}

	// 用 ./-1 明确指定时没有歧义，不该再提示。
	if _, warnings, err := Resolve(ws, "./-1", projects); err != nil || len(warnings) != 0 {
		t.Errorf("./-1 应静默进入，warnings=%+v err=%v", warnings, err)
	}
}

func TestResolveWarnsOnIndexPrefixCollision(t *testing.T) {
	cases := []struct {
		name        string
		token       string
		dirs        []string
		wantDetails []string
	}{
		{"-1 前缀", "-1", []string{"-1-app", "-1-service", "other"}, []string{"-1-app", "-1-service"}},
		{"-2 前缀", "-2", []string{"-2-tool", "other"}, []string{"-2-tool"}},
		{"同名加前缀", "-1", []string{"-1", "-1-app"}, []string{"-1", "-1-app"}},
		{"无同名前缀", "-1", []string{"alpha", "beta"}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			mkdirs(t, ws, c.dirs...)
			projects := List(ws)

			_, warnings, err := Resolve(ws, c.token, projects)
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			if len(c.wantDetails) == 0 {
				if len(warnings) != 0 {
					t.Errorf("没有同名前缀时不该提示：%+v", warnings)
				}
				return
			}
			if len(warnings) != 1 || !hasWarning(warnings, "索引") {
				t.Fatalf("应提示索引与目录名冲突：%+v", warnings)
			}
			details := strings.Join(warnings[0].Details, "\n")
			for _, want := range c.wantDetails {
				if !strings.Contains(details, want) {
					t.Errorf("提示明细应包含 %q：\n%s", want, details)
				}
			}
			if !strings.Contains(details, "work ./") {
				t.Errorf("应给出 ./ 开头的消歧写法：\n%s", details)
			}
			if !strings.Contains(details, "完整名") || strings.Contains(details, "更长前缀") {
				t.Errorf("./ 是字面精确匹配，提示不应建议更长前缀：\n%s", details)
			}
		})
	}
}

func TestResolveWithoutIndexTreatsDashNumberAsName(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "normal")
	projects := List(ws)

	// 没有名为 -1 的目录时，ResolveWithoutIndex 不能退化成索引。
	if _, _, err := ResolveWithoutIndex(ws, "-1", projects); err == nil {
		t.Fatal("-1 没有同名目录时应报目录不存在，而不是按索引跳转")
	}
	// 同样的输入，普通 Resolve 仍按索引解析。
	if _, _, err := Resolve(ws, "-1", projects); err != nil {
		t.Fatalf("-1 在普通 Resolve 中应作为索引：%v", err)
	}

	mkdirs(t, ws, "-1")
	projects = List(ws)
	target, warnings, err := ResolveWithoutIndex(ws, "-1", projects)
	if err != nil {
		t.Fatalf("应进入名为 -1 的目录：%v", err)
	}
	if filepath.Base(target) != "-1" {
		t.Errorf("应进入 -1，得到 %q", target)
	}
	if len(warnings) != 0 {
		t.Errorf("按名字进入时不应再提示索引冲突：%v", warnings)
	}
}

func TestResolveDotSlashIsExactName(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "-114514")
	projects := List(ws)

	if _, _, err := Resolve(ws, "./-114", projects); err == nil {
		t.Fatal("./-114 不应前缀匹配 -114514")
	}
	if target, _, err := Resolve(ws, "./-114514", projects); err != nil {
		t.Fatalf("./-114514 应进入完整名目录：%v", err)
	} else if want := filepath.Join(ws, "-114514"); target != want {
		t.Errorf("target = %q, 想要 %q", target, want)
	}
}

func TestResolveDoesNotWarnOnInitPrefixOnly(t *testing.T) {
	ws := t.TempDir()
	mkdirs(t, ws, "init-app", "init-service")
	projects := List(ws)

	_, warnings, err := Resolve(ws, "init", projects)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if len(warnings) != 1 || !hasWarning(warnings, "前缀") {
		t.Errorf("只有前缀歧义时不该报 init 同名：%+v", warnings)
	}
}

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
