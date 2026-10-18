package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
  · 环境变量：$env:%s = '<工作区路径>'
  · 配置文件：%s
      { "workspace": "<工作区路径>" }
  · 临时指定：work -Workspace <工作区路径>`, workspaceEnv, path)
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

// appDir 返回 %LOCALAPPDATA%\gowork，配置与缓存共用。
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
