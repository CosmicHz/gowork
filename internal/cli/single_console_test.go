package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSingleConsoleInstance 钉住“整个程序只有一个 Console 实例”：
// 除编排层 cli.go 外，非测试源码不得调用 console.New / console.NewWith，
// 也不得直接写 console.Console{}。测试文件可以自由构造注入流。
func TestSingleConsoleInstance(t *testing.T) {
	root := moduleRoot(t)
	allowed := filepath.Join("internal", "cli", "cli.go")

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// 跳过版本库与构建产物，以及所有隐藏目录。
			if name := d.Name(); name == "bin" || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == allowed {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, pos := range consoleConstructors(f) {
			offenders = append(offenders, fset.Position(pos).String())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描源码：%v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("只允许 cli.Main 构造 Console，以下位置另建了实例：\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestConsoleConstructorDetection 用一段合成源码确认扫描不是空转。
func TestConsoleConstructorDetection(t *testing.T) {
	src := `package probe

import "gowork/internal/console"

var _ = console.New()
var _ = console.NewWith(nil, nil, false, false)
var _ = console.Console{}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "probe.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := consoleConstructors(f); len(got) != 3 {
		t.Fatalf("应识别出 3 处构造，得到 %d", len(got))
	}
}

// consoleConstructors 返回文件里构造 Console 的位置：
// console.New / console.NewWith 调用，以及 console.Console{} 复合字面量。
func consoleConstructors(f *ast.File) []token.Pos {
	var pos []token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok && isConsolePkg(sel.X) &&
				(sel.Sel.Name == "New" || sel.Sel.Name == "NewWith") {
				pos = append(pos, node.Pos())
			}
		case *ast.CompositeLit:
			if sel, ok := node.Type.(*ast.SelectorExpr); ok && isConsolePkg(sel.X) &&
				sel.Sel.Name == "Console" {
				pos = append(pos, node.Pos())
			}
		}
		return true
	})
	return pos
}

func isConsolePkg(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "console"
}

// moduleRoot 从当前包目录向上找到含 go.mod 的仓库根目录。
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("找不到 go.mod，无法确定仓库根目录")
		}
		dir = parent
	}
}
