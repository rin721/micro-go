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
	values   map[reflect.Type]json.RawMessage
	hashes   map[reflect.Type][32]byte
}

// SnapshotEntry 用于由框架内部组装快照；Data 必须是独立副本。
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
	for _, entry := range entries {
		values[entry.Type] = append(json.RawMessage(nil), entry.Data...)
		hashes[entry.Type] = entry.Hash
	}
	return Snapshot{Version: version, LoadedAt: loadedAt, values: values, hashes: hashes}
}

// Value 返回配置值的深复制，调用者不能修改快照内部状态。
func Value[T any](snapshot Snapshot) (T, error) {
	var zero T
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	data, ok := snapshot.values[typeOf]
	if !ok {
		return zero, fmt.Errorf("configuration %s is not present in snapshot", typeOf)
	}
	if err := json.Unmarshal(data, &zero); err != nil {
		return zero, fmt.Errorf("decode configuration %s: %w", typeOf, err)
	}
	return zero, nil
}

// Hash 返回框架用于比较某个配置类型是否变化的摘要。
func (s Snapshot) Hash(typeOf reflect.Type) ([32]byte, bool) {
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
}

// Error 返回首个问题的稳定摘要，完整问题列表仍可通过 Issues 检查。
func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "configuration validation failed"
	}
	issue := e.Issues[0]
	return fmt.Sprintf("configuration validation failed at %s: %s", issue.Path, issue.Message)
}

// Validator 允许配置类型提供不依赖第三方标签的领域校验。
// Adapter 会把该错误转换为 ValidationIssue，与标签校验统一呈现。
type Validator interface {
	Validate() error
}
