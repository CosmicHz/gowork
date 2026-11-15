package sysinfo

import "testing"

// TestMountPointFromTable 覆盖最长挂载点匹配与 /proc/mounts 的八进制转义。
func TestMountPointFromTable(t *testing.T) {
	const table = `/dev/sda1 / ext4 rw 0 0
/dev/sdb1 /home ext4 rw 0 0
/dev/sdc1 /home/user/My\040Drive ext4 rw 0 0
`
	cases := []struct{ path, want string }{
		{"/", "/"},
		{"/etc/hosts", "/"},
		{"/home", "/home"},
		{"/home/user/x", "/home"},
		{"/home/user/My Drive/x", "/home/user/My Drive"},
	}
	for _, c := range cases {
		if got := mountPointFromTable(c.path, table); got != c.want {
			t.Errorf("mountPointFromTable(%q) = %q, 想要 %q", c.path, got, c.want)
		}
	}
}

func TestMountPointFromTableMissing(t *testing.T) {
	if got := mountPointFromTable("/tmp/x", ""); got != "" {
		t.Errorf("空挂载表应返回空挂载点，得到 %q", got)
	}
}
