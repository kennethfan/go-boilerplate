// Package database 提供统一的多数据库 GORM 初始化，支持 MySQL / PostgreSQL / SQLite。
// SQLite 采用纯 Go 驱动（glebarez/sqlite），无需 CGO，兼容交叉编译。
package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/kenneth/go-boilerplate/pkg/metrics"
)

// 支持的数据库类型常量
const (
	TypeMySQL    = "mysql"
	TypePostgres = "postgres"
	TypeSQLite   = "sqlite"
)

// Options 数据库连接配置
type Options struct {
	Type            string        // mysql / postgres / sqlite
	DSN             string        // 已拼接好的连接串（由 config 层生成）
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	LogLevel        string // silent / error / warn / info
	EnableTracing   bool   // 是否启用 OTel GORM 埋点
	EnableMetrics   bool   // 是否启用 Prometheus GORM 指标
}

// New 按类型选择驱动建立 GORM 连接，应用连接池 / 日志 / tracing 配置。
func New(opts Options) (*gorm.DB, error) {
	dialector, err := dialectorFor(opts.Type, opts.DSN)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(parseLogLevel(opts.LogLevel)),
	})
	if err != nil {
		return nil, fmt.Errorf("database: open %s failed: %w", opts.Type, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: get raw db failed: %w", err)
	}
	sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)

	// OTel 埋点：GORM 每条 SQL 生成 span（SpanKind=client），仅 tracing 不打指标
	if opts.EnableTracing {
		if err := db.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
			return nil, fmt.Errorf("database: enable otel plugin failed: %w", err)
		}
	}

	// Prometheus 指标：GORM 查询量 / 耗时 / 错误
	if opts.EnableMetrics {
		if err := db.Use(&metrics.GormPlugin{}); err != nil {
			return nil, fmt.Errorf("database: enable prometheus plugin failed: %w", err)
		}
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping %s failed: %w", opts.Type, err)
	}
	return db, nil
}

// dialectorFor 根据类型返回对应 GORM 驱动
func dialectorFor(typ, dsn string) (gorm.Dialector, error) {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case TypeMySQL:
		return mysql.Open(dsn), nil
	case TypePostgres, "postgresql", "pgsql":
		return postgres.Open(dsn), nil
	case TypeSQLite, "sqlite3":
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("database: unsupported type %q (support mysql / postgres / sqlite)", typ)
	}
}

// parseLogLevel 解析日志级别
func parseLogLevel(s string) gormlogger.LogLevel {
	switch strings.ToLower(s) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}
