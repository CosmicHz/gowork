//go:build darwin

package sysinfo

import "golang.org/x/sys/unix"

// mountPoint 直接取 statfs 返回的挂载点（Mntonname）。
func mountPoint(_ string, st *unix.Statfs_t) string {
	return unix.ByteSliceToString(st.Mntonname[:])
}
