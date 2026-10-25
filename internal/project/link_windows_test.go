package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/windows"
)

// linkDir 建目录符号链接：直接失败时尝试启用 SeCreateSymbolicLinkPrivilege 再建，
// 仍失败就跳过。不退回目录联接。
func linkDir(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if enableSymlinkPrivilege() {
		if err = os.Symlink(target, link); err == nil {
			return
		}
	}
	t.Skipf("当前环境建不了符号链接：%v", err)
}

// enableSymlinkPrivilege 尝试在当前进程令牌里启用创建符号链接的特权。
// 令牌里本来就没有这个特权时（普通用户）会失败，由调用方跳过用例。
func enableSymlinkPrivilege() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeCreateSymbolicLinkPrivilege"), &luid); err != nil {
		return false
	}
	state := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{{
			Luid:       luid,
			Attributes: windows.SE_PRIVILEGE_ENABLED,
		}},
	}
	return windows.AdjustTokenPrivileges(token, false, &state, 0, nil, nil) == nil
}

// TestListFollowsJunction 固定用目录联接覆盖 Windows。
// 目录联接不需要任何特权，符号链接用例因权限跳过时它仍然会跑。
func TestListFollowsJunction(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "link"), real).CombinedOutput(); err != nil {
		t.Fatalf("创建目录联接失败：%v\n%s", err, out)
	}
	if got, want := projectNames(List(root)), []string{"link"}; !slices.Equal(got, want) {
		t.Errorf("指向目录的目录联接应算项目，得到 %v", got)
	}
}
