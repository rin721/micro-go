package bootstrap

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
)

func TestRuntimeObserverReportsReloadFailureAndRedactsAssignments(t *testing.T) {
	var output bytes.Buffer
	observer := newRuntimeObserver(&output)
	observer.Observe(kernelapp.Event{
		Sequence: 4,
		Kind:     kernelapp.ConfigurationFail,
		State:    kernelapp.Running,
		Phase:    diagnostic.ConfigValidate,
		Err:      errors.New("invalid candidate token=visible password: also-visible at line 3"),
	})
	logged := output.String()
	for _, forbidden := range []string{"token=visible", "also-visible"} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("observer leaked %q in %s", forbidden, logged)
		}
	}
	for _, required := range []string{`"kind":"config.failed"`, `"state":"Running"`, "[REDACTED]", "line 3"} {
		if !strings.Contains(logged, required) {
			t.Fatalf("observer output %s does not contain %q", logged, required)
		}
	}
}
