package sysinfo

import (
	"strconv"
	"strings"
)

// mountPointFromTable 在 /proc/mounts 风格的文本里找覆盖 abs 的最长挂载点。
// 单独抽出来是为了不依赖具体平台，任意系统都能用 go test 覆盖。
func mountPointFromTable(abs, table string) string {
	best := ""
	for _, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		mount := unescapeMount(fields[1])
		if !covers(mount, abs) || len(mount) <= len(best) {
			continue
		}
		best = mount
	}
	return best
}

// covers 判断挂载点是否覆盖给定路径。
func covers(mount, path string) bool {
	if mount == "" {
		return false
	}
	if mount == "/" {
		return true
	}
	return path == mount || strings.HasPrefix(path, mount+"/")
}

// unescapeMount 还原挂载表里的八进制转义（空格写作 \040 等）。
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
