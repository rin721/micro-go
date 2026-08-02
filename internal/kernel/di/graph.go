// Package di 暴露与具体容器无关的依赖图只读模型。
package di

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NodeKind 区分依赖图中的构造节点和配置节点，使用项目自有字符串类型可避免
// Dig 等容器的图模型进入 Kernel 诊断结果。
type NodeKind string

const (
	// ProviderNode 表示由普通 Go 构造函数创建的组件。
	ProviderNode NodeKind = "provider"
	// ConfigNode 表示已经加载并验证的强类型配置。
	ConfigNode NodeKind = "config"
)

// Node 是可序列化的只读依赖图节点。Order 是 Compiler 计算出的稳定构造顺序，
// Lifecycle 仅描述类型声明的能力，不包含实例或容器引用。
type Node struct {
	// ID 在一张图中稳定标识节点。
	ID string `json:"id"`
	// Module 是声明该节点的模块名。
	Module string `json:"module"`
	// Type 是 Provider 结果或配置的 Go 类型字符串。
	Type string `json:"type"`
	// Kind 区分 Provider 与配置节点。
	Kind NodeKind `json:"kind"`
	// Order 是稳定构造或配置展示顺序。
	Order int `json:"order"`
	// Lifecycle 列出类型实现的可选生命周期能力。
	Lifecycle []string `json:"lifecycle,omitempty"`
}

// Edge 表示依赖由 From 流向消费者 To，Via 保留消费者请求的原始类型，便于诊断
// 接口 Binding 与实际实现类型之间的关系。
type Edge struct {
	// From 是依赖节点 ID。
	From string `json:"from"`
	// To 是消费者 Provider 节点 ID。
	To string `json:"to"`
	// Via 是消费者构造函数请求的原始类型。
	Via string `json:"via"`
}

// Graph 是与具体 DI 引擎无关的依赖图值模型。
// 调用方拿到的是副本，可用于诊断和导出，但不能借此在运行期解析实例。
type Graph struct {
	// Nodes 保存按稳定顺序生成的图节点。
	Nodes []Node `json:"nodes"`
	// Edges 保存依赖到消费者的有向边。
	Edges []Edge `json:"edges"`
}

// JSON 输出稳定的、便于工具消费的缩进 JSON。
func (g Graph) JSON() ([]byte, error) {
	// Graph 只包含可序列化的项目值，因此直接交给标准库并完整返回编码错误。
	return json.MarshalIndent(g, "", "  ")
}

// Text 输出适合终端阅读的节点顺序和依赖边。
func (g Graph) Text() string {
	// Builder 避免为每个节点和边创建中间字符串。
	var builder strings.Builder
	// 节点已由 Compiler 按稳定顺序生成，这里保留该顺序输出。
	for _, node := range g.Nodes {
		fmt.Fprintf(&builder, "%02d %s [%s] %s\n", node.Order, node.ID, node.Kind, node.Type)
	}
	// 边随后输出，方向始终是依赖指向消费者。
	for _, edge := range g.Edges {
		fmt.Fprintf(&builder, "%s -> %s (%s)\n", edge.From, edge.To, edge.Via)
	}
	// 最终一次性取出字符串，保持调用方只读。
	return builder.String()
}

// DOT 输出 Graphviz 图描述。节点先按稳定 Order 排序，避免同一声明在不同运行中
// 产生无意义的图差异。
func (g Graph) DOT() string {
	// 先写入固定图头，所有节点和边都位于同一有向图中。
	var builder strings.Builder
	builder.WriteString("digraph micro_go {\n")
	// 复制节点切片后排序，避免诊断导出修改 Graph 自身的顺序。
	nodes := append([]Node(nil), g.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Order < nodes[j].Order })
	// %q 负责转义类型和模块名，避免手工拼接产生非法 DOT。
	for _, node := range nodes {
		fmt.Fprintf(&builder, "  %q [label=%q];\n", node.ID, node.Module+"\\n"+node.Type)
	}
	// Via 作为边标签保留接口 Binding 等原始依赖请求。
	for _, edge := range g.Edges {
		fmt.Fprintf(&builder, "  %q -> %q [label=%q];\n", edge.From, edge.To, edge.Via)
	}
	// 闭合图并返回完整文本。
	builder.WriteString("}\n")
	return builder.String()
}
