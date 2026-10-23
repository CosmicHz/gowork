package main

import "testing"

func TestRepoURL(t *testing.T) {
	yes := []string{
		"https://github.com/octocat/Hello-World.git",
		"http://host/a/b",
		"HTTPS://GITHUB.COM/a/b",
		"ssh://git@host:2222/o/r.git",
		"git://host/o/r",
		"ftp://host/o/r",
		"ftps://host/o/r",
		"git@github.com:owner/repo.git",
	}
	for _, s := range yes {
		if !repoURL(s) {
			t.Errorf("repoURL(%q) = false, 想要 true", s)
		}
	}

	// 裸写域名、本地路径、普通项目名都必须当成项目名，不能误判成地址。
	no := []string{
		"github.com/foo/bar",
		"my-app",
		"my-a",
		"-1",
		"",
		`C:\Projects\x`,
		"./local",
		"../sibling",
	}
	for _, s := range no {
		if repoURL(s) {
			t.Errorf("repoURL(%q) = true, 想要 false", s)
		}
	}
}

func TestRepoName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/octocat/Hello-World.git": "Hello-World",
		"https://github.com/octocat/Hello-World":     "Hello-World",
		"https://github.com/octocat/Hello-World/":    "Hello-World",
		"git@github.com:owner/repo.git":              "repo",
		"git@github.com:owner/repo":                  "repo",
		"ssh://git@host:2222/owner/repo.git":         "repo",
		"https://host/a/b/c.git":                     "c",
	}
	for in, want := range cases {
		if got := repoName(in); got != want {
			t.Errorf("repoName(%q) = %q, 想要 %q", in, got, want)
		}
	}
}

func TestValidRepoName(t *testing.T) {
	bad := []string{"", ".", "..", "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b"}
	for _, n := range bad {
		if validRepoName(n) {
			t.Errorf("validRepoName(%q) = true, 想要 false", n)
		}
	}

	good := []string{"repo", ".hidden", "a b", "中文项目", "Hello-World"}
	for _, n := range good {
		if !validRepoName(n) {
			t.Errorf("validRepoName(%q) = false, 想要 true", n)
		}
	}
}
