//go:build !windows && !linux && !darwin

package sysinfo

import "golang.org/x/sys/unix"

// mountPoint 在未专门适配的 POSIX 平台上不提供挂载点。
func mountPoint(string, *unix.Statfs_t) string { return "" }
