package configcenter

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// localProvider 本地文件数据源，基于 fsnotify 监听文件变更实现热更新。
type localProvider struct {
	path     string
	mu       sync.Mutex
	onChange func([]byte)
	watcher  *fsnotify.Watcher
}

func newLocal(path string) (*localProvider, error) {
	if path == "" {
		return nil, fmt.Errorf("configcenter: local provider needs file_path")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("configcenter: local config file %q not found: %w", path, err)
	}
	return &localProvider{path: path}, nil
}

func (l *localProvider) Name() string { return "local" }

func (l *localProvider) Fetch(_ context.Context) ([]byte, error) {
	return os.ReadFile(l.path)
}

func (l *localProvider) Watch(ctx context.Context, onChange func(data []byte)) error {
	l.mu.Lock()
	l.onChange = onChange
	l.mu.Unlock()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("configcenter: local watch init failed: %w", err)
	}
	if err := w.Add(l.path); err != nil {
		w.Close()
		return fmt.Errorf("configcenter: local watch add failed: %w", err)
	}
	l.watcher = w

	go func() {
		defer w.Close()
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				// 关注写入/创建（编辑器保存常见事件）
				if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					data, err := os.ReadFile(l.path)
					if err != nil {
						continue
					}
					l.mu.Lock()
					fn := l.onChange
					l.mu.Unlock()
					if fn != nil {
						fn(data)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (l *localProvider) Close() error {
	if l.watcher != nil {
		return l.watcher.Close()
	}
	return nil
}
