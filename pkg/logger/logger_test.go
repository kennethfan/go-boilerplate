package logger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		s    string
		want zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"", zapcore.InfoLevel},     // 默认 info
		{"bogus", zapcore.InfoLevel}, // 未知回落 info
	}
	for _, c := range cases {
		if got := parseLevel(c.s); got != c.want {
			t.Errorf("parseLevel(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestNew_Development(t *testing.T) {
	l, err := New(Options{Level: "debug", Development: true})
	if err != nil {
		t.Fatalf("New dev: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	l.Info("dev log line")
	if err := l.Sync(); err != nil {
		// Sync 在 stdout 上可能返回错误，忽略
		_ = err
	}
}

func TestNew_ProductionNoFile(t *testing.T) {
	l, err := New(Options{Level: "info", Development: false, Dir: "", FileName: ""})
	if err != nil {
		t.Fatalf("New prod no file: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	l.Warn("prod log line without file")
}

func TestNew_ProductionWithFile(t *testing.T) {
	dir := t.TempDir()
	l, err := New(Options{
		Level:    "info",
		Development: false,
		Dir:      dir,
		FileName: "app.log",
		MaxSizeMB: 10,
	})
	if err != nil {
		t.Fatalf("New prod with file: %v", err)
	}
	l.Info("persisted log line")
	// lumberjack 在首次写入时落盘
	path := filepath.Join(dir, "app.log")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file should exist: %v", err)
	}
}

func TestL_Uninitialized(t *testing.T) {
	// 直接重置全局（包内可访问），验证未初始化时返回 Nop（不 panic）
	global = nil
	if got := L(); got == nil {
		t.Fatal("L() should never return nil (returns Nop)")
	}
}

func TestFromContext_Nil(t *testing.T) {
	if got := FromContext(nil); got == nil {
		t.Fatal("FromContext(nil) should return non-nil logger")
	}
}

func TestFromContext_NoTrace(t *testing.T) {
	if got := FromContext(context.Background()); got == nil {
		t.Fatal("FromContext without trace should return non-nil logger")
	}
}

func TestFromContext_TraceIDInjected(t *testing.T) {
	// 用 observer core 替换全局 logger，断言 trace_id 字段被注入
	core, logs := observer.New(zapcore.InfoLevel)
	global = zap.New(core)

	tid, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("trace id hex: %v", err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, Remote: true})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	l := FromContext(ctx)
	l.Info("with trace")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 log entry, got %d", len(entries))
	}
	field := entries[0].ContextMap()["trace_id"]
	if field != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id field = %v, want 4bf92f3577b34da6a3ce929d0e0e4736", field)
	}
}

func TestSync(t *testing.T) {
	// Sync 不应 panic（Nop 或真实 logger 均可）
	Sync()
}
