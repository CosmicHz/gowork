// Package config 负责工作区解析、用户配置与缓存目录。
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// workspaceEnv 覆盖工作区的环境变量名。
const workspaceEnv = "GOWORK_WORKSPACE"

// 配置文件名，放在用户配置目录下的 gowork/ 里。
const configFileName = "config.toml"

const (
	// DefaultIndent 是仪表盘默认左侧缩进；MinIndent 是允许的最小值。
	DefaultIndent = 3
	MinIndent     = 3

	DefaultRecentLimit     = 4
	DefaultRecentNameWidth = 35
	MinRecentNameWidth     = 5
	DefaultDiskWarnGB      = 10

	// 彩虹主色的默认参数；console 构造时也从这里取默认值。
	DefaultRainbowChance    = 0.05
	DefaultRainbowFreq      = 0.2
	DefaultRainbowSpread    = 1.0
	DefaultRainbowPhaseStep = 2.0
)

// Config 是从配置文件解析，补好默认值后的运行配置。
// 配置文件没写的字段采用默认值，否则用填写的值
type Config struct {
	Workspace        string
	Indent           int
	NoColor          bool
	RecentLimit      int
	RecentNameWidth  int
	DiskWarnGB       float64
	ShowDisk         bool
	ShowGit          bool
	ShowHoliday      bool
	ShowRecent       bool
	ClearScreen      bool
	RainbowChance    float64
	RainbowFreq      float64
	RainbowSpread    float64
	RainbowPhaseStep float64
	Quotes           []string
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		Indent:           DefaultIndent,
		RecentLimit:      DefaultRecentLimit,
		RecentNameWidth:  DefaultRecentNameWidth,
		DiskWarnGB:       DefaultDiskWarnGB,
		ShowDisk:         true,
		ShowGit:          true,
		ShowHoliday:      true,
		ShowRecent:       true,
		ClearScreen:      true,
		RainbowChance:    DefaultRainbowChance,
		RainbowFreq:      DefaultRainbowFreq,
		RainbowSpread:    DefaultRainbowSpread,
		RainbowPhaseStep: DefaultRainbowPhaseStep,
	}
}

// fileConfig 是 config.toml 的结构：与显示相关的一组开关各自成表，其余是顶层标量。
// 所有标量与表都用指针，nil 表示文件里没写，保留默认值；显式写 0 / false 也能覆盖默认值。
type fileConfig struct {
	Workspace   *string       `toml:"workspace"`
	Indent      *int          `toml:"indent"`
	NoColor     *bool         `toml:"no_color"`
	ClearScreen *bool         `toml:"clear_screen"`
	Quotes      []string      `toml:"quotes"`
	Disk        *diskTable    `toml:"disk"`
	Git         *gitTable     `toml:"git"`
	Holiday     *holidayTable `toml:"holiday"`
	Recent      *recentTable  `toml:"recent"`
	Rainbow     *rainbowTable `toml:"rainbow"`
}

type diskTable struct {
	Show   *bool    `toml:"show"`
	WarnGB *float64 `toml:"warn_gb"`
}

type gitTable struct {
	Show *bool `toml:"show"`
}

type holidayTable struct {
	Show *bool `toml:"show"`
}

type recentTable struct {
	Show      *bool `toml:"show"`
	Limit     *int  `toml:"limit"`
	NameWidth *int  `toml:"name_width"`
}

type rainbowTable struct {
	Chance    *float64 `toml:"chance"`
	Freq      *float64 `toml:"freq"`
	Spread    *float64 `toml:"spread"`
	PhaseStep *float64 `toml:"phase_step"`
}

// apply 把文件里写了的值覆盖到 c 上；没写的字段保持默认值。
func (f fileConfig) apply(c *Config) {
	set(&c.Workspace, f.Workspace)
	set(&c.Indent, f.Indent)
	set(&c.NoColor, f.NoColor)
	set(&c.ClearScreen, f.ClearScreen)
	if f.Quotes != nil {
		c.Quotes = f.Quotes
	}
	if f.Disk != nil {
		set(&c.ShowDisk, f.Disk.Show)
		set(&c.DiskWarnGB, f.Disk.WarnGB)
	}
	if f.Git != nil {
		set(&c.ShowGit, f.Git.Show)
	}
	if f.Holiday != nil {
		set(&c.ShowHoliday, f.Holiday.Show)
	}
	if f.Recent != nil {
		set(&c.ShowRecent, f.Recent.Show)
		set(&c.RecentLimit, f.Recent.Limit)
		set(&c.RecentNameWidth, f.Recent.NameWidth)
	}
	if f.Rainbow != nil {
		set(&c.RainbowChance, f.Rainbow.Chance)
		set(&c.RainbowFreq, f.Rainbow.Freq)
		set(&c.RainbowSpread, f.Rainbow.Spread)
		set(&c.RainbowPhaseStep, f.Rainbow.PhaseStep)
	}
}

// set 在 src 非 nil 时把值写到 dst，避免每个字段都写一遍 if。
func set[T any](dst, src *T) {
	if src != nil {
		*dst = *src
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

	var f fileConfig
	if err := toml.Unmarshal(raw, &f); err != nil {
		return Config{}, fmt.Errorf("解析配置文件 %s：%w", path, err)
	}
	f.apply(&c)
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
		return fmt.Errorf("recent.limit=%d 小于最小值 1", c.RecentLimit)
	}
	if c.RecentNameWidth < MinRecentNameWidth {
		return fmt.Errorf("recent.name_width=%d 小于最小值 %d", c.RecentNameWidth, MinRecentNameWidth)
	}
	if c.DiskWarnGB < 0 {
		return fmt.Errorf("disk.warn_gb=%v 不能为负数", c.DiskWarnGB)
	}
	if c.RainbowChance < 0 || c.RainbowChance > 1 {
		return fmt.Errorf("rainbow.chance=%v 必须在 0 到 1 之间", c.RainbowChance)
	}
	if c.RainbowFreq < 0 {
		return fmt.Errorf("rainbow.freq=%v 不能为负数", c.RainbowFreq)
	}
	if c.RainbowSpread <= 0 {
		return fmt.Errorf("rainbow.spread=%v 必须大于 0", c.RainbowSpread)
	}
	if c.RainbowPhaseStep < 0 {
		return fmt.Errorf("rainbow.phase_step=%v 不能为负数", c.RainbowPhaseStep)
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
      workspace = '<工作区路径>'
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

// ConfigDir 返回用户配置目录下的 gowork（Windows 是 %APPDATA%\gowork，
// macOS 是 ~/Library/Application Support/gowork，Linux/WSL 是 ~/.config/gowork）。
func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "gowork")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// CacheDir 返回用户缓存目录下的 gowork（Windows 是 %LOCALAPPDATA%\gowork，
// macOS 是 ~/Library/Caches/gowork，Linux/WSL 是 ~/.cache/gowork），目前放节假日缓存。
func CacheDir() (string, error) {
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
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}
