package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"strings"

	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
)

var sensitiveErrorAssignment = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization|dsn)\b\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;]+)`)

// runtimeObserver 在业务 Logger 构造前提供最小诊断出口。它只消费 Kernel 事件，
// 不参与状态决策；业务日志仍由 DI 图中的 logging.Logger 负责。
type runtimeObserver struct{ logger *slog.Logger }

func newRuntimeObserver(writer io.Writer) *runtimeObserver {
	return &runtimeObserver{logger: slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug}))}
}

func (o *runtimeObserver) Observe(event kernelapp.Event) {
	if o == nil || o.logger == nil {
		return
	}
	level := slog.LevelDebug
	switch {
	case event.Kind == kernelapp.ConfigurationFail || event.Kind == kernelapp.RunnerFailed || event.State == kernelapp.Failed:
		level = slog.LevelError
	case event.State == kernelapp.RestartRequired:
		level = slog.LevelWarn
	case event.Kind == kernelapp.ConfigurationLoad || event.State == kernelapp.Running || event.State == kernelapp.Closed:
		level = slog.LevelInfo
	}
	attributes := []slog.Attr{
		slog.Uint64("sequence", event.Sequence),
		slog.String("kind", string(event.Kind)),
		slog.String("state", event.State.String()),
	}
	if event.Phase != "" {
		attributes = append(attributes, slog.String("phase", string(event.Phase)))
	}
	if event.Module != "" {
		attributes = append(attributes, slog.String("module", event.Module))
	}
	if event.Component != "" {
		attributes = append(attributes, slog.String("component", event.Component))
	}
	if event.Err != nil {
		attributes = append(attributes, slog.String("error", redactDiagnosticError(event.Err.Error())))
	}
	o.logger.LogAttrs(context.Background(), level, "runtime event", attributes...)
}

func redactDiagnosticError(message string) string {
	return sensitiveErrorAssignment.ReplaceAllStringFunc(message, func(value string) string {
		separator := strings.IndexAny(value, ":=")
		if separator < 0 {
			return "[REDACTED]"
		}
		return strings.TrimSpace(value[:separator]) + "=[REDACTED]"
	})
}

var _ kernelapp.Observer = (*runtimeObserver)(nil)
