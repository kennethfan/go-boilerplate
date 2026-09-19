package database

import (
	"testing"

	gormlogger "gorm.io/gorm/logger"

	"github.com/kenneth/go-boilerplate/internal/model"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/kenneth/go-boilerplate/pkg/metrics"
)

func TestDialectorFor(t *testing.T) {
	cases := []struct {
		typ     string
		wantErr bool
	}{
		{"mysql", false},
		{"MySQL", false}, // 大小写/空白应被归一化
		{"postgres", false},
		{"postgresql", false},
		{"pgsql", false},
		{"sqlite", false},
		{"sqlite3", false},
		{"oracle", true}, // 不支持
		{"", true},
	}
	for _, c := range cases {
		_, err := dialectorFor(c.typ, "dsn-string")
		if (err != nil) != c.wantErr {
			t.Errorf("dialectorFor(%q): err=%v, wantErr=%v", c.typ, err, c.wantErr)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	if parseLogLevel("silent") != gormlogger.Silent {
		t.Error("silent want Silent")
	}
	if parseLogLevel("error") != gormlogger.Error {
		t.Error("error want Error")
	}
	if parseLogLevel("info") != gormlogger.Info {
		t.Error("info want Info")
	}
	if parseLogLevel("warn") != gormlogger.Warn {
		t.Error("warn want Warn")
	}
	if parseLogLevel("") != gormlogger.Warn {
		t.Error("empty want default Warn")
	}
	if parseLogLevel("bogus") != gormlogger.Warn {
		t.Error("bogus want default Warn")
	}
}

func TestNew_SQLite(t *testing.T) {
	// sqlite 纯 Go 驱动，:memory: 可本地打开并 Ping
	db, err := New(Options{
		Type:         TypeSQLite,
		DSN:          ":memory:",
		LogLevel:     "silent",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("New sqlite: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	if _, err := New(Options{Type: "oracle", DSN: "x"}); err == nil {
		t.Fatal("unsupported type should error")
	}
}

// TestNew_SQLite_WithMetrics 验证 metrics 插件注册路径，并借一次真实查询覆盖 pkg/metrics/gorm.go 的回调。
func TestNew_SQLite_WithMetrics_ExercisesGormPlugin(t *testing.T) {
	db, err := New(Options{
		Type:          TypeSQLite,
		DSN:           ":memory:",
		LogLevel:      "silent",
		MaxOpenConns:  1,
		MaxIdleConns:  1,
		EnableMetrics: true,
	})
	if err != nil {
		t.Fatalf("New sqlite: %v", err)
	}
	// 单连接避免 :memory: 多连接各看各库的坑
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{Username: "u1", Password: "p", Role: "user"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var u model.User
	if err := db.First(&u, 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}

	// 断言 gorm 指标已累加（query 路径）
	if got := promtestutil.ToFloat64(metrics.GormQueriesTotal.WithLabelValues("query", "ok")); got < 1 {
		t.Errorf("gorm query metric want >=1, got %v", got)
	}
}
