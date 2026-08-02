// 本文件通过 Go AST 检查中文注释覆盖，并用内存源码样例验证门禁本身的判定规则。
package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// commentIssue 描述一个缺少中文说明的源码位置。
type commentIssue struct {
	// Line 是问题定义在当前文件中的起始行。
	Line int
	// Subject 是需要补充说明的文件、定义或字段名称。
	Subject string
}

// TestRepositoryOwnedGoCodeHasChineseComments 扫描范围内的生产、测试和示例 Go 文件。
func TestRepositoryOwnedGoCodeHasChineseComments(t *testing.T) {
	// 从当前测试文件反向定位仓库根，避免依赖调用命令时的工作目录。
	root := repositoryRoot(t)
	// 每个代码根独立遍历，任何读取或解析失败都直接终止当前测试。
	for _, relativeRoot := range architecturePolicy.Source.ScanRoots {
		walkRoot := repositoryPath(root, relativeRoot)
		err := filepath.WalkDir(walkRoot, func(path string, entry os.DirEntry, walkErr error) error {
			// 文件系统错误必须原样返回，否则门禁可能在漏扫目录时错误通过。
			if walkErr != nil {
				return walkErr
			}
			// 目录只负责继续遍历；基础忽略目录和受控隔离区在进入前整棵跳过。
			if entry.IsDir() {
				if path != walkRoot && excludedByGatePolicy(root, path) {
					return filepath.SkipDir
				}
				return nil
			}
			// 只有项目自有 Go 源码参与 AST 规则，README 等资产由各自语法检查负责。
			if !strings.EqualFold(filepath.Ext(path), ".go") || excludedByGatePolicy(root, path) {
				return nil
			}
			// 读取原始字节供 parser 保留注释位置；错误包含准确路径并交给 WalkDir 汇总。
			source, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			// 一次性报告文件中的全部缺口，减少维护者反复修复、重跑的轮次。
			for _, issue := range commentCoverageIssues(path, source) {
				t.Errorf("%s:%d %s 缺少相邻中文说明", path, issue.Line, issue.Subject)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestCommentCoverageRules 用正反例锁定文件、函数、字段和中文文本判定。
func TestCommentCoverageRules(t *testing.T) {
	// cases 让每种缺口都拥有独立名称和期望问题数，失败时可以直接定位规则漂移。
	cases := []struct {
		// name 是子测试名称。
		name string
		// source 是交给 Go parser 的完整内存源码。
		source string
		// wantIssues 是当前样例应产生的问题数量。
		wantIssues int
	}{
		{name: "complete", source: "// 本文件说明样例职责。\npackage sample\n\n// item 保存样例值。\ntype item struct {\n// name 标识样例。\nname string\n}\n\n// run 执行样例。\nfunc run() {}\n", wantIssues: 0},
		{name: "english-only-comments", source: "// This file explains the sample.\npackage sample\n\n// run executes the sample.\nfunc run() {}\n", wantIssues: 2},
		{name: "missing-file-comment", source: "package sample\n\n// run 执行样例。\nfunc run() {}\n", wantIssues: 1},
		{name: "missing-function-comment", source: "// 本文件说明样例职责。\npackage sample\n\nfunc run() {}\n", wantIssues: 1},
		{name: "missing-field-comment", source: "// 本文件说明样例职责。\npackage sample\n\n// item 保存样例值。\ntype item struct {\nname string\n}\n", wantIssues: 1},
	}
	// 每个样例独立解析，确保一个失败不会隐藏其他规则的结果。
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			issues := commentCoverageIssues("sample.go", []byte(test.source))
			if len(issues) != test.wantIssues {
				t.Fatalf("commentCoverageIssues() count = %d, want %d: %#v", len(issues), test.wantIssues, issues)
			}
		})
	}

	// 隔离边界单独验证，避免相似前缀目录被错误跳过。
	if !architecturePolicy.Excludes("_quarantine/password/pkg/adapter/password/crypto/crypto.go") {
		t.Fatal("quarantined Password path must be excluded")
	}
	if architecturePolicy.Excludes("_quarantine/password-other/crypto.go") {
		t.Fatal("similar path prefix must not be excluded")
	}
}

// commentCoverageIssues 解析单个 Go 文件并返回所有可附着文档的注释缺口。
func commentCoverageIssues(path string, source []byte) []commentIssue {
	// 独立 FileSet 让每个文件的行号从 1 开始，便于直接显示在测试错误中。
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, parser.ParseComments)
	if err != nil {
		// 语法错误不属于注释缺口，但必须以一个可见问题阻止门禁错误通过。
		return []commentIssue{{Line: 1, Subject: fmt.Sprintf("无法解析源码: %v", err)}}
	}

	// issues 按 AST 的源码顺序追加，输出顺序因此在不同平台保持稳定。
	issues := make([]commentIssue, 0)
	if !fileHasLeadingChineseComment(file) {
		issues = append(issues, commentIssue{Line: fileSet.Position(file.Package).Line, Subject: "文件职责"})
	}
	// 只检查顶层声明；函数内短变量由相邻逻辑块注释和人工 Diff 审阅保证可追踪性。
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			if !hasChineseComment(value.Doc) {
				issues = append(issues, commentIssue{Line: fileSet.Position(value.Pos()).Line, Subject: "函数或方法 " + value.Name.Name})
			}
		case *ast.GenDecl:
			// import 的用途按依赖组解释，不要求给每条标准库 import 制造机械 GoDoc。
			if value.Tok == token.IMPORT {
				continue
			}
			for _, specification := range value.Specs {
				issues = append(issues, specificationCommentIssues(fileSet, value.Doc, specification)...)
			}
		}
	}
	return issues
}

// specificationCommentIssues 检查 type、const、var 声明及其字段是否有中文说明。
func specificationCommentIssues(fileSet *token.FileSet, groupComment *ast.CommentGroup, specification ast.Spec) []commentIssue {
	// issues 仅包含当前 Spec 的问题，由调用方保持全文件顺序。
	issues := make([]commentIssue, 0)
	switch value := specification.(type) {
	case *ast.TypeSpec:
		if !hasChineseComment(value.Doc) && !hasChineseComment(groupComment) {
			issues = append(issues, commentIssue{Line: fileSet.Position(value.Pos()).Line, Subject: "类型 " + value.Name.Name})
		}
		// 只有 struct 和 interface 拥有需要逐项解释的字段列表。
		switch declared := value.Type.(type) {
		case *ast.StructType:
			issues = append(issues, fieldCommentIssues(fileSet, value.Name.Name, declared.Fields)...)
		case *ast.InterfaceType:
			issues = append(issues, fieldCommentIssues(fileSet, value.Name.Name, declared.Methods)...)
		}
	case *ast.ValueSpec:
		if hasChineseComment(value.Doc) || hasChineseComment(value.Comment) || hasChineseComment(groupComment) {
			return issues
		}
		for _, name := range value.Names {
			issues = append(issues, commentIssue{Line: fileSet.Position(name.Pos()).Line, Subject: "常量或变量 " + name.Name})
		}
	}
	return issues
}

// fieldCommentIssues 要求 struct 字段、嵌入字段和 interface 方法都有相邻中文解释。
func fieldCommentIssues(fileSet *token.FileSet, owner string, fields *ast.FieldList) []commentIssue {
	// 空字段列表不产生问题，例如无字段标记 struct。
	if fields == nil {
		return nil
	}
	issues := make([]commentIssue, 0)
	for _, field := range fields.List {
		if hasChineseComment(field.Doc) || hasChineseComment(field.Comment) {
			continue
		}
		// 未命名字段是嵌入类型或匿名接口项，使用表达式文本无法稳定还原时给出统一名称。
		fieldName := "嵌入字段"
		if len(field.Names) > 0 {
			fieldName = field.Names[0].Name
		}
		issues = append(issues, commentIssue{Line: fileSet.Position(field.Pos()).Line, Subject: "类型 " + owner + " 的字段 " + fieldName})
	}
	return issues
}

// fileHasLeadingChineseComment 判断 package 关键字前是否存在中文文件职责说明。
func fileHasLeadingChineseComment(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.End() >= file.Package {
			break
		}
		if hasChineseComment(group) {
			return true
		}
	}
	return false
}

// hasChineseComment 判断注释组是否至少包含一个汉字，英文标识符可以与中文并存。
func hasChineseComment(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	for _, character := range group.Text() {
		if unicode.Is(unicode.Han, character) {
			return true
		}
	}
	return false
}
