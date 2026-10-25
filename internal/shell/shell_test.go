package shell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
