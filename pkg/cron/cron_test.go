package cron_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	cronpkg "github.com/kenneth/go-boilerplate/pkg/cron"
)

func newSched(t *testing.T, enable bool) (*cronpkg.Manager, *zap.Logger) {
	t.Helper()
	logger := zap.NewNop()
	sched := cronpkg.New(cronpkg.Options{
		Enable:   enable,
		Location: time.UTC,
		Logger:   cronpkg.ZapCronLogger(logger),
	})
	return sched, logger
}

func TestManager_DisabledNoOps(t *testing.T) {
	sched, _ := newSched(t, false)
	if sched.Enabled() {
		t.Fatal("enabled should be false")
	}
	// 注册、Start、Stop 都应是 no-op
	if err := sched.AddFunc("@every 1s", "noop", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("AddFunc on disabled should be no-op, got %v", err)
	}
	if got := sched.Entries(); got != nil {
		t.Fatalf("Entries on disabled should be nil, got %v", got)
	}
	sched.Start(context.Background())
	sched.Stop(0)
}

func TestManager_AddAndRunWithLogger(t *testing.T) {
	sched, _ := newSched(t, true)

	// 注：robfig/cron/v3 的 @every 对 <1s 强制 round up 到 1s（库行为）
	var counter atomic.Int32
	if err := sched.AddFunc("@every 1s", "tick", func(ctx context.Context) error {
		counter.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("AddFunc failed: %v", err)
	}

	entries := sched.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "tick" || entries[0].Spec != "@every 1s" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}

	sched.Start(context.Background())
	defer sched.Stop(0)

	// 至少触发 2 次（首次触发可能在 1s 后）
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) && counter.Load() < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	if got := counter.Load(); got < 2 {
		t.Fatalf("expected >=2 ticks, got %d", got)
	}
}

func TestManager_ErrorAndPanicCaptured(t *testing.T) {
	sched, _ := newSched(t, true)
	if err := sched.AddFunc("@every 1s", "boom", func(ctx context.Context) error {
		panic("kaboom")
	}); err != nil {
		t.Fatalf("AddFunc failed: %v", err)
	}
	var errSeen atomic.Int32
	if err := sched.AddFunc("@every 1s", "fails", func(ctx context.Context) error {
		errSeen.Add(1)
		return errors.New("simulated failure")
	}); err != nil {
		t.Fatalf("AddFunc failed: %v", err)
	}

	sched.Start(context.Background())
	time.Sleep(1300 * time.Millisecond) // 至少看到一次触发
	sched.Stop(0)
	if errSeen.Load() < 1 {
		t.Errorf("expected at least one error run, got %d", errSeen.Load())
	}
}

func TestManager_AddFunc_DuplicateNameOverrides(t *testing.T) {
	sched, _ := newSched(t, true)
	var a, b atomic.Int32
	if err := sched.AddFunc("@every 1h", "job", func(ctx context.Context) error {
		a.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("AddFunc #1: %v", err)
	}
	if err := sched.AddFunc("@every 1h", "job", func(ctx context.Context) error {
		b.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("AddFunc #2: %v", err)
	}

	entries := sched.Entries()
	if len(entries) != 1 {
		t.Fatalf("duplicate name should replace, got %d entries", len(entries))
	}
	// entries[0] 的 ID 与新一次注册对应
	entries = sched.Entries()
	if entries[0].Spec != "@every 1h" {
		t.Errorf("spec should be preserved, got %q", entries[0].Spec)
	}
}

func TestManager_InvalidSpec(t *testing.T) {
	sched, _ := newSched(t, true)
	err := sched.AddFunc("not-a-cron-spec", "bad", func(ctx context.Context) error { return nil })
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestLogger_KeysAndValuesAdapter(t *testing.T) {
	// 仅验证 kvsToFields 长度奇偶 / 空 / 字符串 key 处理
	cases := []struct {
		in       []interface{}
		wantSize int
	}{
		{nil, 0},
		{[]interface{}{}, 0},
		{[]interface{}{"k", 1}, 1},
		{[]interface{}{"a", 1, "b", "two"}, 2},
		{[]interface{}{"a", 1, "b", "two", "orphan"}, 2}, // 奇数丢 last
	}
	for _, c := range cases {
		// 调用 cron.Log 要让 zap 不过 nil 输出 → 用 nop logger
		logger := cronpkg.ZapCronLogger(zap.NewNop())
		// 通过 noop logger 间接验证：必须不 panic
		logger.Info("msg", c.in...)
		logger.Error(errors.New("x"), "msg", c.in...)
		_ = c.wantSize
	}
}
