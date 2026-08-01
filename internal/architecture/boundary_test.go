// Package architecture 通过源码结构测试守住脚手架的依赖方向和第三方隔离边界。
package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

const modulePath = "github.com/rin721/micro-go"

// TestTypesContainOnlyContractsAndStandardLibrary 防止公共能力类型反向依赖实现或第三方库。
func TestTypesContainOnlyContractsAndStandardLibrary(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, filepath.Join(root, "types"), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if isThirdParty(importPath) {
				t.Errorf("%s imports third-party package %s", path, importPath)
			}
			if strings.HasPrefix(importPath, modulePath+"/internal/") || strings.HasPrefix(importPath, modulePath+"/pkg/") {
				t.Errorf("%s reversely imports implementation package %s", path, importPath)
			}
		}
	})
}

// TestInternalKernelDoesNotKnowConcreteAdapters 保证 Kernel 只拥有协议和值模型。
func TestInternalKernelDoesNotKnowConcreteAdapters(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, filepath.Join(root, "internal", "kernel"), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if isThirdParty(importPath) {
				t.Errorf("%s imports third-party package %s", path, importPath)
			}
			if strings.HasPrefix(importPath, modulePath+"/pkg/adapter/") || strings.HasPrefix(importPath, modulePath+"/internal/adapter/") {
				t.Errorf("%s imports concrete adapter %s", path, importPath)
			}
		}
	})
}

// TestCapabilityAdaptersDoNotImportKernel 保证可复用能力实现不被应用生命周期协议污染。
func TestCapabilityAdaptersDoNotImportKernel(t *testing.T) {
	root := repositoryRoot(t)
	for _, directory := range []string{"clock", "idgen", "logging"} {
		walkGoFiles(t, filepath.Join(root, "pkg", "adapter", directory), func(path string, file *ast.File) {
			for _, item := range file.Imports {
				importPath := unquoteImport(item)
				if strings.HasPrefix(importPath, modulePath+"/internal/kernel/") {
					t.Errorf("%s imports kernel protocol %s", path, importPath)
				}
			}
		})
	}
}

// TestAdaptersDoNotExposeThirdPartyTypes 允许 Adapter 内部使用成熟库，但禁止第三方类型进入导出契约。
func TestAdaptersDoNotExposeThirdPartyTypes(t *testing.T) {
	root := repositoryRoot(t)
	for _, directory := range []string{filepath.Join(root, "pkg", "adapter"), filepath.Join(root, "internal", "adapter")} {
		walkGoFiles(t, directory, checkExportedThirdPartyTypes(t))
	}
}

// TestCommandImportsOnlyBootstrap 把信号与退出码之外的装配责任收口到唯一组合根。
func TestCommandImportsOnlyBootstrap(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, filepath.Join(root, "cmd", "app"), func(path string, file *ast.File) {
		for _, item := range file.Imports {
			importPath := unquoteImport(item)
			if strings.HasPrefix(importPath, modulePath) && importPath != modulePath+"/internal/bootstrap" {
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
			value.kernel = value.kernel || strings.HasPrefix(importPath, modulePath+"/internal/adapter/kernel/")
			value.capability = value.capability || strings.HasPrefix(importPath, modulePath+"/pkg/adapter/clock/") || strings.HasPrefix(importPath, modulePath+"/pkg/adapter/idgen/") || strings.HasPrefix(importPath, modulePath+"/pkg/adapter/logging/")
		}
		byDirectory[directory] = value
	})
	bootstrap := filepath.Join(root, "internal", "bootstrap")
	for directory, value := range byDirectory {
		if value.kernel && value.capability && directory != bootstrap {
			t.Errorf("%s selects both kernel and capability adapters", directory)
		}
	}
}

// TestLegacyArchitectureRootsAreRemoved 保证目录迁移是单轨替换而不是新旧实现并存。
func TestLegacyArchitectureRootsAreRemoved(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range []string{"kernel", "capability", "adapter", "examples", filepath.Join("internal", "config"), filepath.Join("internal", "di"), filepath.Join("pkg", "adapter", "kernel"), filepath.Join("pkg", "utils")} {
		if _, err := os.Stat(filepath.Join(root, relative)); err == nil {
			t.Errorf("legacy or forbidden directory still exists: %s", relative)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

var markdownLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)

// TestLocalMarkdownLinksResolve 防止目录迁移后文档仍指向已经删除的源码或页面。
func TestLocalMarkdownLinksResolve(t *testing.T) {
	root := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(content), -1) {
			target := strings.Trim(strings.TrimSpace(match[1]), "<>")
			if index := strings.IndexAny(target, " \t"); index >= 0 {
				target = target[:index]
			}
			if index := strings.IndexByte(target, '#'); index >= 0 {
				target = target[:index]
			}
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s has unresolved local link %q: %v", path, match[1], err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGoPackageDirectoriesHaveREADME 保证源码局部设计说明与包目录一起迁移。
func TestGoPackageDirectoriesHaveREADME(t *testing.T) {
	root := repositoryRoot(t)
	packages := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			packages[filepath.Dir(path)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for directory := range packages {
		if _, err := os.Stat(filepath.Join(directory, "README.md")); err != nil {
			t.Errorf("Go package directory %s has no README.md", directory)
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

func unquoteImport(item *ast.ImportSpec) string {
	value, err := strconv.Unquote(item.Path.Value)
	if err != nil {
		return item.Path.Value
	}
	return value
}

func isThirdParty(importPath string) bool {
	first := importPath
	if index := strings.IndexByte(importPath, '/'); index >= 0 {
		first = importPath[:index]
	}
	return strings.Contains(first, ".") && !strings.HasPrefix(importPath, modulePath)
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
