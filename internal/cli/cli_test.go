package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"gowork/internal/config"
	"gowork/internal/project"
)

func parseForTest(t *testing.T, args []string, cfg config.Config) (Options, error) {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	return parseWith(fs, args, cfg)
}

// testFlagSet 返回只用于读取选项元数据的 FlagSet；绑定变量不参与断言。
func testFlagSet(t *testing.T, cfg config.Config) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	o := Options{Config: cfg}
	registerFlags(fs, &o, cfg)
	return fs
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
		t.Errorf("位置参数应成为项目名：%q", o.Project)
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
		{"短选项值恰好是 --", []string{"-w", "--", "-1"}, "-1", false},
		{"终止符后的长选项写法", []string{"--", "--no-color"}, "--no-color", true},
		{"项目名之后的选项不改变字面判定", []string{"my-app", "-n"}, "my-app", false},
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

func TestParseRejectsSingleDashLongOption(t *testing.T) {
	if _, err := parseForTest(t, []string{"-no-color"}, config.DefaultConfig()); err == nil {
		t.Fatal("单横线只表示短选项，-no-color 应报错")
	}
}

func TestParseShorthandForms(t *testing.T) {
	cases := []struct {
		desc  string
		args  []string
		check func(Options) bool
	}{
		{"独立值", []string{"-p", "my-app"}, func(o Options) bool { return o.Project == "my-app" }},
		{"贴值", []string{"-pmy-app"}, func(o Options) bool { return o.Project == "my-app" }},
		{"负值", []string{"-p-1"}, func(o Options) bool { return o.Project == "-1" }},
		{"等号", []string{"-p=foo"}, func(o Options) bool { return o.Project == "foo" }},
		{"布尔", []string{"-n"}, func(o Options) bool { return o.Config.NoColor }},
		{"组合独立值", []string{"-ni", "5"}, func(o Options) bool { return o.Config.NoColor && o.Config.Indent == 5 }},
		{"组合贴值", []string{"-ni5"}, func(o Options) bool { return o.Config.NoColor && o.Config.Indent == 5 }},
		{"布尔赋值", []string{"-n=false"}, func(o Options) bool { return !o.Config.NoColor }},
		{"帮助", []string{"-nh"}, func(o Options) bool { return o.Help }},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.NoColor = true // 让 -n=false 能证明自己生效
			o, err := parseForTest(t, c.args, cfg)
			if err != nil {
				t.Fatalf("解析失败：%v", err)
			}
			if !c.check(o) {
				t.Errorf("%v 未按预期解析：%+v", c.args, o)
			}
		})
	}
}

func TestShorthandValueAfterProject(t *testing.T) {
	o, err := parseForTest(t, []string{"my-app", "-i", "5"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if o.Project != "my-app" || o.Config.Indent != 5 {
		t.Errorf("位置参数后的短选项应生效：project=%q indent=%d", o.Project, o.Config.Indent)
	}
}

func TestOptionToken(t *testing.T) {
	fs := testFlagSet(t, config.DefaultConfig())
	cases := []struct {
		arg      string
		spelling string
		names    []string
		ok       bool
	}{
		{"-n", "-n", []string{"no-color"}, true},
		{"-n=false", "-n", []string{"no-color"}, true},
		{"--no-color", "--no-color", []string{"no-color"}, true},
		{"--no-color=false", "--no-color", []string{"no-color"}, true},
		{"-nh", "-nh", []string{"no-color", "help"}, true},
		{"--project", "--project", []string{"project"}, true},
		{"--project=foo", "--project", []string{"project"}, true},
		{"--recent", "--recent", []string{"recent"}, true},
		{"--recent=2", "--recent", []string{"recent"}, true},
		{"-p", "-p", []string{"project"}, true},
		{"-pfoo", "-p", []string{"project"}, true},
		{"-p-1", "-p", []string{"project"}, true},
		{"-ni", "-ni", []string{"no-color", "indent"}, true},
		{"-ni5", "-ni", []string{"no-color", "indent"}, true},
		{"-c", "-c", []string{"cwd-file"}, true},
		{"-1", "-1", []string{"index1"}, true},
		{"-9", "-9", []string{"index9"}, true},
		{"-n1", "-n1", []string{"no-color", "index1"}, true},
		{"-no-color", "", nil, false},
		{"my-app", "", nil, false},
		{"--", "", nil, false},
		{"-", "", nil, false},
		{"-x", "", nil, false},
		{"--unknown", "", nil, false},
	}

	for _, c := range cases {
		t.Run(c.arg, func(t *testing.T) {
			spelling, names, ok := optionToken(fs, c.arg)
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
	fs := testFlagSet(t, config.DefaultConfig())
	projects := []project.Project{
		{Name: "-n"},
		{Name: "--no-color"},
		{Name: "--project"},
		{Name: "--recent"},
		{Name: "-p"},
		{Name: "-ni"},
		{Name: "-c"},
		{Name: "-i"},
		{Name: "-1"},
		{Name: "-n1"},
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
		{"长选项等号", []string{"--project=foo"}, map[string]bool{"project": true}, []string{"--project"}},
		{"recent 长选项", []string{"--recent", "2"}, map[string]bool{"recent": true}, []string{"--recent"}},
		{"短选项贴值", []string{"-pfoo"}, map[string]bool{"project": true}, []string{"-p"}},
		{"短选项等号", []string{"-p=foo"}, map[string]bool{"project": true}, []string{"-p"}},
		{"短选项组合", []string{"-ni", "5"}, map[string]bool{"no-color": true, "indent": true}, []string{"-ni"}},
		{"重复选项去重", []string{"-n", "-n"}, map[string]bool{"no-color": true}, []string{"-n"}},
		{"cwd-file 不提示", []string{"-c", "x"}, map[string]bool{"cwd-file": true}, nil},
		{"位置参数后的未使用选项", []string{"my-app", "-n"}, map[string]bool{}, nil},
		{"未使用的短选项", []string{"-i", "5"}, map[string]bool{}, nil},
		{"位置参数后同名选项仍提示", []string{"-n", "my-app", "--no-color"}, map[string]bool{"no-color": true}, []string{"-n", "--no-color"}},
		{"-- 后同名选项不提示", []string{"-n", "--", "--no-color"}, map[string]bool{"no-color": true}, []string{"-n"}},
		{"作为选项值的 token 不提示", []string{"-p", "--no-color"}, map[string]bool{"project": true, "no-color": true}, []string{"-p"}},
		{"组合选项的值不提示", []string{"-ni", "--no-color"}, map[string]bool{"no-color": true, "indent": true}, []string{"-ni"}},
		{"单横线不再算长选项", []string{"-no-color"}, map[string]bool{"no-color": true}, nil},
		{"索引交由 Resolve 提示", []string{"-1"}, map[string]bool{"index1": true, "no-color": true}, nil},
		{"索引在组合里", []string{"-n1"}, map[string]bool{"no-color": true, "index1": true}, []string{"-n1"}},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			warnings := optionCollisionWarnings(fs, c.args, c.used, projects)
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

func TestOptionsAfterProjectAreParsed(t *testing.T) {
	o, err := parseForTest(t, []string{"my-app", "-n"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if !o.Config.NoColor {
		t.Fatal("位置参数之后的 -n 应生效")
	}
	if o.Project != "my-app" {
		t.Errorf("项目名不应被选项解析改变：%q", o.Project)
	}
}

func TestParseRejectsUnknownOptionAfterProject(t *testing.T) {
	if _, err := parseForTest(t, []string{"my-app", "--unknown"}, config.DefaultConfig()); err == nil {
		t.Fatal("位置参数之后的未知选项应报错，而不是静默忽略")
	}
}

func TestParseRejectsMultipleProjects(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"两个位置参数", []string{"my-app", "other"}, "other"},
		{"三个位置参数", []string{"my-app", "other", "third"}, "other、third"},
		{"终止符之后", []string{"--", "-1", "-2"}, "-2"},
		{"终止符之前之后", []string{"my-app", "--", "other"}, "other"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseForTest(t, c.args, config.DefaultConfig())
			if err == nil {
				t.Fatal("多个位置参数应报错")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应列出多写的项目 %q：%v", c.want, err)
			}
		})
	}

	// 单个位置参数不受影响。
	o, err := parseForTest(t, []string{"my-app"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("单个位置参数不该报错：%v", err)
	}
	if o.Project != "my-app" {
		t.Errorf("项目名 = %q，想要 my-app", o.Project)
	}

	// 帮助优先于位置参数校验。
	o, err = parseForTest(t, []string{"-h", "my-app", "other"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("带 -h 时不该报错：%v", err)
	}
	if !o.Help {
		t.Error("应设置 Help")
	}
}

func TestParseRejectsProjectWithTargetFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"位置参数之后 --recent", []string{"my-app", "--recent", "2"}, "项目名 my-app 和 --recent"},
		{"位置参数之后 --project", []string{"my-app", "--project", "other"}, "项目名 my-app 和 --project"},
		{"位置参数之后 -1", []string{"my-app", "-1"}, "项目名 my-app 和 索引 -1"},
		{"位置参数之前 --recent", []string{"--recent", "2", "my-app"}, "项目名 my-app 和 --recent"},
		{"位置参数之前 --project", []string{"--project", "other", "my-app"}, "项目名 my-app 和 --project"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseForTest(t, c.args, config.DefaultConfig())
			if err == nil {
				t.Fatal("位置参数与其它目标选择器同时出现应报错")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应说明用哪两种方式指定了项目（%q）：%v", c.want, err)
			}
		})
	}
}

func TestParseRejectsTwoTargetFlags(t *testing.T) {
	_, err := parseForTest(t, []string{"--project", "foo", "--recent", "2"}, config.DefaultConfig())
	if err == nil {
		t.Fatal("--project 与 --recent 同时出现应报错")
	}
	if want := "同时用了 --project 和 --recent"; !strings.Contains(err.Error(), want) {
		t.Errorf("错误信息应包含 %q：%v", want, err)
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

	// 位置参数之后的 -n 由 pflag 的 interspersed 解析。
	o, err = parseForTest(t, []string{"my-app", "-n"}, config.DefaultConfig())
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if !o.UsedFlags["no-color"] {
		t.Errorf("位置参数之后的选项应算作已使用：%v", o.UsedFlags)
	}
	if o.Project != "my-app" {
		t.Errorf("项目名不应被选项解析改变：%q", o.Project)
	}
}

func TestParseIndexShorthand(t *testing.T) {
	cases := []struct {
		desc    string
		args    []string
		project string
		index   int
	}{
		{"第一个", []string{"-1"}, "-1", 1},
		{"第九个", []string{"-9"}, "-9", 9},
		{"与布尔组合", []string{"-n1"}, "-1", 1},
		{"位置参数之后", []string{"my-app", "-1"}, "", 0},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			o, err := parseForTest(t, c.args, config.DefaultConfig())
			if c.project == "" {
				// 位置参数与索引同时出现属于冲突，单独在上面的用例里覆盖。
				if err == nil {
					t.Fatal("位置参数与索引同时出现应报错")
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败：%v", err)
			}
			if o.Project != c.project {
				t.Errorf("Project = %q，想要 %q", o.Project, c.project)
			}
			if !o.index[c.index] {
				t.Errorf("应记录索引开关 -%d：%v", c.index, o.UsedFlags)
			}
		})
	}

	// 索引短选项不能带值。
	if _, err := parseForTest(t, []string{"-1=5"}, config.DefaultConfig()); err == nil {
		t.Fatal("-1=5 应报错")
	}
	// 两个索引短选项同时出现应报错。
	if _, err := parseForTest(t, []string{"-1", "-2"}, config.DefaultConfig()); err == nil {
		t.Fatal("-1 与 -2 同时出现应报错")
	} else if !strings.Contains(err.Error(), "索引 -1") || !strings.Contains(err.Error(), "索引 -2") {
		t.Errorf("错误信息应说明用了哪两个索引：%v", err)
	}
}

func TestCheckIndexArgs(t *testing.T) {
	fs := testFlagSet(t, config.DefaultConfig())
	cases := []struct {
		desc    string
		args    []string
		wantErr string
	}{
		{"单个索引可以", []string{"-1"}, ""},
		{"九个索引可以", []string{"-9"}, ""},
		{"两位数报错", []string{"-12"}, "可能是笔误"},
		{"十报错", []string{"-10"}, "请写 --recent 10"},
		{"零报错", []string{"-0"}, "从 1 开始"},
		{"双横线两位数", []string{"--12"}, "请写 --recent 12"},
		{"双横线零", []string{"--0"}, "从 1 开始"},
		{"赋值报错", []string{"-1=5"}, "不接受取值"},
		{"组合里的赋值报错", []string{"-n1=5"}, "索引 -1 不接受取值"},
		{"-- 之后不检查", []string{"--", "-12"}, ""},
		{"选项的值不检查", []string{"-p", "-12"}, ""},
		{"组合选项的值不检查", []string{"-ni", "-12"}, ""},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := checkIndexArgs(fs, c.args)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("不该报错：%v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("想要错误，得到 nil")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("错误信息应包含 %q：%v", c.wantErr, err)
			}
		})
	}
}

func TestMaxIndexShorthand(t *testing.T) {
	if maxIndexShorthand < 1 || maxIndexShorthand > 9 {
		t.Fatalf("maxIndexShorthand = %d，必须在 1..9 之间：索引从 1 开始，且 pflag 的 shorthand 只能是单个字符",
			maxIndexShorthand)
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
		"-1..-9, --recent <N>",
		"-i, --indent <N>",
		"-w, --workspace <DIR>",
		"-n, --no-color",
		"-h, --help",
		"init <SHELL>",
		"powershell、zsh、sh、fish",
		"位置参数只能有一个",
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
