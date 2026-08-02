// 本文件使用 go/format 对全局配置选择的源码执行只读格式门禁。
package architecture

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepositoryGoFilesAreFormatted 替代脚本中的重复 gofmt 根目录配置。
func TestRepositoryGoFilesAreFormatted(t *testing.T) {
	root := repositoryRoot(t)
	for _, relativeRoot := range architecturePolicy.Source.ScanRoots {
		walkRoot := repositoryPath(root, relativeRoot)
		err := filepath.WalkDir(walkRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != walkRoot && excludedByGatePolicy(root, path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".go") || excludedByGatePolicy(root, path) {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			formatted, err := format.Source(source)
			if err != nil {
				return fmt.Errorf("format %s: %w", path, err)
			}
			if !bytes.Equal(source, formatted) {
				t.Errorf("%s requires gofmt", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
