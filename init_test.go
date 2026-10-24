package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitShell(t *testing.T) {
	for _, shell := range []string{"powershell", "PowerShell", "pwsh", "sh", "bash", "zsh", "posix", "fish"} {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := initShell([]string{shell}, &out); err != nil {
				t.Fatalf("initShell(%q) 返回错误：%v", shell, err)
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

func TestInitShellErrors(t *testing.T) {
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
			if err := initShell(c.args, &out); err == nil {
				t.Fatalf("想要错误，得到 nil；输出：%s", out.String())
			}
		})
	}
}

// TestInitScriptSyntax 用各 shell 自己的解析器检查生成的代码，没装的 shell 跳过。
// 都只解析、不执行：bash / fish 用 -n，PowerShell 用 Parser::ParseFile。
func TestInitScriptSyntax(t *testing.T) {
	for _, shell := range []string{"sh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			prog := shellProg(shell)
			if _, err := exec.LookPath(prog); err != nil {
				t.Skipf("未安装 %s", prog)
			}
			script, ok := getInitScript(shell)
			if !ok {
				t.Fatalf("getInitScript(%q) 没有对应脚本", shell)
			}

			out, err := checkSyntax(t, shell, script)
			if err != nil {
				t.Fatalf("%s 解析失败：%v\n%s", prog, err, out)
			}
		})
	}
}

// checkSyntax 按 shell 选择解析方式，返回命令输出与错误。
func checkSyntax(t *testing.T, shell, script string) ([]byte, error) {
	t.Helper()
	if shell == "powershell" {
		return checkPowerShell(t, script)
	}
	cmd := exec.Command(shellProg(shell), "-n")
	cmd.Stdin = strings.NewReader(script)
	return cmd.CombinedOutput()
}

func shellProg(shell string) string {
	if shell == "fish" {
		return "fish"
	}
	return "bash"
}

// checkPowerShell 用 PowerShell 自带的解析器检查脚本，只解析不执行。
// 脚本写进临时文件再解析，不走 stdin：stdin 按控制台代码页解码，
// 在 GBK 代码页下中文注释会被解坏，行号错位后报出假的语法错误。
func checkPowerShell(t *testing.T, script string) ([]byte, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "init.ps1")
	// BOM 让 Windows PowerShell 5.1 与 PowerShell 7 都按 UTF-8 读取。
	if err := os.WriteFile(path, []byte("\ufeff"+script), 0o644); err != nil {
		t.Fatalf("写入临时脚本：%v", err)
	}

	const check = `$t = $null; $e = $null
[void][System.Management.Automation.Language.Parser]::ParseFile($env:GOWORK_INIT_SCRIPT, [ref]$t, [ref]$e)
if ($e.Count) { $e | ForEach-Object { Write-Error $_ }; exit 1 }`

	cmd := exec.Command("pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", check)
	cmd.Env = append(os.Environ(), "GOWORK_INIT_SCRIPT="+path)
	return cmd.CombinedOutput()
}

// TestCheckPowerShellDetectsError 确认上面的检查不是空转。
func TestCheckPowerShellDetectsError(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("未安装 pwsh")
	}
	if _, err := checkPowerShell(t, "function work {"); err == nil {
		t.Fatal("语法错误的脚本应当报错")
	}
}
