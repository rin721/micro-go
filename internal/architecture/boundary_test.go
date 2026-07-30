package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/rin721/micro-go"

func TestKernelAndCapabilityDoNotImportThirdPartyPackages(t *testing.T) {
	root := repositoryRoot(t)
	for _, directory := range []string{"kernel", "capability"} {
		walkGoFiles(t, filepath.Join(root, directory), func(path string, file *ast.File) {
			for _, item := range file.Imports {
				importPath, _ := strconv.Unquote(item.Path.Value)
				first, _, _ := strings.Cut(importPath, "/")
				if strings.Contains(first, ".") && !strings.HasPrefix(importPath, modulePath) {
					t.Errorf("%s imports third-party package %s", path, importPath)
				}
			}
		})
	}
}

func TestAdaptersDoNotExposeThirdPartyTypes(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, filepath.Join(root, "adapter"), func(path string, file *ast.File) {
		thirdPartyAliases := map[string]struct{}{}
		for _, item := range file.Imports {
			importPath, _ := strconv.Unquote(item.Path.Value)
			first, _, _ := strings.Cut(importPath, "/")
			if !strings.Contains(first, ".") || strings.HasPrefix(importPath, modulePath) {
				continue
			}
			alias := filepath.Base(importPath)
			if item.Name != nil {
				alias = item.Name.Name
			}
			thirdPartyAliases[alias] = struct{}{}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			function, ok := node.(*ast.FuncDecl)
			if !ok || !function.Name.IsExported() {
				return true
			}
			checkType := func(expr ast.Expr) {
				ast.Inspect(expr, func(current ast.Node) bool {
					selector, ok := current.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					identifier, ok := selector.X.(*ast.Ident)
					if ok {
						if _, forbidden := thirdPartyAliases[identifier.Name]; forbidden {
							t.Errorf("%s exported function %s exposes third-party type %s", path, function.Name.Name, identifier.Name)
						}
					}
					return true
				})
			}
			if function.Type.Params != nil {
				for _, field := range function.Type.Params.List {
					checkType(field.Type)
				}
			}
			if function.Type.Results != nil {
				for _, field := range function.Type.Results.List {
					checkType(field.Type)
				}
			}
			return false
		})
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate boundary test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func walkGoFiles(t *testing.T, root string, visit func(string, *ast.File)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		visit(path, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
