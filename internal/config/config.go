// Package config 负责工作区解析、用户配置与缓存目录。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// workspaceEnv 覆盖工作区的环境变量名。
const workspaceEnv = "GOWORK_WORKSPACE"

const (
	// DefaultIndent 是仪表盘默认左侧缩进；MinIndent 是允许的最小值。
	DefaultIndent = 3
	MinIndent     = 3

	DefaultRecentLimit     = 4
	DefaultRecentNameWidth = 35
	MinRecentNameWidth     = 5
	DefaultDiskWarnGB      = 10
)

// Config 是从配置文件解析，补好默认值后的运行配置。
// JSON 里没写的字段采用默认值，否则用填写的值
type Config struct {
	Workspace       string   `json:"workspace"`
	Indent          int      `json:"indent"`
	NoColor         bool     `json:"no_color"`
	RecentLimit     int      `json:"recent_limit"`
	RecentNameWidth int      `json:"recent_name_width"`
	DiskWarnGB      float64  `json:"disk_warn_gb"`
	ShowDisk        bool     `json:"show_disk"`
	ShowGit         bool     `json:"show_git"`
	ShowHoliday     bool     `json:"show_holiday"`
	ShowRecent      bool     `json:"show_recent"`
	ClearScreen     bool     `json:"clear_screen"`
	Quotes          []string `json:"quotes"`
}

// DefaultConfig 返回默认配置。load 先以它为底稿，再让 JSON 覆盖显式字段。
func DefaultConfig() Config {
	return Config{
		Indent:          DefaultIndent,
		RecentLimit:     DefaultRecentLimit,
		RecentNameWidth: DefaultRecentNameWidth,
		DiskWarnGB:      DefaultDiskWarnGB,
		ShowDisk:        true,
		ShowGit:         true,
		ShowHoliday:     true,
		ShowRecent:      true,
		ClearScreen:     true,
	}
}

// Load 读取用户配置文件；文件不存在时返回默认配置。
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	return load(path)
}

// load 从指定路径读取配置，以便测试不碰真实用户目录。
func load(path string) (Config, error) {
	c := DefaultConfig()

	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件 %s：%w", path, err)
	}

	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("解析配置文件 %s：%w", path, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("配置文件 %s：%w", path, err)
	}
	return c, nil
}

// Validate 检查配置里的数值范围，避免把明显的笔误带进运行期。
func (c Config) Validate() error {
	if c.Indent < MinIndent {
		return fmt.Errorf("indent=%d 小于最小值 %d", c.Indent, MinIndent)
	}
	if c.RecentLimit < 1 {
		return fmt.Errorf("recent_limit=%d 小于最小值 1", c.RecentLimit)
	}
	if c.RecentNameWidth < MinRecentNameWidth {
		return fmt.Errorf("recent_name_width=%d 小于最小值 %d", c.RecentNameWidth, MinRecentNameWidth)
	}
	if c.DiskWarnGB < 0 {
		return fmt.Errorf("disk_warn_gb=%v 不能为负数", c.DiskWarnGB)
	}
	for i, q := range c.Quotes {
		if strings.TrimSpace(q) == "" {
			return fmt.Errorf("quotes[%d]=%q 不能为空", i, q)
		}
	}
	return nil
}

// ResolveWorkspace 按优先级解析工作区：命令行 > 环境变量 > 配置文件。
// 三者都没有就报错要求显式配置。
func (c Config) ResolveWorkspace(flagValue string) (string, error) {
	if flagValue != "" { // 命令行参数指定
		return filepath.Clean(flagValue), nil
	}
	if env := os.Getenv(workspaceEnv); env != "" { // 环境变量指定
		return filepath.Clean(env), nil
	}
	if c.Workspace != "" { // 配置文件指定
		return filepath.Clean(c.Workspace), nil
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

// Dir 返回用户缓存目录下的 gowork（Windows 是 %LOCALAPPDATA%\gowork，
// macOS 是 ~/Library/Caches/gowork，Linux/WSL 是 ~/.cache/gowork），配置与缓存共用。
func Dir() (string, error) {
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
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}
