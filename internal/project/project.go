// Package project 负责扫描工作区、解析目标目录与读取 git 状态。
package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Project 是工作区下的一个一级子目录。Mod 用于「最近活跃」排序。
type Project struct {
	Name string
	Mod  time.Time
}

// IndexPattern 匹配 -1 / -2 / ... 形式的“按最近修改排序的第 N 个项目”。
// cli 改写裸索引参数时也用它，避免两处各写一份。
var IndexPattern = regexp.MustCompile(`^-(\d+)$`)

// List 列出工作区下的子目录，跳过 . 开头的隐藏目录，按最近修改时间倒序。
// 工作区不存在时返回空列表，由 Resolve 统一报“目录不存在”。
func List(root string) []Project {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	projects := make([]Project, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// 用 Stat 而不是 DirEntry.IsDir，这样指向目录的符号链接也算项目。
		info, err := os.Stat(filepath.Join(root, e.Name()))
		if err != nil || !info.IsDir() {
			continue
		}
		projects = append(projects, Project{Name: e.Name(), Mod: info.ModTime()})
	}

	sort.SliceStable(projects, func(i, j int) bool {
		return projects[i].Mod.After(projects[j].Mod)
	})
	return projects
}

// Resolve 把用户输入解析成绝对目录，并返回需要在清屏后展示的警告。
func Resolve(workspace, want string, projects []Project) (string, []string, error) {
	var warnings []string

	if m := IndexPattern.FindStringSubmatch(want); m != nil {
		n, _ := strconv.Atoi(m[1])
		idx := n - 1
		if idx < 0 || idx >= len(projects) {
			return "", nil, fmt.Errorf("项目索引超出范围，有效范围: 1-%d", len(projects))
		}
		want = projects[idx].Name
	}

	if want == "" {
		if !IsDir(workspace) {
			return "", nil, fmt.Errorf("目录不存在：%s", workspace)
		}
		return workspace, nil, nil
	}

	if !IsDir(filepath.Join(workspace, want)) {
		switch hits := prefixMatches(projects, want); len(hits) {
		case 1:
			want = hits[0].Name
		case 0:
			// 保留原样，交给下面统一报“目录不存在”
		default:
			shortest := hits[0]
			for _, h := range hits[1:] {
				if len(h.Name) < len(shortest.Name) {
					shortest = h
				}
			}
			warnings = append(warnings, fmt.Sprintf("前缀 '%s' 匹配到多个项目，跳转到最短匹配：", want))
			for _, h := range hits {
				warnings = append(warnings, h.Name)
			}
			want = shortest.Name
		}
	}

	target := filepath.Join(workspace, want)
	if !IsDir(target) {
		return "", nil, fmt.Errorf("目录不存在：%s", target)
	}
	return target, warnings, nil
}

// prefixMatches 返回名称以 prefix 开头的项目，按名称升序排列。
func prefixMatches(projects []Project, prefix string) []Project {
	lower := strings.ToLower(prefix)
	var hits []Project
	for _, p := range projects {
		if strings.HasPrefix(strings.ToLower(p.Name), lower) {
			hits = append(hits, p)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return strings.ToLower(hits[i].Name) < strings.ToLower(hits[j].Name)
	})
	return hits
}

// IsDir 判断路径是否是指向目录的路径；符号链接会被跟随。
func IsDir(path string) bool {
	info, err := os.Stat(path) // Stat 会跟随符号链接
	return err == nil && info.IsDir()
}

// GitInfo 汇报当前分支与未提交条目数；不是 git 仓库时返回 false。
func GitInfo(target string) (string, int, bool) {
	// .git 在 worktree 场景下是文件而不是目录，所以只判断存在。
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		return "", 0, false
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", 0, false
	}

	out, err := exec.Command("git", "-C", target, "branch", "--show-current").Output()
	if err != nil {
		return "", 0, false
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		branch = "detached"
	}

	status, _ := exec.Command("git", "-C", target, "status", "--porcelain").Output()
	changes := 0
	for _, line := range strings.Split(string(status), "\n") {
		if strings.TrimSpace(line) != "" {
			changes++
		}
	}
	return branch, changes, true
}
