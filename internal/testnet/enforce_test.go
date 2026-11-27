package testnet

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryTestPackageBlocksNetwork 扫描仓库里的测试包，要求每个包都在 TestMain
// 里调用 testnet.Block，避免将来新增的包悄悄联网。
func TestEveryTestPackageBlocksNetwork(t *testing.T) {
	root := filepath.Join("..", "..")
	self := filepath.Join(root, "internal", "testnet")

	for _, base := range []string{"internal", "cmd"} {
		rootDir := filepath.Join(root, base)
		err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() || path == self {
				return nil
			}
			tests, err := filepath.Glob(filepath.Join(path, "*_test.go"))
			if err != nil || len(tests) == 0 {
				return err
			}
			for _, f := range tests {
				raw, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				if strings.Contains(string(raw), "testnet.Block(") {
					return nil
				}
			}
			t.Errorf("%s 有测试文件，但没有在 TestMain 里调用 testnet.Block", path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
