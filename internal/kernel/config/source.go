// Package config 定义项目拥有的配置源、快照和校验契约。
package config

import (
	"context"
	"time"
)

// Format 描述原始配置数据格式。解析器实现属于内部适配层。
type Format string

const (
	// FormatMap 表示 Payload 已经是键值树，不需要文本 Parser。
	FormatMap Format = "map"
	// FormatJSON 表示 Payload.Bytes 使用 JSON 编码。
	FormatJSON Format = "json"
	// FormatYAML 表示 Payload.Bytes 使用 YAML 编码。
	FormatYAML Format = "yaml"
)

// Payload 是配置源与配置引擎之间的项目自有传输模型。
type Payload struct {
	// Values 承载已经解析的 map 配置。
	Values map[string]any
	// Bytes 承载仍需 JSON 或 YAML Parser 处理的原始内容。
	Bytes []byte
	// Format 决定内部 Loader 使用 map Provider 还是文本 Parser。
	Format Format
}

// Source 只负责读取一份配置事实，不负责合并或强类型解码。
type Source interface {
	// Name 返回用于诊断、监听和来源排序的稳定名称。
	Name() string
	// Load 按 Context 读取当前事实；失败时不得返回伪造的空配置。
	Load(context.Context) (Payload, error)
}

// WatchDescriptor 描述由内部监听适配器观察的文件。
type WatchDescriptor struct {
	// Path 是需要监听的规范化文件路径。
	Path string
}

// Change 是监听适配器向运行时报告的项目自有变更事件。
type Change struct {
	// Source 是产生变化的项目 Source 名称，而不是 fsnotify 事件类型。
	Source string
	// At 是 Adapter 观察到变化的 UTC 时间。
	At time.Time
}

// WatchSource 是可以声明监听目标的配置源。
type WatchSource interface {
	// Source 复用普通配置源的读取与命名契约。
	Source
	// WatchDescriptor 返回监听所需的项目自有文件描述。
	WatchDescriptor() WatchDescriptor
}
