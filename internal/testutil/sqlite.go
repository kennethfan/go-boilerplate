// Package testutil 提供跨包共享的测试脚手架。
package testutil

import (
	"testing"

	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/pkg/database"
	"gorm.io/gorm"
)

// NewSQLiteMemory 构造一个 :memory: SQLite 单连接 gorm.DB，并 auto-migrate model.User。
//
// 关键约束：MaxOpenConns=1 + MaxIdleConns=1。
// 否则 :memory: 在多连接下各自独立（每个连接看到空库），报 no such table: main.users。
func NewSQLiteMemory(t *testing.T) *gorm.DB {
	t.Helper()
	db := newSQLite(t)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("testutil: migrate users: %v", err)
	}
	return db
}

// NewSQLiteMemoryDB 仅构造 :memory: 单连接 gorm.DB，不建业务表（供 casbin 等自管表结构使用）。
func NewSQLiteMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	return newSQLite(t)
}

func newSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.New(database.Options{
		Type:         database.TypeSQLite,
		DSN:          ":memory:",
		LogLevel:     "silent",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("testutil: open sqlite memory: %v", err)
	}
	return db
}
