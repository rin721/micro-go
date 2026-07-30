package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/config"
	"github.com/rin721/micro-go/kernel/reload"
)

type reloadConfig struct {
	Value string `yaml:"value" validate:"required"`
}
type reloadable struct{ value string }

func (r *reloadable) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	candidate, err := config.Value[reloadConfig](snapshot)
	if err == nil {
		r.value = candidate.Value
	}
	return reload.Applied, err
}

func TestReloadPromotesValidatedSnapshot(t *testing.T) {
	typeOf := reflect.TypeOf(reloadConfig{})
	component := &reloadable{value: "old"}
	provider := compiled.Provider{Type: reflect.TypeOf(component), Dependencies: []compiled.Dependency{{Requested: typeOf, Resolved: typeOf, Config: true}}}
	a := &Application{
		plan:      &compiled.Plan{Configs: []compiled.Config{{Module: "test", Path: "reload", Type: typeOf}}},
		instances: []compiled.Instance{{Provider: provider, Value: component}},
		snapshot:  config.NewSnapshot(1, time.Now(), nil),
		options:   options{sources: []config.Source{config.FromValues(map[string]any{"reload": map[string]any{"value": "new"}})}},
	}
	if err := a.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if component.value != "new" || a.snapshot.Version != 2 {
		t.Fatalf("reload result value=%q version=%d", component.value, a.snapshot.Version)
	}
}
