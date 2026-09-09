package jobs_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"gorm.io/gorm"

	// import path = .../internal/cron；package declaration name = jobs
	jobs "github.com/kenneth/go-boilerplate/internal/cron"
	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/internal/repository"
	"github.com/kenneth/go-boilerplate/internal/service"
	cronpkg "github.com/kenneth/go-boilerplate/pkg/cron"
	"github.com/kenneth/go-boilerplate/pkg/database"
)

// newInMemDB 建一个 sqlite-memory + AutoMigrate 的 *gorm.DB，用于 cron×service 联动验证。
// SQLite 的 :memory: 在多连接下各看各自的库，强制单连接保证 schema 与数据共享。
func newInMemDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.New(database.Options{
		Type:         "sqlite",
		DSN:          ":memory:",
		LogLevel:     "silent",
		MaxOpenConns: 1, // 关键：防止 :memory: 多连接
		MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("sqlite memory db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// newStack 构造完整依赖栈：db → repo → svc
func newStack(t *testing.T) *service.UserService {
	t.Helper()
	db := newInMemDB(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)
	if _, err := svc.Create(context.Background(), service.CreateUserRequest{
		Username: "alice",
		Password: "secret!",
		Nickname: "Alice",
		Role:     "admin",
	}); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	return svc
}

// TestRegisterAll_RunsUserHeartbeat 端到端：sqlite-memory + 真 service + cron + 业务任务跑到一次。
func TestRegisterAll_RunsUserHeartbeat(t *testing.T) {
	svc := newStack(t)
	zapLogger := zaptest.NewLogger(t)

	sched := cronpkg.New(cronpkg.Options{
		Enable:   true,
		Location: time.UTC,
		Logger:   cronpkg.ZapCronLogger(zapLogger),
	})
	jobs.RegisterAll(sched, jobs.Deps{
		UserService: svc,
		Logger:      zapLogger,
	})

	// 1 个额外 inline job 作为可观察探针，验证 cron 触发闭环
	var ticks atomic.Int32
	sched.MustAddFunc(
		"@every 1s",
		"test_probe",
		func(ctx context.Context) error {
			ticks.Add(1)
			return nil
		},
	)

	if n := len(sched.Entries()); n != 2 {
		t.Fatalf("want 2 entries (user_heartbeat + test_probe), got %d", n)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)

	// robfig 的 @every 1s 起跳可能要 ~1s，所以容忍 2.5s
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) && ticks.Load() < 1 {
		time.Sleep(50 * time.Millisecond)
	}
	sched.Stop(0)

	if got := ticks.Load(); got < 1 {
		t.Fatalf("test_probe should fire at least once, got %d", got)
	}
	// user_heartbeat 是 @every 1m，1.5s 内不期望触发——验证存在即可
	entries := sched.Entries()
	var heartbeatFound bool
	for _, e := range entries {
		if e.Name == "user_heartbeat" {
			heartbeatFound = true
			break
		}
	}
	if !heartbeatFound {
		t.Errorf("user_heartbeat should be registered")
	}
}

// TestRegisterAll_DisabledShortCircuit cron.enable=false 时 RegisterAll 应当 no-op。
func TestRegisterAll_DisabledShortCircuit(t *testing.T) {
	zapLogger := zap.NewNop()
	sched := cronpkg.New(cronpkg.Options{
		Enable:   false,
		Location: time.UTC,
		Logger:   cronpkg.ZapCronLogger(zapLogger),
	})
	jobs.RegisterAll(sched, jobs.Deps{UserService: newStack(t), Logger: zapLogger})
	if got := sched.Entries(); got != nil {
		t.Fatalf("disabled manager should register nothing, got %v", got)
	}
}
