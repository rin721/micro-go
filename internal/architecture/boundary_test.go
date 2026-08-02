// Package architecture 通过源码结构测试守住脚手架的依赖方向和第三方隔离边界。
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
	"unicode"
)

// TestTypesContainOnlyContractsAndStandardLibrary 防止公共能力类型反向依赖实现或第三方库。
func TestTypesContainOnlyContractsAndStandardLibrary(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, repositoryPath(root, architecturePolicy.Source.TypesRoot), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if isThirdParty(importPath) {
				t.Errorf("%s imports third-party package %s", path, importPath)
			}
			if strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath+"/internal/") || strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath+"/pkg/") {
				t.Errorf("%s reversely imports implementation package %s", path, importPath)
			}
		}
	})
}

// TestInternalKernelDoesNotKnowConcreteAdapters 保证 Kernel 只拥有协议和值模型。
func TestInternalKernelDoesNotKnowConcreteAdapters(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, repositoryPath(root, architecturePolicy.Source.InternalKernelRoot), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if isThirdParty(importPath) {
				t.Errorf("%s imports third-party package %s", path, importPath)
			}
			if strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath+"/pkg/adapter/") || strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath+"/internal/adapter/") {
				t.Errorf("%s imports concrete adapter %s", path, importPath)
			}
		}
	})
}

// TestCapabilityAdaptersDoNotImportKernel 保证可复用能力实现不被应用生命周期协议污染。
func TestCapabilityAdaptersDoNotImportKernel(t *testing.T) {
	root := repositoryRoot(t)
	for _, directory := range architecturePolicy.Source.CapabilityAdapterRoots {
		walkGoFiles(t, repositoryPath(root, directory), func(path string, file *ast.File) {
			for _, item := range file.Imports {
				importPath := unquoteImport(item)
				if strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath+"/internal/kernel/") {
					t.Errorf("%s imports kernel protocol %s", path, importPath)
				}
			}
		})
	}
}

// TestKernelCapabilityDependencyIsLimitedToLogging 保证双阶段日志不会演变为 Kernel 任意依赖业务能力。
func TestKernelCapabilityDependencyIsLimitedToLogging(t *testing.T) {
	root := repositoryRoot(t)
	allowedRoots := make([]string, 0, len(architecturePolicy.Source.KernelCapabilityConsumerRoots))
	for _, relative := range architecturePolicy.Source.KernelCapabilityConsumerRoots {
		allowedRoots = append(allowedRoots, repositoryPath(root, relative))
	}
	walkGoFiles(t, repositoryPath(root, architecturePolicy.Source.KernelAdapterRoot), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if !strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath+"/types/capability/") {
				continue
			}
			allowedDirectory := false
			for _, allowedRoot := range allowedRoots {
				allowedDirectory = allowedDirectory || isWithinDirectory(allowedRoot, path)
			}
			if importPath != architecturePolicy.Source.KernelCapabilityImport || !allowedDirectory {
				t.Errorf("%s imports unsupported kernel capability %s", path, importPath)
			}
		}
	})
}

// TestDefaultBootstrapDoesNotImportPublicLoggingAdapters 保证默认基线只来自 Kernel Slog。
func TestDefaultBootstrapDoesNotImportPublicLoggingAdapters(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, repositoryPath(root, architecturePolicy.Source.BootstrapRoot), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if strings.HasPrefix(importPath, moduleImport(architecturePolicy.Source.PublicLoggingAdapterRoot)+"/") {
				t.Errorf("%s imports public logging adapter %s", path, importPath)
			}
		}
	})
}

// TestAdaptersDoNotExposeThirdPartyTypes 允许 Adapter 内部使用成熟库，但禁止第三方类型进入导出契约。
func TestAdaptersDoNotExposeThirdPartyTypes(t *testing.T) {
	root := repositoryRoot(t)
	for _, directory := range architecturePolicy.Source.ThirdPartyBoundaryRoots {
		walkGoFiles(t, repositoryPath(root, directory), checkExportedThirdPartyTypes(t))
	}
}

// TestCommandImportsOnlyBootstrap 把信号与退出码之外的装配责任收口到唯一组合根。
func TestCommandImportsOnlyBootstrap(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, repositoryPath(root, architecturePolicy.Source.CommandRoot), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath) && importPath != architecturePolicy.Source.CommandAllowedImport {
				t.Errorf("%s bypasses bootstrap through %s", path, importPath)
			}
		}
	})
}

// TestOnlyBootstrapSelectsBothAdapterFamilies 防止其他包形成隐蔽的第二组合根。
func TestOnlyBootstrapSelectsBothAdapterFamilies(t *testing.T) {
	root := repositoryRoot(t)
	type imports struct{ kernel, capability bool }
	byDirectory := map[string]imports{}
	walkGoFiles(t, root, func(path string, file *ast.File) {
		directory := filepath.Dir(path)
		value := byDirectory[directory]
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			kernelRoot := moduleImport(architecturePolicy.Source.KernelAdapterRoot)
			value.kernel = value.kernel || importPath == kernelRoot || strings.HasPrefix(importPath, kernelRoot+"/")
			value.capability = value.capability || importsConfiguredRoot(importPath, architecturePolicy.Source.CapabilityAdapterRoots)
		}
		byDirectory[directory] = value
	})
	bootstrap := repositoryPath(root, architecturePolicy.Source.BootstrapRoot)
	for directory, value := range byDirectory {
		if value.kernel && value.capability && directory != bootstrap {
			t.Errorf("%s selects both kernel and capability adapters", directory)
		}
	}
}

// TestLegacyArchitectureRootsAreRemoved 保证目录迁移是单轨替换而不是新旧实现并存。
func TestLegacyArchitectureRootsAreRemoved(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range architecturePolicy.Source.ForbiddenRoots {
		if _, err := os.Stat(repositoryPath(root, relative)); err == nil {
			t.Errorf("legacy or forbidden directory still exists: %s", relative)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

// TestExportedDeclarationsHaveChineseGoDoc 保证迁移后的自有契约仍能在源码旁解释职责和原因。
func TestExportedDeclarationsHaveChineseGoDoc(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, root, func(path string, file *ast.File) {
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if value.Name.IsExported() && receiverIsExported(value) {
					checkGoDoc(t, path, value.Name.Name, value.Doc)
				}
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					switch item := spec.(type) {
					case *ast.TypeSpec:
						if item.Name.IsExported() {
							comments := item.Doc
							if comments == nil {
								comments = value.Doc
							}
							checkGoDoc(t, path, item.Name.Name, comments)
						}
					case *ast.ValueSpec:
						comments := item.Doc
						if comments == nil {
							comments = value.Doc
						}
						for _, name := range item.Names {
							if name.IsExported() {
								checkGoDoc(t, path, name.Name, comments)
							}
						}
					}
				}
			}
		}
	})
}

// receiverIsExported 判断方法接收者的基础类型是否为导出类型。
func receiverIsExported(function *ast.FuncDecl) bool {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return true
	}
	receiver := function.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	identifier, ok := receiver.(*ast.Ident)
	return ok && identifier.IsExported()
}

// checkGoDoc 验证导出定义具有以符号名开头且包含中文的 GoDoc。
func checkGoDoc(t *testing.T, path, name string, comments *ast.CommentGroup) {
	t.Helper()
	if comments == nil {
		t.Errorf("%s exported declaration %s has no GoDoc", path, name)
		return
	}
	text := strings.TrimSpace(comments.Text())
	if !strings.HasPrefix(text, name) {
		t.Errorf("%s GoDoc for %s does not start with its name", path, name)
	}
	for _, character := range text {
		if unicode.Is(unicode.Han, character) {
			return
		}
	}
	t.Errorf("%s GoDoc for %s has no Chinese explanation", path, name)
}

// checkExportedThirdPartyTypes 返回检查导出签名是否暴露第三方类型的文件访问器。
func checkExportedThirdPartyTypes(t *testing.T) func(string, *ast.File) {
	t.Helper()
	return func(path string, file *ast.File) {
		thirdPartyAliases := map[string]struct{}{}
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if !isThirdParty(importPath) {
				continue
			}
			alias := filepath.Base(importPath)
			if item.Name != nil {
				alias = item.Name.Name
			}
			thirdPartyAliases[alias] = struct{}{}
		}
		check := func(owner string, expression ast.Expr) {
			ast.Inspect(expression, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				identifier, ok := selector.X.(*ast.Ident)
				if ok {
					if _, forbidden := thirdPartyAliases[identifier.Name]; forbidden {
						t.Errorf("%s exported declaration %s exposes third-party type %s", path, owner, identifier.Name)
					}
				}
				return true
			})
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if !value.Name.IsExported() {
					continue
				}
				if value.Type.Params != nil {
					for _, field := range value.Type.Params.List {
						check(value.Name.Name, field.Type)
					}
				}
				if value.Type.Results != nil {
					for _, field := range value.Type.Results.List {
						check(value.Name.Name, field.Type)
					}
				}
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Name.IsExported() {
						continue
					}
					structure, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						check(typeSpec.Name.Name, typeSpec.Type)
						continue
					}
					for _, field := range structure.Fields.List {
						exported := len(field.Names) == 0
						for _, name := range field.Names {
							exported = exported || name.IsExported()
						}
						if exported {
							check(typeSpec.Name.Name, field.Type)
						}
					}
				}
			}
		}
	}
}

// unquoteImport 去除 Go import 字面量引号，失败时返回空串供调用方报告。
func unquoteImport(item *ast.ImportSpec) string {
	value, err := strconv.Unquote(item.Path.Value)
	if err != nil {
		return item.Path.Value
	}
	return value
}

// isThirdParty 把不属于标准库和当前 Module 的路径判定为第三方依赖。
func isThirdParty(importPath string) bool {
	first := importPath
	if index := strings.IndexByte(importPath, '/'); index >= 0 {
		first = importPath[:index]
	}
	return strings.Contains(first, ".") && !strings.HasPrefix(importPath, architecturePolicy.Repository.ModulePath)
}

// repositoryRoot 从当前测试源码路径向上定位仓库根目录。
func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate boundary test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// walkGoFiles 解析指定目录下全部 Go 文件，并把带注释 AST 交给检查函数。
func walkGoFiles(t *testing.T, root string, visit func(string, *ast.File)) {
	t.Helper()
	repository := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && excludedByGatePolicy(repository, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// ParseComments 让同一 AST 既能执行依赖门禁，也能验证中文 GoDoc。
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
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
