package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAllFields(t *testing.T) {
	path := writeConfig(t, `{
		"workspace": "/work",
		"indent": 5,
		"no_color": true,
		"recent_limit": 2,
		"recent_name_width": 20,
		"disk_warn_gb": 5.5,
		"show_disk": false,
		"show_git": false,
		"show_holiday": false,
		"show_recent": false,
		"clear_screen": false,
		"quotes": ["甲", "乙"]
	}`)

	c, err := load(path)
	if err != nil {
		t.Fatalf("解析配置失败：%v", err)
	}
	if c.Workspace != "/work" {
		t.Errorf("workspace = %q", c.Workspace)
	}
	if c.Indent != 5 {
		t.Errorf("indent = %d", c.Indent)
	}
	if !c.NoColor {
		t.Error("no_color 应为 true")
	}
	if c.RecentLimit != 2 {
		t.Errorf("recent_limit = %d", c.RecentLimit)
	}
	if c.RecentNameWidth != 20 {
		t.Errorf("recent_name_width = %d", c.RecentNameWidth)
	}
	if c.DiskWarnGB != 5.5 {
		t.Errorf("disk_warn_gb = %v", c.DiskWarnGB)
	}
	for name, got := range map[string]bool{
		"show_disk": c.ShowDisk, "show_git": c.ShowGit,
		"show_holiday": c.ShowHoliday, "show_recent": c.ShowRecent,
		"clear_screen": c.ClearScreen,
	} {
		if got {
			t.Errorf("%s 应为 false", name)
		}
	}
	if got := strings.Join(c.Quotes, "/"); got != "甲/乙" {
		t.Errorf("quotes = %q", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	c, err := load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("文件不存在不该报错：%v", err)
	}
	if c.Indent != DefaultIndent {
		t.Errorf("缺少文件时应得到默认配置，indent = %d", c.Indent)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	if _, err := load(writeConfig(t, `{"indent":`)); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
}

func TestLoadKeepsDefaultsForOmittedFields(t *testing.T) {
	c, err := load(writeConfig(t, `{"workspace": "/work"}`))
	if err != nil {
		t.Fatalf("解析配置失败：%v", err)
	}
	if c.Indent != DefaultIndent || c.RecentLimit != DefaultRecentLimit ||
		c.DiskWarnGB != DefaultDiskWarnGB {
		t.Errorf("省略字段应保留默认值：%+v", c)
	}
	if !c.ShowDisk || !c.ShowGit || !c.ShowHoliday || !c.ShowRecent || !c.ClearScreen {
		t.Errorf("省略开关应保留 true：%+v", c)
	}
}

func TestLoadExplicitZeroOverridesDefault(t *testing.T) {
	c, err := load(writeConfig(t, `{"disk_warn_gb": 0}`))
	if err != nil {
		t.Fatalf("解析配置失败：%v", err)
	}
	if c.DiskWarnGB != 0 {
		t.Errorf("显式 0 应覆盖默认值，得到 %v", c.DiskWarnGB)
	}
}

func TestLoadValidation(t *testing.T) {
	cases := []struct {
		desc    string
		content string
		want    string
	}{
		{"缩进过小", `{"indent": 2}`, "indent=2"},
		{"最近数量为 0", `{"recent_limit": 0}`, "recent_limit=0"},
		{"名称宽度过小", `{"recent_name_width": 4}`, "recent_name_width=4"},
		{"磁盘阈值为负", `{"disk_warn_gb": -1}`, "disk_warn_gb=-1"},
		{"空格言", `{"quotes": ["ok", "  "]}`, "quotes[1]"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			_, err := load(writeConfig(t, c.content))
			if err == nil {
				t.Fatal("非法配置应报错")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应包含实际值 %q：%v", c.want, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Indent != DefaultIndent || c.RecentLimit != DefaultRecentLimit ||
		c.RecentNameWidth != DefaultRecentNameWidth || c.DiskWarnGB != DefaultDiskWarnGB {
		t.Errorf("默认数值不对：%+v", c)
	}
	if !c.ShowDisk || !c.ShowGit || !c.ShowHoliday || !c.ShowRecent || !c.ClearScreen {
		t.Errorf("默认开关应全部为 true：%+v", c)
	}
	if c.Quotes != nil {
		t.Errorf("默认不该带自定义格言：%v", c.Quotes)
	}
}

func TestResolveWorkspace(t *testing.T) {
	flagWS := filepath.Join(t.TempDir(), "flag")
	envWS := filepath.Join(t.TempDir(), "env")
	cfgWS := filepath.Join(t.TempDir(), "config")

	t.Setenv(workspaceEnv, envWS)
	c := DefaultConfig()
	c.Workspace = cfgWS

	got, err := c.ResolveWorkspace(flagWS)
	if err != nil || got != filepath.Clean(flagWS) {
		t.Errorf("命令行最优先：got %q, err %v", got, err)
	}
	got, err = c.ResolveWorkspace("")
	if err != nil || got != filepath.Clean(envWS) {
		t.Errorf("环境变量其次：got %q, err %v", got, err)
	}

	t.Setenv(workspaceEnv, "")
	got, err = c.ResolveWorkspace("")
	if err != nil || got != filepath.Clean(cfgWS) {
		t.Errorf("配置文件第三：got %q, err %v", got, err)
	}
}

func TestResolveWorkspaceEnvPriority(t *testing.T) {
	envWS := filepath.Join(t.TempDir(), "env-ws")
	flagWS := filepath.Join(t.TempDir(), "flag-ws")
	t.Setenv(workspaceEnv, envWS)

	c := DefaultConfig()
	got, err := c.ResolveWorkspace(flagWS)
	if err != nil {
		t.Fatalf("命令行指定时不该报错：%v", err)
	}
	if want := filepath.Clean(flagWS); got != want {
		t.Errorf("命令行应最优先：got %q, 想要 %q", got, want)
	}

	got, err = c.ResolveWorkspace("")
	if err != nil {
		t.Fatalf("环境变量已设置时不该报错：%v", err)
	}
	if want := filepath.Clean(envWS); got != want {
		t.Errorf("其次才是环境变量：got %q, 想要 %q", got, want)
	}
}

func TestResolveUnconfigured(t *testing.T) {
	// 把各类缓存目录都指到临时位置，避免读到本机真实配置。
	t.Setenv(workspaceEnv, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	// Windows 的 os.UserCacheDir 走 Known Folder API，不受环境变量影响，
	// 所以本机若真有配置文件就跳过，避免误报。
	if path, err := configPath(); err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			t.Skipf("本机存在配置文件 %s，跳过", path)
		}
	}

	c, err := Load()
	if err != nil {
		t.Fatalf("读取默认配置失败：%v", err)
	}
	_, err = c.ResolveWorkspace("")
	if err == nil {
		t.Fatal("没有配置时应报错")
	}
	if !strings.Contains(err.Error(), workspaceEnv) {
		t.Errorf("错误信息应提示 %s：%v", workspaceEnv, err)
	}
}

func TestHintsMatchPlatform(t *testing.T) {
	env := envHint()
	if !strings.Contains(env, workspaceEnv) {
		t.Errorf("envHint 应包含 %s：%q", workspaceEnv, env)
	}
	temp := tempFlagHint()
	if temp == "" {
		t.Fatal("tempFlagHint 不该为空")
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(env, "$env:") {
			t.Errorf("Windows 下应是 PowerShell 写法：%q", env)
		}
		if !strings.Contains(temp, "-Workspace") {
			t.Errorf("Windows 下应提示 shell 函数的 -Workspace：%q", temp)
		}
		return
	}
	if !strings.Contains(env, "export ") {
		t.Errorf("POSIX 下应是 export 写法：%q", env)
	}
	if !strings.Contains(temp, "--workspace") {
		t.Errorf("POSIX 下应提示 gowork 的 --workspace：%q", temp)
	}
}
