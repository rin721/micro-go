package registration

import (
	"errors"
	"strings"
	"testing"

	"github.com/rin721/micro-go/internal/kernel/module"
)

type testModule struct {
	name     string
	register func(module.Registry) error
}

func (m testModule) Name() string { return m.name }
func (m testModule) Register(registry module.Registry) error {
	if m.register == nil {
		return nil
	}
	return m.register(registry)
}

func TestCollectRejectsInvalidModules(t *testing.T) {
	sentinel := errors.New("register failed")
	tests := []struct {
		name    string
		modules []module.Module
		message string
	}{
		{name: "nil", modules: []module.Module{nil}, message: "is nil"},
		{name: "empty-name", modules: []module.Module{testModule{}}, message: "empty name"},
		{name: "duplicate", modules: []module.Module{testModule{name: "same"}, testModule{name: "same"}}, message: "duplicate module name"},
		{name: "error", modules: []module.Module{testModule{name: "error", register: func(module.Registry) error { return sentinel }}}, message: sentinel.Error()},
		{name: "panic", modules: []module.Module{testModule{name: "panic", register: func(module.Registry) error { panic("boom") }}}, message: "panic: boom"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Collect(test.modules)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Collect() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestCollectFreezesRegistryAfterRegistration(t *testing.T) {
	var captured module.Registry
	_, err := Collect([]module.Module{testModule{name: "freeze", register: func(registry module.Registry) error {
		captured = registry
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	err = module.Provide(captured, func() *struct{} { return &struct{}{} })
	if err == nil || !strings.Contains(err.Error(), "is frozen") {
		t.Fatalf("Provide() error = %v", err)
	}
}
