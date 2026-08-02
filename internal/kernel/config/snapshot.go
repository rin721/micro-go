// 本文件实现不可变配置快照、强类型读取和稳定校验错误模型。
package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// Snapshot 是一次完整配置加载产生的不可变事实。
type Snapshot struct {
	// Version 由 Application 单调分配，初始加载为 1。
	Version uint64
	// LoadedAt 是该完整快照成功生成的 UTC 时间。
	LoadedAt time.Time
	// values 按准确配置类型保存规范化 JSON，永不向调用方直接暴露。
	values map[reflect.Type]json.RawMessage
	// hashes 保存与 values 同内容的摘要，供 Reload 快速判断类型级变化。
	hashes map[reflect.Type][32]byte
}

// SnapshotEntry 用于由 Kernel Adapter 组装快照；Data 必须是独立副本。
type SnapshotEntry struct {
	// Type 是该强类型配置的 reflect.Type 键。
	Type reflect.Type
	// Data 是规范化 JSON 深复制。
	Data json.RawMessage
	// Hash 是 Data 的内容摘要，用于 Reload 变化判断。
	Hash [32]byte
}

// NewSnapshot 根据已验证的强类型配置创建快照。
func NewSnapshot(version uint64, loadedAt time.Time, entries []SnapshotEntry) Snapshot {
	// 再复制一次 RawMessage，切断 Loader 临时缓冲区与已发布 Snapshot 的所有权关系。
	values := make(map[reflect.Type]json.RawMessage, len(entries))
	hashes := make(map[reflect.Type][32]byte, len(entries))
	// 每个类型在一份快照中只有一个最终值，重复类型按调用方给出的最后条目覆盖。
	for _, entry := range entries {
		values[entry.Type] = append(json.RawMessage(nil), entry.Data...)
		hashes[entry.Type] = entry.Hash
	}
	// 两张 map 在构造后不再对外暴露，从而形成可并发读取的不可变值。
	return Snapshot{Version: version, LoadedAt: loadedAt, values: values, hashes: hashes}
}

// Value 返回配置值的深复制，调用者不能修改快照内部状态。
func Value[T any](snapshot Snapshot) (T, error) {
	// zero 同时充当解码目标和任意失败路径的类型正确零值。
	var zero T
	// 通过 *T 的 Elem 获取 T 本身，即使 T 是接口也不会创建运行期实例。
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	// 快照按准确类型索引，禁止把缺失配置误当成该类型的零值。
	data, ok := snapshot.values[typeOf]
	if !ok {
		return zero, fmt.Errorf("configuration %s is not present in snapshot", typeOf)
	}
	// JSON 是快照内部的规范传输形式；解码失败保留类型上下文和原始错误链。
	if err := json.Unmarshal(data, &zero); err != nil {
		return zero, fmt.Errorf("decode configuration %s: %w", typeOf, err)
	}
	// 返回新解码的值，调用方无法取得或修改内部 RawMessage。
	return zero, nil
}

// Hash 返回 Runtime 用于比较某个配置类型是否变化的摘要。
func (s Snapshot) Hash(typeOf reflect.Type) ([32]byte, bool) {
	// 同时返回存在标记，使全零摘要与“不存在”保持不同语义。
	value, ok := s.hashes[typeOf]
	return value, ok
}

// ValidationIssue 是第三方校验错误归一化后的稳定模型。
type ValidationIssue struct {
	// Path 指向失败字段；领域校验无法定位字段时可以为空。
	Path string
	// Rule 是失败的标签规则或项目 Validate 标识。
	Rule string
	// Message 是不依赖第三方错误类型的可读说明。
	Message string
}

// ValidationError 聚合项目自有的配置校验问题；具体 validator 错误只在内部适配器存在。
type ValidationError struct {
	// Issues 保存本次配置值的全部已知校验问题。
	Issues []ValidationIssue
	// causes 保存标签和领域校验的原始错误链。
	causes []error
}

// NewValidationError 创建同时保留稳定问题摘要和原始原因链的校验错误。
func NewValidationError(issues []ValidationIssue, causes ...error) *ValidationError {
	// 两个切片都复制底层数组，防止调用方在错误发布后改写诊断内容。
	return &ValidationError{Issues: append([]ValidationIssue(nil), issues...), causes: append([]error(nil), causes...)}
}

// Error 返回首个问题的稳定摘要，完整问题列表仍可通过 Issues 检查。
func (e *ValidationError) Error() string {
	// 没有字段级问题时仍返回稳定总述，原始原因可继续通过 Unwrap 读取。
	if len(e.Issues) == 0 {
		return "configuration validation failed"
	}
	// Error 文本保持简短，只展示首项；完整集合保留在 Issues 字段中。
	issue := e.Issues[0]
	return fmt.Sprintf("configuration validation failed at %s: %s", issue.Path, issue.Message)
}

// Unwrap 允许调用方使用 errors.Is/As 识别标签校验或领域校验的原始原因。
func (e *ValidationError) Unwrap() []error {
	// 返回副本，避免错误链的内部顺序被外部修改。
	return append([]error(nil), e.causes...)
}

// Validator 允许配置类型提供不依赖第三方标签的领域校验。
// Adapter 会把该错误转换为 ValidationIssue，与标签校验统一呈现，同时保留原因链。
type Validator interface {
	// Validate 检查标签无法表达的领域约束，并返回可保留在错误链中的原因。
	Validate() error
}
