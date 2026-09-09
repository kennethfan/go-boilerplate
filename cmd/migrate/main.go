// Package main 数据库迁移工具（golang-migrate），支持 mysql / postgres / sqlite。
// 迁移文件按数据库方言分目录组织：migrations/{mysql,postgres,sqlite}/。
// 用法：
//
//	go run ./cmd/migrate -config configs/config.yaml -up        # 升级到最新
//	go run ./cmd/migrate -config configs/config.yaml -down 1    # 回滚 1 个版本
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/kenneth/go-boilerplate/internal/config"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	up := flag.Bool("up", false, "升级到最新版本")
	down := flag.Int("down", 0, "回滚 N 个版本")
	flag.Parse()

	if !*up && *down == 0 {
		fmt.Println("用法: go run ./cmd/migrate -config configs/config.yaml -up | -down N")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	source, dbURL, err := buildMigrateArgs(cfg.Database)
	if err != nil {
		log.Fatalf("构建迁移参数失败: %v", err)
	}

	m, err := migrate.New(source, dbURL)
	if err != nil {
		log.Fatalf("初始化 migrate 失败: %v", err)
	}
	defer m.Close()

	switch {
	case *up:
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("迁移失败: %v", err)
		}
		fmt.Println("迁移完成（已是最新或升级成功）")
	case *down > 0:
		if err := m.Steps(-*down); err != nil {
			log.Fatalf("回滚失败: %v", err)
		}
		fmt.Printf("已回滚 %d 个版本\n", *down)
	}
}

// buildMigrateArgs 根据数据库类型返回迁移源目录与 database URL
func buildMigrateArgs(db config.Database) (source, dbURL string, err error) {
	typ := strings.ToLower(db.Type)
	dsn := db.DSN()

	switch typ {
	case "postgres", "postgresql", "pgsql":
		// DSN 为 key=value 形式，转为 URL 形式供 migrate 使用
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			db.Username, db.Password, db.Host, db.Port, db.Name)
		return "file://migrations/postgres", dbURL, nil
	case "sqlite", "sqlite3":
		return "file://migrations/sqlite", "sqlite3://" + dsn, nil
	case "mysql", "":
		return "file://migrations/mysql", "mysql://" + dsn, nil
	default:
		return "", "", fmt.Errorf("不支持的数据库类型 %q（支持 mysql / postgres / sqlite）", typ)
	}
}
