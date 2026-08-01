package digadapter

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
)

type cancelCloseable struct{ closed bool }

func (c *cancelCloseable) Close(context.Context) error {
	c.closed = true
	return nil
}

type afterCancellation struct{ dependency *cancelCloseable }

func TestConstructCancellationRollsBackCompletedInstances(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var first *cancelCloseable
	firstConstructor := func() *cancelCloseable {
		first = &cancelCloseable{}
		cancel()
		return first
	}
	secondConstructor := func(value *cancelCloseable) *afterCancellation {
		return &afterCancellation{dependency: value}
	}
	plan := &compiled.Plan{Providers: []compiled.Provider{
		{ID: "test:first", Module: "test", Name: "first", Type: reflect.TypeOf((*cancelCloseable)(nil)), Constructor: reflect.ValueOf(firstConstructor)},
		{ID: "test:second", Module: "test", Name: "second", Type: reflect.TypeOf((*afterCancellation)(nil)), Constructor: reflect.ValueOf(secondConstructor), Dependencies: []compiled.Dependency{{Requested: reflect.TypeOf((*cancelCloseable)(nil)), Resolved: reflect.TypeOf((*cancelCloseable)(nil))}}},
	}}
	instances, err := New().Construct(ctx, plan, nil)
	if len(instances) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Construct() = (%+v, %v)", instances, err)
	}
	if first == nil || !first.closed {
		t.Fatal("constructed instance was not rolled back")
	}
}
