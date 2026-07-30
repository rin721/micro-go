package watcher

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rin721/micro-go/kernel/config"
)

// Watch 启动一个由调用方 Context 管理的文件监听器。
func Watch(ctx context.Context, sources []config.Source) (<-chan config.Change, <-chan error, error) {
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("create file watcher: %w", err)
	}
	names := make(map[string]string)
	for _, source := range sources {
		watchable, ok := source.(config.WatchSource)
		if !ok {
			continue
		}
		path := filepath.Clean(watchable.WatchDescriptor().Path)
		names[path] = source.Name()
		if err := watch.Add(filepath.Dir(path)); err != nil {
			_ = watch.Close()
			return nil, nil, fmt.Errorf("watch config directory for %q: %w", path, err)
		}
	}
	events := make(chan config.Change, 1)
	errorsChannel := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errorsChannel)
		defer watch.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-watch.Errors:
				if !ok {
					return
				}
				select {
				case errorsChannel <- fmt.Errorf("watch configuration: %w", err):
				default:
				}
			case event, ok := <-watch.Events:
				if !ok {
					return
				}
				path := filepath.Clean(event.Name)
				name, tracked := names[path]
				if !tracked {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				select {
				case events <- config.Change{Source: name, At: time.Now().UTC()}:
				default:
				}
			}
		}
	}()
	return events, errorsChannel, nil
}
