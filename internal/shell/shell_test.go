package shell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestPowerShellAcceptsNativeArgs 把生成的函数放进 pwsh，用一个会逐行记录
// 命令行参数的假 gowork.cmd，逐项验证 PowerShell 写法与原生参数的对应关系：
// 原生长选项/短选项/项目名/索引原样透传，Windows 写法翻译成对应的原生参数。
// 只在 Windows + pwsh 下运行。
func TestPowerShellAcceptsNativeArgs(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell 参数行为只在 Windows 下验证")
	}
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("未安装 pwsh")
	}

	script, ok := template("powershell")
	if !ok {
		t.Fatal("缺少 powershell 模板")
	}

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"无参数", nil, nil},
		{"位置项目", []string{"my-app"}, []string{"my-app"}},
		{"原生项目独立值", []string{"--project", "foo"}, []string{"--project", "foo"}},
		{"原生项目等号", []string{"--project=foo"}, []string{"--project=foo"}},
		{"原生缩进独立值", []string{"--indent", "5"}, []string{"--indent", "5"}},
		{"原生缩进等号", []string{"--indent=5"}, []string{"--indent=5"}},
		{"原生工作区独立值", []string{"--workspace", `C:\code`}, []string{"--workspace", `C:\code`}},
		{"原生工作区等号", []string{`--workspace=C:\code`}, []string{`--workspace=C:\code`}},
		{"原生禁用颜色", []string{"--no-color"}, []string{"--no-color"}},
		{"原生 cwd-file", []string{`--cwd-file=C:\tmp`}, []string{`--cwd-file=C:\tmp`}},
		{"短项目", []string{"-p", "foo"}, []string{"-p", "foo"}},
		{"短缩进", []string{"-i", "5"}, []string{"-i", "5"}},
		{"短工作区", []string{"-w", `C:\code`}, []string{"-w", `C:\code`}},
		{"短 cwd-file", []string{"-c", `C:\tmp`}, []string{"-c", `C:\tmp`}},
		{"短禁用颜色", []string{"-n"}, []string{"-n"}},
		{"短选项组合", []string{"-ni", "5"}, []string{"-ni", "5"}},
		{"裸索引", []string{"-1"}, []string{"-1"}},
		{"短帮助", []string{"-h"}, []string{"-h"}},
		{"Windows 项目", []string{"-Project", "foo"}, []string{"--project", "foo"}},
		{"Windows 项目等号", []string{"-Project=foo"}, []string{"--project=foo"}},
		{"Windows 项目缩写", []string{"-Proj", "foo"}, []string{"--project", "foo"}},
		{"Windows 项目单字母大写", []string{"-P", "foo"}, []string{"--project", "foo"}},
		{"Windows 项目小写缩写", []string{"-proj", "foo"}, []string{"--project", "foo"}},
		{"Windows 缩进", []string{"-IndentSize", "5"}, []string{"--indent", "5"}},
		{"Windows 缩进等号", []string{"-IndentSize=5"}, []string{"--indent=5"}},
		{"Windows 缩进缩写", []string{"-Ind", "5"}, []string{"--indent", "5"}},
		{"Windows 工作区", []string{"-Workspace", `C:\code`}, []string{"--workspace", `C:\code`}},
		{"Windows 工作区等号", []string{`-Workspace=C:\code`}, []string{`--workspace=C:\code`}},
		{"Windows 工作区缩写", []string{"-Work", `C:\code`}, []string{"--workspace", `C:\code`}},
		{"带空格项目", []string{"-Project", "my app"}, []string{"--project", "my app"}},
		{"带空格工作区", []string{`-Workspace=C:\my code`}, []string{`--workspace=C:\my code`}},
	}

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	fake := filepath.Join(dir, "gowork.cmd")
	// 每行记录一个参数：%~1 去掉外层引号，shift 逐个前移，便于断言含空格的参数。
	content := "@echo off\r\n" +
		">>\"" + argvFile + "\" echo CALL\r\n" +
		":loop\r\n" +
		"if \"%~1\"==\"\" goto done\r\n" +
		">>\"" + argvFile + "\" echo %~1\r\n" +
		"shift\r\n" +
		"goto loop\r\n" +
		":done\r\n" +
		"exit /b 0\r\n"
	if err := os.WriteFile(fake, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var runner strings.Builder
	runner.WriteString(script)
	runner.WriteString("\n$env:GOWORK_BIN = '")
	runner.WriteString(strings.ReplaceAll(fake, "'", "''"))
	runner.WriteString("'\n")
	for _, c := range cases {
		runner.WriteString("work")
		for _, arg := range c.args {
			runner.WriteString(" ")
			runner.WriteString(psQuote(arg))
		}
		runner.WriteString("\n")
	}
	path := filepath.Join(dir, "run.ps1")
	if err := os.WriteFile(path, []byte("\ufeff"+runner.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command("pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-File", path).CombinedOutput(); err != nil {
		t.Fatalf("执行 work 失败：%v\n%s", err, out)
	}

	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("假 gowork 没有收到参数：%v", err)
	}
	calls := splitCalls(string(raw))
	if len(calls) != len(cases) {
		t.Fatalf("记录了 %d 次调用，想要 %d 次：\n%s", len(calls), len(cases), raw)
	}

	for i, c := range cases {
		got := calls[i]
		// 第一个参数必须由 work 函数自己注入；这里只要求它是个选项，
		// 不绑定具体名字，避免把内部参数写死成 cwd-file。
		if len(got) == 0 || !strings.HasPrefix(got[0], "-") {
			t.Errorf("%s：第一次调用缺少内部参数：%v", c.name, got)
			continue
		}
		// 第一项是函数自己注入的内部参数，之后才是用户参数。
		got = got[1:]
		if !slices.Equal(got, c.want) {
			t.Errorf("%s：得到 %v，想要 %v", c.name, got, c.want)
		}
	}
}

// psQuote 把字符串包成 PowerShell 单引号字面量。
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// splitCalls 按假 gowork 写下的 CALL 标记拆分每次调用的参数。
func splitCalls(raw string) [][]string {
	var calls [][]string
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		switch line {
		case "CALL":
			calls = append(calls, nil)
		case "":
			// 忽略文件末尾换行产生的空行。
		default:
			if len(calls) == 0 {
				calls = append(calls, nil)
			}
			calls[len(calls)-1] = append(calls[len(calls)-1], line)
		}
	}
	return calls
}

func TestInit(t *testing.T) {
	for _, name := range []string{"powershell", "PowerShell", "pwsh", "sh", "bash", "zsh", "posix", "fish"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Init([]string{name}, &out); err != nil {
				t.Fatalf("Init(%q) 返回错误：%v", name, err)
			}
			script := out.String()
			for _, want := range []string{"work", "--cwd-file", "GOWORK_BIN"} {
				if !strings.Contains(script, want) {
					t.Errorf("输出缺少 %q：\n%s", want, script)
				}
			}
		})
	}
}

// TestZshTemplateIncludesCompletion 锁定 zsh 模板包含补全所需的全部要素。
func TestZshTemplateIncludesCompletion(t *testing.T) {
	s, ok := template("zsh")
	if !ok {
		t.Fatal("缺少 zsh 模板")
	}
	for _, want := range []string{
		"work()",
		"_gowork()",
		"_arguments",
		"complete projects",
		"compdef _gowork work",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("zsh 模板缺少 %q：\n%s", want, s)
		}
	}
}

// TestShTemplateHasNoCompletion 保证 init sh 保持原样，不把 zsh 专用语法带进去。
func TestShTemplateHasNoCompletion(t *testing.T) {
	s, ok := template("sh")
	if !ok {
		t.Fatal("缺少 sh 模板")
	}
	for _, unwanted := range []string{"compdef", "_gowork", "_arguments"} {
		if strings.Contains(s, unwanted) {
			t.Errorf("init sh 不应包含 %q：\n%s", unwanted, s)
		}
	}
}

// TestZshCompletionRegisters 在真实 zsh 里加载 compinit 后执行模板，
// 确认 work 被注册到 _gowork。没装 zsh 就跳过（CI 会装）。
func TestZshCompletionRegisters(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("未安装 zsh")
	}
	script, ok := template("zsh")
	if !ok {
		t.Fatal("缺少 zsh 模板")
	}

	check := `autoload -Uz compinit && compinit -u -d "$PWD/zcompdump"
` + script + `
[[ ${_comps[work]} == _gowork ]] || { print -r -- "work 未注册到 _gowork: ${_comps[work]}" >&2; exit 1 }
whence -w work
whence -w _gowork
`

	cmd := exec.Command("zsh", "-df")
	cmd.Dir = t.TempDir()
	cmd.Stdin = strings.NewReader(check)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh 加载模板失败：%v\n%s", err, out)
	}
	for _, want := range []string{"work: function", "_gowork: function"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("输出缺少 %q：\n%s", want, out)
		}
	}
}

// TestZshCompletionSuggestsIndexes 用一个假的 gowork 提供两个项目，
// 确认裸 -N 分支给出 -1、-2 而不是把 -1 交给 _arguments。
func TestZshCompletionSuggestsIndexes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 上没有可直接执行的 zsh 测试环境")
	}
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("未安装 zsh")
	}
	script, ok := template("zsh")
	if !ok {
		t.Fatal("缺少 zsh 模板")
	}

	fake := filepath.Join(t.TempDir(), "gowork")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'alpha\\nbeta\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	harness := "export GOWORK_BIN=" + shQuote(fake) + "\n" + script + `
compadd() { print -r -- "CAND:$@" }
words=(work -1)
CURRENT=2
_gowork
`

	cmd := exec.Command("zsh", "-df")
	cmd.Stdin = strings.NewReader(harness)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("执行补全函数失败：%v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"CAND:-U -- -1", "CAND:-U -- -2"} {
		if !strings.Contains(got, want) {
			t.Errorf("输出缺少 %q：\n%s", want, got)
		}
	}
	if strings.Contains(got, "-3") {
		t.Errorf("只有两个项目，不该补 -3：\n%s", got)
	}
}

// shQuote 把字符串包成 POSIX shell 单引号字面量。
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func TestInitErrors(t *testing.T) {
	cases := []struct {
		desc string
		args []string
	}{
		{"缺少 shell", nil},
		{"多给了参数", []string{"sh", "fish"}},
		{"不支持的 shell", []string{"nushell"}},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var out bytes.Buffer
			if err := Init(c.args, &out); err == nil {
				t.Fatalf("想要错误，得到 nil；输出：%s", out.String())
			}
		})
	}
}

// TestPowerShellTemplateIsASCII 锁定 PowerShell 模板不含非 ASCII 字符。
// pwsh 会按控制台代码页解码 `gowork init powershell` 的输出（中文系统常见 GBK/936），
// UTF-8 中文会被解坏并吞掉换行/引号，让 profile 里的 Invoke-Expression 报假的语法错误。
func TestPowerShellTemplateIsASCII(t *testing.T) {
	s, ok := template("powershell")
	if !ok {
		t.Fatal("缺少 powershell 模板")
	}
	for i, r := range s {
		if r > 0x7f {
			t.Fatalf("PowerShell 模板在字节 %d 含非 ASCII 字符 %q（U+%04X），请改用英文注释与提示语", i, r, r)
		}
	}
}

// TestScriptSyntax 用各 shell 自己的解析器检查生成的代码，没装的 shell 跳过。
// 都只解析、不执行：bash / zsh / dash / fish 用 -n，PowerShell 用 Parser::ParseFile。
func TestScriptSyntax(t *testing.T) {
	cases := []struct {
		shell string
		prog  string
	}{
		{"sh", "bash"},
		{"sh", "zsh"},
		{"sh", "dash"},
		{"zsh", "zsh"},
		{"fish", "fish"},
		{"powershell", "pwsh"},
		{"powershell", "powershell"},
	}

	for _, c := range cases {
		t.Run(c.shell+"/"+c.prog, func(t *testing.T) {
			if _, err := exec.LookPath(c.prog); err != nil {
				t.Skipf("未安装 %s", c.prog)
			}
			script, ok := template(c.shell)
			if !ok {
				t.Fatalf("template(%q) 没有对应脚本", c.shell)
			}

			out, err := checkSyntax(t, c.prog, script)
			if err != nil {
				t.Fatalf("%s 解析失败：%v\n%s", c.prog, err, out)
			}
		})
	}
}

// checkSyntax 按解释器选择解析方式，返回命令输出与错误。
func checkSyntax(t *testing.T, prog, script string) ([]byte, error) {
	t.Helper()
	switch prog {
	case "powershell", "pwsh":
		return checkPowerShell(t, prog, script)
	}
	cmd := exec.Command(prog, "-n")
	cmd.Stdin = strings.NewReader(script)
	return cmd.CombinedOutput()
}

// checkPowerShell 用 PowerShell 自带的解析器检查脚本，只解析不执行。
// 脚本写进临时文件再解析，不走 stdin：stdin 按控制台代码页解码，
// 在 GBK 代码页下中文注释会被解坏，行号错位后报出假的语法错误。
func checkPowerShell(t *testing.T, prog, script string) ([]byte, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "init.ps1")
	// BOM 让 Windows PowerShell 5.1 与 PowerShell 7 都按 UTF-8 读取。
	if err := os.WriteFile(path, []byte("\ufeff"+script), 0o644); err != nil {
		t.Fatalf("写入临时脚本：%v", err)
	}

	const check = `$t = $null; $e = $null
[void][System.Management.Automation.Language.Parser]::ParseFile($env:GOWORK_INIT_SCRIPT, [ref]$t, [ref]$e)
if ($e.Count) { $e | ForEach-Object { Write-Error $_ }; exit 1 }`

	cmd := exec.Command(prog, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", check)
	cmd.Env = append(os.Environ(), "GOWORK_INIT_SCRIPT="+path)
	return cmd.CombinedOutput()
}

// TestCheckPowerShellDetectsError 确认上面的检查不是空转。
func TestCheckPowerShellDetectsError(t *testing.T) {
	for _, prog := range []string{"pwsh", "powershell"} {
		t.Run(prog, func(t *testing.T) {
			if _, err := exec.LookPath(prog); err != nil {
				t.Skipf("未安装 %s", prog)
			}
			if _, err := checkPowerShell(t, prog, "function work {"); err == nil {
				t.Fatal("语法错误的脚本应当报错")
			}
		})
	}
}
