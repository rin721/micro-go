package compiler

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/rin721/micro-go/internal/adapter/kernel/module"
	publicmodule "github.com/rin721/micro-go/internal/kernel/module"
)

func declaration(owner string, order int, constructor any) registration.Provider {
	return registration.Provider{Module: owner, ModuleOrder: order, Declaration: publicmodule.ProviderDeclaration{Constructor: constructor}}
}

func TestCompileRejectsInvalidProviderSignatures(t *testing.T) {
	tests := []struct {
		name        string
		constructor any
		message     string
	}{
		{name: "nil", constructor: nil, message: "nil provider"},
		{name: "not-function", constructor: 42, message: "must be a function"},
		{name: "variadic", constructor: func(...string) *struct{} { return nil }, message: "must not be variadic"},
		{name: "no-result", constructor: func() {}, message: "must return Component"},
		{name: "interface-result", constructor: func() error { return nil }, message: "must return a concrete component"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(registration.Collection{Providers: []registration.Provider{declaration("test", 0, test.constructor)}})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Compile() error = %v, want %q", err, test.message)
			}
		})
	}
}

type providerCycleA struct{ b *providerCycleB }
type providerCycleB struct{ a *providerCycleA }

func TestCompileRejectsProviderCycle(t *testing.T) {
	collection := registration.Collection{Providers: []registration.Provider{
		declaration("cycle", 0, func(b *providerCycleB) *providerCycleA { return &providerCycleA{b: b} }),
		declaration("cycle", 0, func(a *providerCycleA) *providerCycleB { return &providerCycleB{a: a} }),
	}}
	_, err := Compile(collection)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("Compile() error = %v", err)
	}
}

type moduleContractA interface{ A() }
type moduleContractB interface{ B() }
type moduleImplementationA struct{}
type moduleImplementationB struct{}
type moduleConsumerA struct{ dependency moduleContractB }
type moduleConsumerB struct{ dependency moduleContractA }

func (*moduleImplementationA) A() {}
func (*moduleImplementationB) B() {}

func TestCompileRejectsModuleCycleWithoutProviderCycle(t *testing.T) {
	contractA := reflect.TypeOf((*moduleContractA)(nil)).Elem()
	contractB := reflect.TypeOf((*moduleContractB)(nil)).Elem()
	implementationA := reflect.TypeOf((*moduleImplementationA)(nil))
	implementationB := reflect.TypeOf((*moduleImplementationB)(nil))
	collection := registration.Collection{
		Providers: []registration.Provider{
			declaration("module-a", 0, func() *moduleImplementationA { return &moduleImplementationA{} }),
			declaration("module-a", 0, func(value moduleContractB) *moduleConsumerA { return &moduleConsumerA{dependency: value} }),
			declaration("module-b", 1, func() *moduleImplementationB { return &moduleImplementationB{} }),
			declaration("module-b", 1, func(value moduleContractA) *moduleConsumerB { return &moduleConsumerB{dependency: value} }),
		},
		Bindings: []registration.Binding{
			{Module: "module-a", Declaration: publicmodule.BindingDeclaration{Contract: contractA, Implementation: implementationA}},
			{Module: "module-b", ModuleOrder: 1, Declaration: publicmodule.BindingDeclaration{Contract: contractB, Implementation: implementationB}},
		},
		Exports: []registration.Export{
			{Module: "module-a", Declaration: publicmodule.ExportDeclaration{Contract: contractA}},
			{Module: "module-b", ModuleOrder: 1, Declaration: publicmodule.ExportDeclaration{Contract: contractB}},
		},
	}
	_, err := Compile(collection)
	if err == nil || !strings.Contains(err.Error(), "module dependency cycle detected among module-a, module-b") {
		t.Fatalf("Compile() error = %v", err)
	}
}

type stableA struct{}
type stableB struct{ a *stableA }

func TestCompileProducesStableGraph(t *testing.T) {
	collection := registration.Collection{Providers: []registration.Provider{
		declaration("stable", 0, func() *stableA { return &stableA{} }),
		declaration("stable", 0, func(a *stableA) *stableB { return &stableB{a: a} }),
	}}
	first, err := Compile(collection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(collection)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := first.Graph.JSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := second.Graph.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("graph is not stable:\n%s\n%s", firstJSON, secondJSON)
	}
}
