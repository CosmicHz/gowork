package cli

import (
	"slices"
	"testing"
)

func TestRewriteIndexArgs(t *testing.T) {
	cases := []struct {
		desc string
		in   []string
		want []string
	}{
		{"裸索引", []string{"-1"}, []string{"--project=-1"}},
		{"多位索引", []string{"-22"}, []string{"--project=-22"}},
		{"布尔选项后的裸索引", []string{"--no-color", "-1"}, []string{"--no-color", "--project=-1"}},
		{"索引在选项之前", []string{"-1", "--no-color"}, []string{"--project=-1", "--no-color"}},
		{"等号形式本就是对的", []string{"--project=-1"}, []string{"--project=-1"}},
		{"--project 的独立值不能改写", []string{"--project", "-1"}, []string{"--project", "-1"}},
		{"--indent 的独立值不能改写", []string{"--indent", "-5"}, []string{"--indent", "-5"}},
		{"--workspace 的独立值不能改写", []string{"--workspace", "-1"}, []string{"--workspace", "-1"}},
		{"单横线长选项的值也不能改写", []string{"-project", "-2"}, []string{"-project", "-2"}},
		{"普通项目名", []string{"my-app"}, []string{"my-app"}},
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
