package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// workspaceEnv 覆盖工作区的环境变量名。
const workspaceEnv = "GOWORK_WORKSPACE"

type config struct {
	Workspace string `json:"workspace"`
}

// resolveWorkspace 按优先级解析工作区：命令行 > 环境变量 > 配置文件。
// 三者都没有就报错要求显式配置——工具不猜默认路径，也不该把某个人的目录写进源码。
func resolveWorkspace(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Clean(flagValue), nil
	}
	if env := os.Getenv(workspaceEnv); env != "" {
		return filepath.Clean(env), nil
	}
	if ws, ok := readWorkspaceConfig(); ok {
		return filepath.Clean(ws), nil
	}

	path, err := configPath()
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf(`未配置工作区，任选一种方式：
  · 环境变量：%s
  · 配置文件：%s
      { "workspace": "<工作区路径>" }
  · 临时指定：%s`, envHint(), path, tempFlagHint())
}

// envHint 返回设置工作区环境变量的当前平台写法。
func envHint() string {
	if runtime.GOOS == "windows" {
		return "$env:" + workspaceEnv + " = '<工作区路径>'"
	}
	return "export " + workspaceEnv + "='<工作区路径>'"
}

// tempFlagHint 返回临时指定工作区的写法：PowerShell 的 shell 函数暴露 -Workspace，
// POSIX 下的工作流直接把参数透传给 gowork，所以用它的 --workspace。
func tempFlagHint() string {
	if runtime.GOOS == "windows" {
		return "work -Workspace <工作区路径>"
	}
	return "work --workspace <工作区路径>"
}

func readWorkspaceConfig() (string, bool) {
	path, err := configPath()
	if err != nil {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var c config
	if err := json.Unmarshal(raw, &c); err != nil || c.Workspace == "" {
		return "", false
	}
	return c.Workspace, true
}

// appDir 返回用户缓存目录下的 gowork（Windows 是 %LOCALAPPDATA%\gowork，
// macOS 是 ~/Library/Caches/gowork，Linux/WSL 是 ~/.cache/gowork），配置与缓存共用。
func appDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "gowork")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}
