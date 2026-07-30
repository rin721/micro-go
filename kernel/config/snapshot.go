package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// Snapshot 是一次完整配置加载产生的不可变事实。
type Snapshot struct {
	Version  uint64
	LoadedAt time.Time
	values   map[reflect.Type]json.RawMessage
	hashes   map[reflect.Type][32]byte
}

// SnapshotEntry 用于由框架内部组装快照；Data 必须是独立副本。
type SnapshotEntry struct {
	Type reflect.Type
	Data json.RawMessage
	Hash [32]byte
}

// NewSnapshot 根据已验证的强类型配置创建快照。
func NewSnapshot(version uint64, loadedAt time.Time, entries []SnapshotEntry) Snapshot {
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
	Path    string
	Rule    string
	Message string
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "configuration validation failed"
	}
	issue := e.Issues[0]
	return fmt.Sprintf("configuration validation failed at %s: %s", issue.Path, issue.Message)
}

type Validator interface {
	Validate() error
}
