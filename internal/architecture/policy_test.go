// 本文件把强类型全局门禁配置转换为当前仓库的跨平台路径判断。
package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rin721/micro-go/types/testing/gateconfig"
)

// architecturePolicy 是本次测试进程只读使用的门禁策略快照。
var architecturePolicy = gateconfig.Current()

// TestGateConfigurationIsValid 保证任何源码或文档扫描前都能解释全局策略。
func TestGateConfigurationIsValid(t *testing.T) {
	if err := gateconfig.Validate(architecturePolicy); err != nil {
		t.Fatal(err)
	}
	root := repositoryRoot(t)
	for _, quarantine := range architecturePolicy.Quarantines {
		item, err := os.Stat(repositoryPath(root, quarantine.Root))
		if err != nil {
			t.Fatalf("quarantine %q is unavailable: %v", quarantine.Root, err)
		}
		if !item.IsDir() {
			t.Fatalf("quarantine %q is not a directory", quarantine.Root)
		}
		for _, originalRoot := range quarantine.OriginalRoots {
			if _, err := os.Stat(repositoryPath(root, originalRoot)); err == nil {
				t.Errorf("quarantined original root still exists: %s", originalRoot)
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}
}

// repositoryPath 把配置中的正斜杠仓库相对路径转换为当前平台绝对路径。
func repositoryPath(root, relative string) string {
	return filepath.Join(root, filepath.FromSlash(relative))
}

// moduleImport 把仓库相对包路径转换为当前 Module 的完整 import 前缀。
func moduleImport(relative string) string {
	return strings.TrimSuffix(architecturePolicy.Repository.ModulePath, "/") + "/" + strings.TrimPrefix(relative, "/")
}

// excludedByGatePolicy 判断绝对路径是否位于基础忽略目录或受控隔离区。
func excludedByGatePolicy(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return architecturePolicy.Excludes(filepath.ToSlash(relative))
}

// importsConfiguredRoot 判断 import 是否位于任一配置包根内。
func importsConfiguredRoot(importPath string, roots []string) bool {
	for _, root := range roots {
		prefix := moduleImport(root)
		if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
			return true
		}
	}
	return false
}
