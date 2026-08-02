// Package gateconfig 集中声明仓库质量门禁可调整的策略数据。
package gateconfig

import (
	"fmt"
	"path"
	"strings"
)

// Policy 是一次完整门禁执行使用的强类型策略快照。
type Policy struct {
	// Repository 保存 Module 身份和仓库遍历时忽略的基础目录。
	Repository RepositoryPolicy
	// Source 保存源码边界检查和格式检查需要的路径策略。
	Source SourcePolicy
	// Documentation 保存文档结构、篇幅和导航策略。
	Documentation DocumentationPolicy
	// Quarantines 保存暂不属于当前工程基线的受控隔离区。
	Quarantines []Quarantine
}

// RepositoryPolicy 保存跨门禁共享的仓库级事实。
type RepositoryPolicy struct {
	// ModulePath 是区分项目内 import 与第三方 import 的 Go Module 路径。
	ModulePath string
	// IgnoredRoots 是任何仓库遍历都不进入的非交付目录。
	IgnoredRoots []string
}

// SourcePolicy 保存源码门禁可调整的扫描范围和路径选择。
type SourcePolicy struct {
	// ScanRoots 是中文注释与只读格式检查覆盖的项目源码根。
	ScanRoots []string
	// TypesRoot 是只允许稳定契约和标准库依赖的类型根。
	TypesRoot string
	// InternalKernelRoot 是禁止依赖具体 Adapter 和第三方库的 Kernel 根。
	InternalKernelRoot string
	// KernelAdapterRoot 是允许有限 Capability 依赖的 Kernel Adapter 根。
	KernelAdapterRoot string
	// CapabilityAdapterRoots 是必须与 Kernel 协议隔离的业务 Adapter 根。
	CapabilityAdapterRoots []string
	// ThirdPartyBoundaryRoots 是禁止在导出签名中暴露第三方类型的 Adapter 根。
	ThirdPartyBoundaryRoots []string
	// KernelCapabilityImport 是 Kernel Adapter 唯一允许依赖的业务 Capability。
	KernelCapabilityImport string
	// KernelCapabilityConsumerRoots 是允许使用 KernelCapabilityImport 的准确目录。
	KernelCapabilityConsumerRoots []string
	// BootstrapRoot 是当前唯一组合根目录。
	BootstrapRoot string
	// PublicLoggingAdapterRoot 是默认 Bootstrap 不得选择的公共日志 Adapter 根。
	PublicLoggingAdapterRoot string
	// CommandRoot 是进程入口源码根。
	CommandRoot string
	// CommandAllowedImport 是 Command 唯一允许导入的项目包。
	CommandAllowedImport string
	// ForbiddenRoots 是单轨演进后不允许重新出现的旧目录或杂物目录。
	ForbiddenRoots []string
}

// DocumentationPolicy 保存文档门禁可调整的结构和篇幅策略。
type DocumentationPolicy struct {
	// Root 是权威主题文档根。
	Root string
	// AdapterRoot 是需要 README 与 usage 导航检查的业务 Adapter 根。
	AdapterRoot string
	// Sections 是文档中心允许的顶级语义分区。
	Sections []string
	// RootRequiredLinks 是根 README 必须提供的文档入口。
	RootRequiredLinks []string
	// PackageREADMEHeadings 是 Go 包 README 必须具有的章节。
	PackageREADMEHeadings []string
	// AdapterUsageHeadings 是具体 Adapter usage 页必须具有的章节。
	AdapterUsageHeadings []string
	// RetiredPaths 是不得重新出现的过期文档或包路径。
	RetiredPaths []string
	// RetiredReferences 是任何当前文档都不得引用的旧路径文本。
	RetiredReferences []string
	// MaxTopicLines 是普通 Markdown 主题页的最大物理行数。
	MaxTopicLines int
	// MaxPackageREADMELines 是 Go 包 README 的最大物理行数。
	MaxPackageREADMELines int
}

// Quarantine 描述一个由完整工具链和质量扫描共同忽略的受控隔离区。
type Quarantine struct {
	// Root 是仓库内以正斜杠表示的隔离根。
	Root string
	// OriginalRoots 保存代码进入隔离区前的原始目录。
	OriginalRoots []string
	// Owner 是负责决定恢复或删除的维护责任主体。
	Owner string
	// Reason 解释该目录为何不能作为当前实现参与门禁。
	Reason string
	// RestoreWhen 描述解除隔离必须满足的可验证条件。
	RestoreWhen string
}

// currentPolicy 是仓库当前唯一有效的门禁策略来源。
var currentPolicy = Policy{
	Repository: RepositoryPolicy{
		ModulePath:   "github.com/rin721/micro-go",
		IgnoredRoots: []string{".git", "tmp"},
	},
	Source: SourcePolicy{
		ScanRoots:                     []string{"cmd", "internal", "pkg", "types"},
		TypesRoot:                     "types",
		InternalKernelRoot:            "internal/kernel",
		KernelAdapterRoot:             "internal/adapter/kernel",
		CapabilityAdapterRoots:        []string{"pkg/adapter/clock", "pkg/adapter/idgen", "pkg/adapter/logging"},
		ThirdPartyBoundaryRoots:       []string{"pkg/adapter", "internal/adapter"},
		KernelCapabilityImport:        "github.com/rin721/micro-go/types/capability/logging",
		KernelCapabilityConsumerRoots: []string{"internal/adapter/kernel/logging", "internal/adapter/kernel/runtime"},
		BootstrapRoot:                 "internal/bootstrap",
		PublicLoggingAdapterRoot:      "pkg/adapter/logging",
		CommandRoot:                   "cmd/app",
		CommandAllowedImport:          "github.com/rin721/micro-go/internal/bootstrap",
		ForbiddenRoots: []string{
			"kernel", "capability", "adapter", "examples", "internal/config", "internal/di",
			"pkg/adapter/kernel", "pkg/utils",
		},
	},
	Documentation: DocumentationPolicy{
		Root:              "docs",
		AdapterRoot:       "pkg/adapter",
		Sections:          []string{"getting-started", "development", "concepts", "maintenance", "reference", "decisions", "roadmap"},
		RootRequiredLinks: []string{"docs/README.md", "docs/development/README.md", "docs/maintenance/README.md"},
		PackageREADMEHeadings: []string{
			"## 职责", "## 边界与失败语义", "## 关键入口", "## 验证",
		},
		AdapterUsageHeadings: []string{
			"## 适用场景", "## 接入方式", "## 配置与行为", "## 错误、并发与资源", "## 示例与验证",
		},
		RetiredPaths: []string{
			"docs/getting-started/first-application.md", "docs/development/modules-and-providers.md",
			"docs/internals", "docs/reference/api.md", "pkg/adapter/kernel", "pkg/adapter/logging/slog",
		},
		RetiredReferences: []string{
			"getting-started/first-application.md", "development/modules-and-providers.md",
			"internals/adapters.md", "reference/api.md", "pkg/adapter/kernel", "pkg/adapter/logging/slog",
		},
		MaxTopicLines:         2500,
		MaxPackageREADMELines: 1500,
	},
	Quarantines: []Quarantine{
		{
			Root:          "_quarantine/password",
			OriginalRoots: []string{"pkg/adapter/password", "types/capability/password"},
			Owner:         "repository maintainers",
			Reason:        "手工复制的 Password 契约与 Adapter 尚未适配当前边界、文档和 DI 装配",
			RestoreWhen:   "完成契约与 Adapter 单轨改造、Module 接入、消费者验证并通过完整门禁",
		},
	},
}

// Current 返回与全局策略不共享切片底层数组的只读工作快照。
func Current() Policy {
	result := currentPolicy
	result.Repository.IgnoredRoots = cloneStrings(currentPolicy.Repository.IgnoredRoots)
	result.Source.ScanRoots = cloneStrings(currentPolicy.Source.ScanRoots)
	result.Source.CapabilityAdapterRoots = cloneStrings(currentPolicy.Source.CapabilityAdapterRoots)
	result.Source.ThirdPartyBoundaryRoots = cloneStrings(currentPolicy.Source.ThirdPartyBoundaryRoots)
	result.Source.KernelCapabilityConsumerRoots = cloneStrings(currentPolicy.Source.KernelCapabilityConsumerRoots)
	result.Source.ForbiddenRoots = cloneStrings(currentPolicy.Source.ForbiddenRoots)
	result.Documentation.Sections = cloneStrings(currentPolicy.Documentation.Sections)
	result.Documentation.RootRequiredLinks = cloneStrings(currentPolicy.Documentation.RootRequiredLinks)
	result.Documentation.PackageREADMEHeadings = cloneStrings(currentPolicy.Documentation.PackageREADMEHeadings)
	result.Documentation.AdapterUsageHeadings = cloneStrings(currentPolicy.Documentation.AdapterUsageHeadings)
	result.Documentation.RetiredPaths = cloneStrings(currentPolicy.Documentation.RetiredPaths)
	result.Documentation.RetiredReferences = cloneStrings(currentPolicy.Documentation.RetiredReferences)
	result.Quarantines = make([]Quarantine, len(currentPolicy.Quarantines))
	for index, item := range currentPolicy.Quarantines {
		result.Quarantines[index] = item
		result.Quarantines[index].OriginalRoots = cloneStrings(item.OriginalRoots)
	}
	return result
}

// Validate 拒绝无法跨平台解释或缺少治理信息的门禁策略。
func Validate(policy Policy) error {
	if strings.TrimSpace(policy.Repository.ModulePath) == "" {
		return fmt.Errorf("repository module path is empty")
	}
	if policy.Documentation.MaxTopicLines <= 0 || policy.Documentation.MaxPackageREADMELines <= 0 {
		return fmt.Errorf("documentation line limits must be positive")
	}
	pathGroups := [][]string{
		policy.Repository.IgnoredRoots, policy.Source.ScanRoots, {policy.Source.TypesRoot},
		{policy.Source.InternalKernelRoot}, {policy.Source.KernelAdapterRoot}, policy.Source.CapabilityAdapterRoots,
		policy.Source.ThirdPartyBoundaryRoots, policy.Source.KernelCapabilityConsumerRoots,
		{policy.Source.BootstrapRoot}, {policy.Source.PublicLoggingAdapterRoot}, {policy.Source.CommandRoot},
		policy.Source.ForbiddenRoots, {policy.Documentation.Root}, {policy.Documentation.AdapterRoot},
		policy.Documentation.Sections, policy.Documentation.RootRequiredLinks, policy.Documentation.RetiredPaths,
	}
	for _, values := range pathGroups {
		if err := validatePaths(values); err != nil {
			return err
		}
	}
	if err := validateStrings(policy.Documentation.PackageREADMEHeadings); err != nil {
		return fmt.Errorf("package README headings: %w", err)
	}
	if err := validateStrings(policy.Documentation.AdapterUsageHeadings); err != nil {
		return fmt.Errorf("adapter usage headings: %w", err)
	}
	if err := validateStrings(policy.Documentation.RetiredReferences); err != nil {
		return fmt.Errorf("retired references: %w", err)
	}
	imports := []struct {
		// name 是错误中标识配置项的稳定名称。
		name string
		// value 是必须位于当前 Module 下的完整 import。
		value string
	}{
		{name: "kernel capability import", value: policy.Source.KernelCapabilityImport},
		{name: "command allowed import", value: policy.Source.CommandAllowedImport},
	}
	for _, item := range imports {
		if item.value == "" || (item.value != policy.Repository.ModulePath && !strings.HasPrefix(item.value, policy.Repository.ModulePath+"/")) {
			return fmt.Errorf("%s %q is outside module %q", item.name, item.value, policy.Repository.ModulePath)
		}
	}
	quarantineRoots := make(map[string]struct{}, len(policy.Quarantines))
	originalRoots := make(map[string]struct{})
	for _, quarantine := range policy.Quarantines {
		if err := validatePaths([]string{quarantine.Root}); err != nil {
			return fmt.Errorf("quarantine root: %w", err)
		}
		firstSegment := strings.SplitN(quarantine.Root, "/", 2)[0]
		if !strings.HasPrefix(firstSegment, "_") && !strings.HasPrefix(firstSegment, ".") {
			return fmt.Errorf("quarantine root %q is not ignored by the Go tool", quarantine.Root)
		}
		if _, exists := quarantineRoots[quarantine.Root]; exists {
			return fmt.Errorf("quarantine root %q is duplicated", quarantine.Root)
		}
		quarantineRoots[quarantine.Root] = struct{}{}
		if err := validatePaths(quarantine.OriginalRoots); err != nil {
			return fmt.Errorf("quarantine %q original roots: %w", quarantine.Root, err)
		}
		for _, originalRoot := range quarantine.OriginalRoots {
			if _, exists := originalRoots[originalRoot]; exists {
				return fmt.Errorf("quarantine original root %q is duplicated", originalRoot)
			}
			originalRoots[originalRoot] = struct{}{}
		}
		if len(quarantine.OriginalRoots) == 0 || strings.TrimSpace(quarantine.Owner) == "" || strings.TrimSpace(quarantine.Reason) == "" || strings.TrimSpace(quarantine.RestoreWhen) == "" {
			return fmt.Errorf("quarantine %q has incomplete governance metadata", quarantine.Root)
		}
	}
	return nil
}

// validateStrings 验证策略文本集合非空且没有重复项。
func validateStrings(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("policy text is empty")
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("policy text %q is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

// Excludes 判断仓库相对路径是否落在基础忽略目录或受控隔离区内。
func (policy Policy) Excludes(relativePath string) bool {
	cleaned := path.Clean(strings.ReplaceAll(relativePath, "\\", "/"))
	for _, root := range policy.Repository.IgnoredRoots {
		if withinPath(root, cleaned) {
			return true
		}
	}
	for _, quarantine := range policy.Quarantines {
		if withinPath(quarantine.Root, cleaned) {
			return true
		}
	}
	return false
}

// validatePaths 验证一组仓库路径采用唯一、规范的正斜杠相对表示。
func validatePaths(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
			return fmt.Errorf("repository path %q is not a normalized relative slash path", value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("repository path %q is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

// withinPath 使用目录边界匹配路径，避免把 password-other 误判为 password 子树。
func withinPath(root, candidate string) bool {
	cleanRoot := path.Clean(root)
	return candidate == cleanRoot || strings.HasPrefix(candidate, cleanRoot+"/")
}

// cloneStrings 复制字符串切片，nil 输入保持 nil 语义。
func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}
