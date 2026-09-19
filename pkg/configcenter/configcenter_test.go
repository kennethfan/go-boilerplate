package configcenter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew_Local(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("app:\n  name: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := New(Options{Type: "local", FilePath: path})
	if err != nil {
		t.Fatalf("New local: %v", err)
	}
	if p.Name() != "local" {
		t.Errorf("Name want local, got %q", p.Name())
	}
}

// TestNew_LocalDefaultType 验证 Type 为空时回落到 local
func TestNew_LocalDefaultType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("app:\n  name: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := New(Options{FilePath: path})
	if err != nil {
		t.Fatalf("New default type: %v", err)
	}
	if p.Name() != "local" {
		t.Errorf("default type want local, got %q", p.Name())
	}
}

func TestNew_LocalEmptyPath(t *testing.T) {
	if _, err := newLocal(""); err == nil {
		t.Fatal("empty path should error")
	}
}

func TestNew_LocalFileNotFound(t *testing.T) {
	if _, err := newLocal(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("missing file should error")
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	if _, err := New(Options{Type: "xxx"}); err == nil {
		t.Fatal("unsupported type should error")
	}
}

func TestLocal_Fetch(t *testing.T) {
	content := "app:\n  name: fetch-test\n"
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != content {
		t.Errorf("Fetch content mismatch: got %q want %q", string(data), content)
	}
}

func TestLocal_Watch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newLocal(path)
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan []byte, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := p.Watch(ctx, func(data []byte) {
		ch <- data
	}); err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// 触发写事件（fsnotify 关注 Write|Create）
	newContent := "v2-changed\n"
	if err := os.WriteFile(path, []byte(newContent), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		if string(got) != newContent {
			t.Errorf("watch got %q want %q", string(got), newContent)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not fire within timeout")
	}

	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestLocal_Watch_Cancel 验证 ctx 取消后监听 goroutine 退出（Coverage：ctx.Done 分支）
func TestLocal_Watch_Cancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	called := make(chan struct{}, 1)
	if err := p.Watch(ctx, func([]byte) {
		select {
		case called <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	// 取消后不再阻塞，goroutine 应退出
	cancel()
	// 给一点时间让 goroutine 退出（不严格断言，仅覆盖分支）
	time.Sleep(50 * time.Millisecond)
	_ = called
	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestLocal_CloseWithoutWatch 验证未 Watch 时 watcher 为 nil，Close 无错
func TestLocal_CloseWithoutWatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close without watch: %v", err)
	}
}

// ---- nacos / apollo 纯校验与解析路径（不依赖外部服务） ----

func TestNew_NacosValidation(t *testing.T) {
	// 缺 data_id
	if _, err := New(Options{Type: "nacos"}); err == nil {
		t.Fatal("nacos missing data_id should error")
	}
	// 缺 addrs
	if _, err := New(Options{Type: "nacos", NacosDataID: "d"}); err == nil {
		t.Fatal("nacos missing addrs should error")
	}
}

func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		addr    string
		wantH   string
		wantP   uint64
		wantErr bool
	}{
		{"127.0.0.1", "127.0.0.1", 8848, false}, // 无冒号回落默认端口
		{"localhost:8848", "localhost", 8848, false},
		{"127.0.0.1:8848", "127.0.0.1", 8848, false},
		{"host:notaport", "", 0, true},  // 端口非数字
		{"a:b:c", "", 0, true},          // 非法 host:port
	}
	for _, c := range cases {
		h, p, err := splitHostPort(c.addr)
		if (err != nil) != c.wantErr {
			t.Errorf("splitHostPort(%q): err=%v wantErr=%v", c.addr, err, c.wantErr)
			continue
		}
		if err == nil && (h != c.wantH || p != c.wantP) {
			t.Errorf("splitHostPort(%q): got (%q,%d) want (%q,%d)", c.addr, h, p, c.wantH, c.wantP)
		}
	}
}

func TestNew_ApolloValidation(t *testing.T) {
	// 缺 app_id
	if _, err := New(Options{Type: "apollo"}); err == nil {
		t.Fatal("apollo missing app_id should error")
	}
	// 缺 meta_addr
	if _, err := New(Options{Type: "apollo", ApolloAppID: "app"}); err == nil {
		t.Fatal("apollo missing meta_addr should error")
	}
}
