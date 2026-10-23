package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveWorkspacePriority(t *testing.T) {
	envWS := filepath.Join(t.TempDir(), "env-ws")
	flagWS := filepath.Join(t.TempDir(), "flag-ws")
	t.Setenv(workspaceEnv, envWS)

	got, err := resolveWorkspace(flagWS)
	if err != nil {
		t.Fatalf("命令行指定时不该报错：%v", err)
	}
	if want := filepath.Clean(flagWS); got != want {
		t.Errorf("命令行应最优先：got %q, 想要 %q", got, want)
	}

	got, err = resolveWorkspace("")
	if err != nil {
		t.Fatalf("环境变量已设置时不该报错：%v", err)
	}
	if want := filepath.Clean(envWS); got != want {
		t.Errorf("其次才是环境变量：got %q, 想要 %q", got, want)
	}
}

func TestResolveWorkspaceUnconfigured(t *testing.T) {
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

	_, err := resolveWorkspace("")
	if err == nil {
		t.Fatal("没有配置时应报错")
	}
	if !strings.Contains(err.Error(), workspaceEnv) {
		t.Errorf("错误信息应提示 %s：%v", workspaceEnv, err)
	}
}

func TestConfigHintsMatchPlatform(t *testing.T) {
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
			t.Errorf("Windows 下应提示启动器的 -Workspace：%q", temp)
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
