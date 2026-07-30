// Package di 暴露与具体容器无关的依赖图只读模型。
package di

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type NodeKind string

const (
	ProviderNode NodeKind = "provider"
	ConfigNode   NodeKind = "config"
)

type Node struct {
	ID        string   `json:"id"`
	Module    string   `json:"module"`
	Type      string   `json:"type"`
	Kind      NodeKind `json:"kind"`
	Order     int      `json:"order"`
	Lifecycle []string `json:"lifecycle,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func (g Graph) JSON() ([]byte, error) { return json.MarshalIndent(g, "", "  ") }

func (g Graph) Text() string {
	var builder strings.Builder
	for _, node := range g.Nodes {
		fmt.Fprintf(&builder, "%02d %s [%s] %s\n", node.Order, node.ID, node.Kind, node.Type)
	}
	for _, edge := range g.Edges {
		fmt.Fprintf(&builder, "%s -> %s (%s)\n", edge.From, edge.To, edge.Via)
	}
	return builder.String()
}

func (g Graph) DOT() string {
	var builder strings.Builder
	builder.WriteString("digraph micro_go {\n")
	nodes := append([]Node(nil), g.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Order < nodes[j].Order })
	for _, node := range nodes {
		fmt.Fprintf(&builder, "  %q [label=%q];\n", node.ID, node.Module+"\\n"+node.Type)
	}
	for _, edge := range g.Edges {
		fmt.Fprintf(&builder, "  %q -> %q [label=%q];\n", edge.From, edge.To, edge.Via)
	}
	builder.WriteString("}\n")
	return builder.String()
}
