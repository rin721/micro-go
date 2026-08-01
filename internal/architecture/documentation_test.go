package architecture

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	maxMarkdownLines      = 300
	maxPackageREADMELines = 80
)

var (
	markdownLinkPattern     = regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	markdownFencePattern    = regexp.MustCompile("(?s)```.*?```")
	markdownInlinePattern   = regexp.MustCompile("`[^`\\r\\n]*`")
	levelOneHeadingPattern  = regexp.MustCompile(`(?m)^# [^\r\n]+$`)
	numericDirectoryPattern = regexp.MustCompile(`^\d+(?:[-_]|$)`)
)

// TestDocumentationLocalLinksResolve 防止文档迁移后仍链接不存在的页面、源码或配置。
func TestDocumentationLocalLinksResolve(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range collectMarkdownFiles(t, root) {
		content := readDocumentationFile(t, path)
		for _, target := range localMarkdownTargets(path, content) {
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s has unresolved local link %q: %v", path, target, err)
			}
		}
	}
}

// TestDocumentationMarkdownShape 统一标题、篇幅和目录命名，避免重新产生巨型聚合页。
func TestDocumentationMarkdownShape(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range collectMarkdownFiles(t, root) {
		content := readDocumentationFile(t, path)
		prose := markdownFencePattern.ReplaceAll(content, nil)
		if headings := levelOneHeadingPattern.FindAll(prose, -1); len(headings) != 1 {
			t.Errorf("%s has %d level-one headings, want 1", path, len(headings))
		}
		if lines := markdownLineCount(content); lines > maxMarkdownLines {
			t.Errorf("%s has %d lines, limit is %d", path, lines, maxMarkdownLines)
		}
	}

	docsRoot := filepath.Join(root, "docs")
	err := filepath.WalkDir(docsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != docsRoot && numericDirectoryPattern.MatchString(entry.Name()) {
			t.Errorf("documentation directory uses a numeric prefix: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDocumentationSectionsAreIndexedAndReachable 保证双路线入口能够到达每个权威主题页。
func TestDocumentationSectionsAreIndexedAndReachable(t *testing.T) {
	root := repositoryRoot(t)
	docsRoot := filepath.Join(root, "docs")
	sections := []string{"getting-started", "development", "concepts", "maintenance", "reference", "decisions", "roadmap"}
	wantedSections := make(map[string]struct{}, len(sections))
	for _, section := range sections {
		wantedSections[section] = struct{}{}
		indexPath := filepath.Join(docsRoot, section, "README.md")
		indexContent := readDocumentationFile(t, indexPath)
		indexed := make(map[string]struct{})
		for _, target := range localMarkdownTargets(indexPath, indexContent) {
			indexed[filepath.Clean(target)] = struct{}{}
		}
		sectionRoot := filepath.Join(docsRoot, section)
		err := filepath.WalkDir(sectionRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || path == indexPath || !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			if _, ok := indexed[filepath.Clean(path)]; !ok {
				t.Errorf("section index %s does not link topic %s", indexPath, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(docsRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := wantedSections[entry.Name()]; !ok {
			t.Errorf("docs contains ungoverned top-level directory %s", entry.Name())
		}
	}

	docsIndex := filepath.Join(docsRoot, "README.md")
	reachable := reachableDocumentation(t, docsRoot, docsIndex)
	for _, path := range collectMarkdownFiles(t, docsRoot) {
		if _, ok := reachable[filepath.Clean(path)]; !ok {
			t.Errorf("documentation topic is unreachable from docs/README.md: %s", path)
		}
	}

	rootREADME := string(readDocumentationFile(t, filepath.Join(root, "README.md")))
	for _, target := range []string{"docs/README.md", "docs/development/README.md", "docs/maintenance/README.md"} {
		if !strings.Contains(rootREADME, "]("+target+")") {
			t.Errorf("root README does not link %s", target)
		}
	}
}

// TestDocumentationGoPackageREADMEsAreBoundaryCards 保证包级说明保持局部、简洁且结构一致。
func TestDocumentationGoPackageREADMEsAreBoundaryCards(t *testing.T) {
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
		if strings.EqualFold(filepath.Ext(path), ".go") {
			packages[filepath.Dir(path)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	requiredHeadings := []string{"## 职责", "## 边界与失败语义", "## 关键入口", "## 验证"}
	for directory := range packages {
		path := filepath.Join(directory, "README.md")
		content := readDocumentationFile(t, path)
		for _, heading := range requiredHeadings {
			if !strings.Contains(string(content), heading) {
				t.Errorf("package README %s is missing heading %q", path, heading)
			}
		}
		if lines := markdownLineCount(content); lines > maxPackageREADMELines {
			t.Errorf("package README %s has %d lines, limit is %d", path, lines, maxPackageREADMELines)
		}
	}
}

// TestDocumentationRetiredPathsAreAbsent 保证页面和 Kernel Adapter 迁移后不再形成双轨入口。
func TestDocumentationRetiredPathsAreAbsent(t *testing.T) {
	root := repositoryRoot(t)
	retiredPaths := []string{
		filepath.Join("docs", "getting-started", "first-application.md"),
		filepath.Join("docs", "development", "modules-and-providers.md"),
		filepath.Join("docs", "internals"),
		filepath.Join("docs", "reference", "api.md"),
		filepath.Join("pkg", "adapter", "kernel"),
	}
	for _, relative := range retiredPaths {
		if _, err := os.Stat(filepath.Join(root, relative)); err == nil {
			t.Errorf("retired path still exists: %s", relative)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	retiredReferences := []string{
		"getting-started/first-application.md",
		"development/modules-and-providers.md",
		"internals/adapters.md",
		"reference/api.md",
		"pkg/adapter/kernel",
	}
	for _, path := range collectMarkdownFiles(t, root) {
		content := string(readDocumentationFile(t, path))
		for _, reference := range retiredReferences {
			if strings.Contains(content, reference) {
				t.Errorf("%s still references retired path %q", path, reference)
			}
		}
	}
}

func collectMarkdownFiles(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
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
		if strings.EqualFold(filepath.Ext(path), ".md") {
			paths = append(paths, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func readDocumentationFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func localMarkdownTargets(path string, content []byte) []string {
	var targets []string
	withoutCode := markdownFencePattern.ReplaceAllString(string(content), "")
	withoutCode = markdownInlinePattern.ReplaceAllString(withoutCode, "")
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(withoutCode, -1) {
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
		targets = append(targets, filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target))))
	}
	return targets
}

func reachableDocumentation(t *testing.T, docsRoot, start string) map[string]struct{} {
	t.Helper()
	reachable := make(map[string]struct{})
	queue := []string{filepath.Clean(start)}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if _, ok := reachable[path]; ok {
			continue
		}
		reachable[path] = struct{}{}
		for _, target := range localMarkdownTargets(path, readDocumentationFile(t, path)) {
			if !isWithinDirectory(docsRoot, target) || !strings.EqualFold(filepath.Ext(target), ".md") {
				continue
			}
			if _, ok := reachable[target]; !ok {
				queue = append(queue, target)
			}
		}
	}
	return reachable
}

func isWithinDirectory(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func markdownLineCount(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	return strings.Count(string(content), "\n") + 1
}
