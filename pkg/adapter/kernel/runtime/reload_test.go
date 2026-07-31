// Package runtime 的重载测试直接验证候选 Snapshot 的提交边界。
package runtime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/reload"
	koanfadapter "github.com/rin721/micro-go/pkg/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/pkg/adapter/kernel/config/source"
	"github.com/rin721/micro-go/pkg/adapter/kernel/di/compiled"
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

// TestReloadPromotesValidatedSnapshot 确认组件成功应用候选后 Application 才提升版本，
// 从而避免组件看到新配置而 Runtime 仍报告旧快照。
func TestReloadPromotesValidatedSnapshot(t *testing.T) {
	typeOf := reflect.TypeOf(reloadConfig{})
	component := &reloadable{value: "old"}
	provider := compiled.Provider{Type: reflect.TypeOf(component), Dependencies: []compiled.Dependency{{Requested: typeOf, Resolved: typeOf, Config: true}}}
	a := &Application{
		plan:      &compiled.Plan{Configs: []compiled.Config{{Module: "test", Path: "reload", Type: typeOf}}},
		instances: []compiled.Instance{{Provider: provider, Value: component}},
		snapshot:  config.NewSnapshot(1, time.Now(), nil),
		options:   options{sources: []config.Source{configsource.FromValues(map[string]any{"reload": map[string]any{"value": "new"}})}},
		runtime:   &Runtime{dependencies: Dependencies{Loader: koanfadapter.New()}},
	}
	if err := a.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if component.value != "new" || a.snapshot.Version != 2 {
		t.Fatalf("reload result value=%q version=%d", component.value, a.snapshot.Version)
	}
}
