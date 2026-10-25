//go:build !windows

package sysinfo

// EnableVT 在 POSIX 终端上不需要额外开关：ANSI 转义序列本来就是默认协议，
// 终端编码也由 locale 决定，不像 Windows 控制台那样要显式切到 UTF-8 代码页。
func EnableVT() {}
