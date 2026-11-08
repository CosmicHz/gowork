package cli

import (
	"bytes"
	"flag"
	"slices"
	"strings"
	"testing"

	"gowork/internal/config"
	"gowork/internal/project"
)

func parseForTest(t *testing.T, args []string, cfg config.Config) (Options, error) {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	return parseWith(fs, args, cfg)
}

func TestParseUsesConfigDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Indent = 5
	cfg.NoColor = true
	o, err := parseForTest(t, []string{"my-app"}, cfg)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if o.Config.Indent != 5 || !o.Config.NoColor {
		t.Errorf("应使用配置默认值：%+v", o.Config)
	}
	if o.Project != "my-app" {
		t.Errorf("位置参数应作为项目：%q", o.Project)
	}
}

func TestParseFlagsOverrideConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Indent = 5
	cfg.NoColor = true
	o, err := parseForTest(t, []string{"--indent", "7", "--no-color=false"}, cfg)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if o.Config.Indent != 7 {
		t.Errorf("--indent 应覆盖配置：%d", o.Config.Indent)
	}
	if o.Config.NoColor {
		t.Error("--no-color=false 应覆盖配置里的 true")
	}
}

func TestParseRejectsSmallIndent(t *testing.T) {
	_, err := parseForTest(t, []string{"--indent", "2"}, config.DefaultConfig())
	if err == nil {
		t.Fatal("缩进小于最小值应报错")
	}
	if !strings.Contains(err.Error(), "--indent=2") {
		t.Errorf("错误信息应包含实际值：%v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Indent = 2
	if _, err := parseForTest(t, nil, cfg); err == nil {
		t.Fatal("配置里的缩进也应在解析时校验")
	}
}

func TestParseRecent(t *testing.T) {
	o, err := parseForTest(t, []string{"--recent", "2"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if o.Project != "-2" {
		t.Errorf("--recent 2 应等价于 -2，得到 %q", o.Project)
	}

	o, err = parseForTest(t, []string{"-2"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析裸索引失败：%v", err)
	}
	if o.Project != "-2" {
		t.Errorf("裸 -2 应等价于 --recent 2，得到 %q", o.Project)
	}

	if _, err := parseForTest(t, []string{"--recent", "0"}, config.DefaultConfig()); err == nil {
		t.Fatal("--recent 0 应报错")
	} else if !strings.Contains(err.Error(), "--recent=0") {
		t.Errorf("错误信息应包含 --recent=0：%v", err)
	}
	if _, err := parseForTest(t, []string{"--recent", "-1"}, config.DefaultConfig()); err == nil {
		t.Fatal("--recent 负数应报错")
	}
	if _, err := parseForTest(t, []string{"--project", "foo", "--recent", "2"}, config.DefaultConfig()); err == nil {
		t.Fatal("--project 与 --recent 同时出现应报错")
	} else if !strings.Contains(err.Error(), "--project") || !strings.Contains(err.Error(), "--recent") {
		t.Errorf("错误信息应同时提到两个选项：%v", err)
	}
}

func TestParseMarksProjectAfterTerminator(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		project string
		literal bool
	}{
		{"终止符后的索引写法按名字", []string{"--", "-1"}, "-1", true},
		{"短选项后的终止符", []string{"-n", "--", "-1"}, "-1", true},
		{"带值选项的值恰好是 --", []string{"--workspace", "--", "-1"}, "-1", false},
		{"终止符后的长选项写法", []string{"--", "--no-color"}, "--no-color", true},
		{"项目名之后的选项不改变字面判定", []string{"my-app", "-n", "--", "-1"}, "my-app", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := parseForTest(t, c.args, config.DefaultConfig())
			if err != nil {
				t.Fatalf("解析失败：%v", err)
			}
			if o.Project != c.project || o.ProjectAfterTerminator != c.literal {
				t.Errorf("得到 project=%q literal=%v，期望 project=%q literal=%v",
					o.Project, o.ProjectAfterTerminator, c.project, c.literal)
			}
		})
	}
}

func TestParseAcceptsSingleDashLongOption(t *testing.T) {
	o, err := parseForTest(t, []string{"-no-color"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("单横线长选项应被 flag 接受：%v", err)
	}
	if !o.Config.NoColor {
		t.Error("-no-color 应等同于 --no-color")
	}
}

func TestExpandShortFlags(t *testing.T) {
	cases := []struct {
		desc string
		in   []string
		want []string
	}{
		{"单个布尔短选项", []string{"-n"}, []string{"--no-color"}},
		{"组合两个布尔短选项", []string{"-nh"}, []string{"--no-color", "--help"}},
		{"布尔加带值的短选项", []string{"-ni", "5"}, []string{"--no-color", "--indent", "5"}},
		{"带值选项贴着写", []string{"-ni5"}, []string{"--no-color", "--indent=5"}},
		{"项目短选项独立值", []string{"-p", "my-app"}, []string{"--project", "my-app"}},
		{"项目短选项贴值", []string{"-pmy-app"}, []string{"--project=my-app"}},
		{"项目短选项接受负数", []string{"-p-1"}, []string{"--project=-1"}},
		{"工作区短选项贴值", []string{"-w/tmp/code"}, []string{"--workspace=/tmp/code"}},
		{"裸索引不是短选项", []string{"-1"}, []string{"-1"}},
		{"多位索引不是短选项", []string{"-22"}, []string{"-22"}},
		{"长选项保持原样", []string{"--no-color"}, []string{"--no-color"}},
		{"单横线长选项保持原样", []string{"-no-color"}, []string{"-no-color"}},
		{"未知短选项保持原样", []string{"-x"}, []string{"-x"}},
		{"未知组合保持原样", []string{"-nz"}, []string{"-nz"}},
		{"空参数", nil, []string{}},
		{"-- 之后的参数原样保留", []string{"--", "-n", "my-app"}, []string{"--", "-n", "my-app"}},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := ExpandShortFlags(c.in); !slices.Equal(got, c.want) {
				t.Errorf("ExpandShortFlags(%v) = %v, 想要 %v", c.in, got, c.want)
			}
		})
	}
}

func TestOptionToken(t *testing.T) {
	cases := []struct {
		arg      string
		spelling string
		names    []string
		ok       bool
	}{
		{"-n", "-n", []string{"no-color"}, true},
		{"--no-color", "--no-color", []string{"no-color"}, true},
		{"--no-color=false", "--no-color", []string{"no-color"}, true},
		{"-no-color", "-no-color", []string{"no-color"}, true},
		{"-nh", "-nh", []string{"no-color", "help"}, true},
		{"--project", "--project", []string{"project"}, true},
		{"--project=foo", "--project", []string{"project"}, true},
		{"-project", "-project", []string{"project"}, true},
		{"--recent", "--recent", []string{"recent"}, true},
		{"--recent=2", "--recent", []string{"recent"}, true},
		{"-recent", "-recent", []string{"recent"}, true},
		{"-p", "-p", []string{"project"}, true},
		{"-pfoo", "-p", []string{"project"}, true},
		{"-p-1", "-p", []string{"project"}, true},
		{"-ni", "-ni", []string{"no-color", "indent"}, true},
		{"-ni5", "-ni", []string{"no-color", "indent"}, true},
		{"-c", "-c", []string{"cwd-file"}, true},
		{"-1", "", nil, false},
		{"my-app", "", nil, false},
		{"--", "", nil, false},
		{"-", "", nil, false},
		{"-x", "", nil, false},
		{"--unknown", "", nil, false},
	}

	for _, c := range cases {
		t.Run(c.arg, func(t *testing.T) {
			spelling, names, ok := optionToken(c.arg)
			if ok != c.ok {
				t.Fatalf("optionToken(%q) ok = %v, 想要 %v", c.arg, ok, c.ok)
			}
			if spelling != c.spelling || !slices.Equal(names, c.names) {
				t.Errorf("optionToken(%q) = %q, %v；想要 %q, %v", c.arg, spelling, names, c.spelling, c.names)
			}
		})
	}
}

func TestOptionCollisionWarnings(t *testing.T) {
	projects := []project.Project{
		{Name: "-n"},
		{Name: "--no-color"},
		{Name: "-no-color"},
		{Name: "--project"},
		{Name: "--recent"},
		{Name: "-p"},
		{Name: "-ni"},
		{Name: "-c"},
		{Name: "-i"},
		{Name: "my-app"},
	}

	cases := []struct {
		desc string
		args []string
		used map[string]bool
		want []string
	}{
		{"短选项", []string{"-n"}, map[string]bool{"no-color": true}, []string{"-n"}},
		{"长选项", []string{"--no-color"}, map[string]bool{"no-color": true}, []string{"--no-color"}},
		{"单横线长选项", []string{"-no-color"}, map[string]bool{"no-color": true}, []string{"-no-color"}},
		{"长选项等号", []string{"--project=foo"}, map[string]bool{"project": true}, []string{"--project"}},
		{"recent 长选项", []string{"--recent", "2"}, map[string]bool{"recent": true}, []string{"--recent"}},
		{"短选项贴值", []string{"-pfoo"}, map[string]bool{"project": true}, []string{"-p"}},
		{"短选项组合", []string{"-ni", "5"}, map[string]bool{"no-color": true, "indent": true}, []string{"-ni"}},
		{"重复选项去重", []string{"-n", "-n"}, map[string]bool{"no-color": true}, []string{"-n"}},
		{"cwd-file 不提示", []string{"-c", "x"}, map[string]bool{"cwd-file": true}, nil},
		{"位置参数后的未使用选项", []string{"my-app", "-n"}, map[string]bool{}, nil},
		{"未使用的短选项", []string{"-i", "5"}, map[string]bool{}, nil},
		{"位置参数后同名选项仍提示", []string{"-n", "my-app", "--no-color"}, map[string]bool{"no-color": true}, []string{"-n", "--no-color"}},
		{"-- 后同名选项不提示", []string{"-n", "--", "--no-color"}, map[string]bool{"no-color": true}, []string{"-n"}},
		{"作为选项值的 token 不提示", []string{"-p", "--no-color"}, map[string]bool{"project": true, "no-color": true}, []string{"-p"}},
		{"组合选项的值不提示", []string{"-ni", "--no-color"}, map[string]bool{"no-color": true, "indent": true}, []string{"-ni"}},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			warnings := optionCollisionWarnings(c.args, c.used, projects)
			if len(warnings) != len(c.want) {
				t.Fatalf("得到 %d 条提示，想要 %d 条：%+v", len(warnings), len(c.want), warnings)
			}
			for i, spelling := range c.want {
				if !strings.Contains(warnings[i].Message, spelling) {
					t.Errorf("提示应包含 %q：%+v", spelling, warnings[i])
				}
				details := strings.Join(warnings[i].Details, "\n")
				if !strings.Contains(details, "work ./"+spelling) {
					t.Errorf("应给出 work ./%s 的消歧写法：\n%s", spelling, details)
				}
			}
		})
	}
}

func TestInitCollisionWarning(t *testing.T) {
	w, ok := initCollisionWarning("init", []project.Project{{Name: "init"}})
	if !ok {
		t.Fatal("init 与目录同名时应提示")
	}
	if !strings.Contains(w.Message, "init") || !strings.Contains(strings.Join(w.Details, "\n"), "./init") {
		t.Errorf("提示内容不对：%+v", w)
	}

	if _, ok := initCollisionWarning("Init", []project.Project{{Name: "init"}}); ok {
		t.Error("子命令只认小写 init，Init 不应提示")
	}
	if _, ok := initCollisionWarning("init", []project.Project{{Name: "init-app"}}); ok {
		t.Error("只有前缀命中时不该提示 init 同名")
	}
}

func TestCollisionWarnings(t *testing.T) {
	projects := []project.Project{{Name: "init"}, {Name: "-n"}}
	warnings := collisionWarnings("init", []string{"-n"}, map[string]bool{"no-color": true}, projects)
	if len(warnings) != 2 {
		t.Fatalf("应合并 init 与选项名提示，得到 %+v", warnings)
	}
	if !strings.Contains(warnings[0].Message, "init") || !strings.Contains(warnings[1].Message, "-n") {
		t.Errorf("提示顺序或内容不对：%+v", warnings)
	}
}

func TestOptionsAfterProjectAreParsed(t *testing.T) {
	o, err := parseForTest(t, []string{"my-app", "-n"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if !o.Config.NoColor {
		t.Fatal("项目名之后的 -n 应生效")
	}
	if o.Project != "my-app" {
		t.Errorf("项目名不应被重排改变：%q", o.Project)
	}
}

func TestParseRejectsUnknownOptionAfterProject(t *testing.T) {
	if _, err := parseForTest(t, []string{"my-app", "--unknown"}, config.DefaultConfig()); err == nil {
		t.Fatal("项目名之后的未知选项应报错，而不是静默忽略")
	}
}

func TestParseRejectsProjectWithTargetFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"项目名之后 --recent", []string{"my-app", "--recent", "2"}},
		{"项目名之后 --project", []string{"my-app", "--project", "other"}},
		{"项目名之后 -1", []string{"my-app", "-1"}},
		{"项目名之前 --recent", []string{"--recent", "2", "my-app"}},
		{"项目名之前 --project", []string{"--project", "other", "my-app"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseForTest(t, c.args, config.DefaultConfig()); err == nil {
				t.Fatal("项目名与目标选项同时出现应报错")
			}
		})
	}
}

func TestParseRecordsRawArgsAndUsedFlags(t *testing.T) {
	raw := []string{"-n", "--indent", "5", "my-app"}
	o, err := parseForTest(t, raw, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if !slices.Equal(o.Args, raw) {
		t.Errorf("应保留原始参数：%v", o.Args)
	}
	if !o.UsedFlags["no-color"] || !o.UsedFlags["indent"] {
		t.Errorf("应记录实际生效的选项：%v", o.UsedFlags)
	}

	// 参数重排后，位置参数之后的 -n 也会真正进入 flag 解析。
	o, err = parseForTest(t, []string{"my-app", "-n"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if !o.UsedFlags["no-color"] {
		t.Errorf("位置参数之后的选项应算作已使用：%v", o.UsedFlags)
	}
	if o.Project != "my-app" {
		t.Errorf("项目名不应被重排改变：%q", o.Project)
	}
}

func TestNormalizeArgs(t *testing.T) {
	cases := []struct {
		desc string
		in   []string
		want []string
	}{
		{"组合短选项后仍改写裸索引", []string{"-n", "-1"}, []string{"--no-color", "--recent=1"}},
		{"短选项贴值后仍改写裸索引", []string{"-i5", "-2"}, []string{"--indent=5", "--recent=2"}},
		{"项目短选项的值不能被改写", []string{"-p", "-1"}, []string{"--project", "-1"}},
		{"贴在一起的项目短选项值也不能被改写", []string{"-p-1"}, []string{"--project=-1"}},
		{"-- 之后不展开也不改写", []string{"-n", "--", "-n", "-1"}, []string{"--no-color", "--", "-n", "-1"}},
		{"项目后的布尔选项移到前面", []string{"my-app", "-n"}, []string{"--no-color", "my-app"}},
		{"项目后的带值选项移到前面", []string{"my-app", "-i", "5"}, []string{"--indent", "5", "my-app"}},
		{"项目后的贴值选项移到前面", []string{"my-app", "-i5"}, []string{"--indent=5", "my-app"}},
		{"项目后的值以 - 开头", []string{"my-app", "--workspace", "-foo"}, []string{"--workspace", "-foo", "my-app"}},
		{"-- 作为选项值时继续展开", []string{"my-app", "--workspace", "--", "-n"}, []string{"--workspace", "--", "--no-color", "my-app"}},
		{"项目后的未知选项也移到前面", []string{"my-app", "--unknown"}, []string{"--unknown", "my-app"}},
		{"项目后的索引重排后报冲突", []string{"my-app", "-1"}, []string{"--recent=1", "my-app"}},
		{"项目后的目标选项也重排", []string{"my-app", "--recent", "2"}, []string{"--recent", "2", "my-app"}},
		{"-- 之后的重排也停止", []string{"my-app", "-n", "--", "-n", "-1"}, []string{"--no-color", "my-app", "--", "-n", "-1"}},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := NormalizeArgs(c.in); !slices.Equal(got, c.want) {
				t.Errorf("NormalizeArgs(%v) = %v, 想要 %v", c.in, got, c.want)
			}
		})
	}
}

func TestRewriteIndexArgs(t *testing.T) {
	cases := []struct {
		desc string
		in   []string
		want []string
	}{
		{"裸索引", []string{"-1"}, []string{"--recent=1"}},
		{"多位索引", []string{"-22"}, []string{"--recent=22"}},
		{"布尔选项后的裸索引", []string{"--no-color", "-1"}, []string{"--no-color", "--recent=1"}},
		{"索引在选项之前", []string{"-1", "--no-color"}, []string{"--recent=1", "--no-color"}},
		{"-- 之后的裸索引不改写", []string{"--", "-1"}, []string{"--", "-1"}},
		{"-- 前后的索引分开处理", []string{"-1", "--", "-2"}, []string{"--recent=1", "--", "-2"}},
		{"-- 之后的值也不改写", []string{"--", "--project", "-1"}, []string{"--", "--project", "-1"}},
		{"显式 --project 保持原样", []string{"--project=-1"}, []string{"--project=-1"}},
		{"--project 的独立值不能改写", []string{"--project", "-1"}, []string{"--project", "-1"}},
		{"--recent 的独立值不能改写", []string{"--recent", "-1"}, []string{"--recent", "-1"}},
		{"--indent 的独立值不能改写", []string{"--indent", "-5"}, []string{"--indent", "-5"}},
		{"--workspace 的独立值不能改写", []string{"--workspace", "-1"}, []string{"--workspace", "-1"}},
		{"单横线长选项的值也不能改写", []string{"-project", "-2"}, []string{"-project", "-2"}},
		{"普通项目名", []string{"my-app"}, []string{"my-app"}},
		{"项目名之后的索引也改写", []string{"my-app", "-1"}, []string{"my-app", "--recent=1"}},
		{"负号但非数字", []string{"-x"}, []string{"-x"}},
		{"空参数", nil, []string{}},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := RewriteIndexArgs(c.in); !slices.Equal(got, c.want) {
				t.Errorf("RewriteIndexArgs(%v) = %v, 想要 %v", c.in, got, c.want)
			}
		})
	}
}

func TestParseHelp(t *testing.T) {
	cases := [][]string{
		{"-h"},
		{"--help"},
		{"-nh"},
		// 帮助优先：冲突参数与越界值都不该挡住帮助。
		{"-h", "--project", "x", "--recent", "2"},
		{"-h", "--indent", "1"},
	}
	for _, args := range cases {
		o, err := parseForTest(t, args, config.DefaultConfig())
		if err != nil {
			t.Fatalf("解析 %v 失败：%v", args, err)
		}
		if !o.Help {
			t.Errorf("%v 应设置 Help", args)
		}
	}
}

func TestHelpOutput(t *testing.T) {
	var out bytes.Buffer
	writeHelp(&out)
	got := out.String()

	for _, want := range []string{
		"用法: gowork [选项] [项目]",
		"gowork init <SHELL>",
		"-p, --project <PROJECT>",
		"-N, --recent <N>",
		"-i, --indent <N>",
		"-w, --workspace <DIR>",
		"-n, --no-color",
		"-h, --help",
		"init <SHELL>",
		"powershell、zsh、sh、fish",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("帮助缺少 %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "cwd-file") {
		t.Errorf("内部选项 cwd-file 不该出现在帮助里\n%s", got)
	}

	// 所有选项的说明必须从同一列开始。左列保证是 ASCII，所以行内第一个
	// 非 ASCII 字节就是中文说明的起点。
	descCol := -1
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "  -") {
			continue
		}
		i := 0
		for i < len(line) && line[i] < 0x80 {
			i++
		}
		if descCol < 0 {
			descCol = i
			continue
		}
		if i != descCol {
			t.Errorf("选项说明应从第 %d 列开始，实际第 %d 列：%q", descCol, i, line)
		}
	}
}
