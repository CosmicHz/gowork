package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// scpLikePattern 匹配 git 的 scp 形式地址：可带用户名的 [user@]host:path。
// 与 git 一致，用户名可以省略（如 github.com:owner/repo.git）。
var scpLikePattern = regexp.MustCompile(`^([^/\\:@]+@)?[^/\\:]+:.+$`)

// urlSchemes 是 git 认的协议。带 + 的两个是 git 对 ssh 的别名。
var urlSchemes = []string{
	"https://", "http://", "ssh://", "git://", "ftp://", "ftps://",
	"git+ssh://", "ssh+git://", "file://",
}

// repoURL 判断参数是不是仓库地址。判定范围与 git 自身保持一致：
// 带协议的 URL 与 scp 形式算地址；裸写域名（github.com/foo/bar）与
// 单字母主机（C:\x 这类盘符路径）不认。
func repoURL(arg string) bool {
	if scpLikePattern.MatchString(arg) && !windowsDrivePath(arg) {
		return true
	}
	lower := strings.ToLower(arg)
	for _, scheme := range urlSchemes {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return false
}

// windowsDrivePath 匹配 C:\x、C:/x、C:relative 这类盘符写法。
// git 把单字母主机当盘符处理成本地路径，我们照做。
func windowsDrivePath(s string) bool {
	if len(s) < 2 || s[1] != ':' {
		return false
	}
	c := s[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// repoName 取地址里最后一段路径，去掉 .git 后缀，作为本地目录名。
func repoName(rawURL string) string {
	s := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	s = strings.TrimSuffix(s, ".git")

	// scp 形式的主机与路径用冒号分隔，不能当路径切
	if !strings.Contains(s, "://") {
		if i := strings.LastIndex(s, ":"); i >= 0 {
			s = s[i+1:]
		}
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// validRepoName 挡住 . / .. / 路径分隔符等会跑出工作区的名字。
func validRepoName(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	return !strings.ContainsAny(name, `/\:*?"<>|`)
}

// cloneProject 把仓库克隆到工作区并返回目标目录。
// 目录已存在时什么都不做，直接进去，所以重复执行是幂等的。
func cloneProject(o options, r *renderer) (string, error) {
	if !isDir(o.workspace) {
		return "", fmt.Errorf("目录不存在：%s", o.workspace)
	}

	name := repoName(o.project)
	if !validRepoName(name) {
		return "", fmt.Errorf("无法从地址推断仓库名：%s", o.project)
	}

	dest := filepath.Join(o.workspace, name)
	if isDir(dest) {
		return dest, nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("未找到 git，无法克隆 %s", o.project)
	}

	r.clearScreen()
	r.println()
	r.println(r.indent + r.paint(r.accent, "正在克隆仓库"))
	r.row("地址", o.project, ansiWhite)
	r.row("目标", dest, ansiWhite)
	r.println()

	// 让 git 直接占用当前控制台，进度条和凭据提示都能正常工作。
	cmd := exec.Command("git", "clone", o.project, dest)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("克隆失败：%s", o.project)
	}
	return dest, nil
}
