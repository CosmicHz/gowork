//go:build !windows

package main

import (
	"os"
	"testing"
)

// linkDir 建目录符号链接；当前环境建不了就跳过。
func linkDir(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("当前环境建不了符号链接：%v", err)
	}
}
