package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabase_DSN(t *testing.T) {
	// RawDSN 优先于各字段拼接
	if got := (Database{RawDSN: "RAW-DSN", Type: "mysql"}).DSN(); got != "RAW-DSN" {
		t.Errorf("RawDSN priority: got %q", got)
	}
	// mysql
	mysql := Database{Type: "mysql", Host: "127.0.0.1", Port: 3306, Username: "u", Password: "p", Name: "db"}.DSN()
	if !strings.Contains(mysql, "tcp(127.0.0.1:3306)") {
		t.Errorf("mysql DSN missing host:port: %q", mysql)
	}
	// postgres
	pg := Database{Type: "postgres", Host: "h", Port: 5432, Username: "u", Password: "p", Name: "db"}.DSN()
	if !strings.Contains(pg, "dbname=db") {
		t.Errorf("postgres DSN missing dbname: %q", pg)
	}
	// sqlite 直接返回 Name
	if got := (Database{Type: "sqlite", Name: ":memory:"}).DSN(); got != ":memory:" {
		t.Errorf("sqlite DSN: got %q", got)
	}
	// 缺 type 默认走 mysql
	def := Database{Host: "h", Port: 3306, Username: "u", Password: "p", Name: "db"}.DSN()
	if !strings.Contains(def, "tcp(h:3306)") {
		t.Errorf("default mysql DSN: %q", def)
	}
}

func TestLoad(t *testing.T) {
	yaml := `
app:
  name: test-app
  env: dev
server:
  host: 0.0.0.0
  port: 8080
database:
  type: sqlite
  name: ":memory:"
jwt:
  secret: test-secret-needs-32-bytes-minimum-xxxx
  access_expire: 900
  refresh_expire: 86400
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "test-app" || cfg.App.Env != "dev" {
		t.Errorf("app: %+v", cfg.App)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("server.port: %d", cfg.Server.Port)
	}
	if cfg.Database.Type != "sqlite" {
		t.Errorf("database.type: %q", cfg.Database.Type)
	}
	if cfg.JWT.Secret != "test-secret-needs-32-bytes-minimum-xxxx" {
		t.Errorf("jwt.secret: %q", cfg.JWT.Secret)
	}
	// Get 应返回刚加载的全局配置
	if got := Get(); got == nil || got.App.Name != "test-app" {
		t.Errorf("Get: %+v", got)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("want error for missing config file")
	}
}

func TestLoadRemote(t *testing.T) {
	// 先 Load 一个带 config_center 的本地配置作为全局基线
	base := `
app:
  name: base
config_center:
  type: local
  file_path: /tmp/x.yaml
`
	basePath := filepath.Join(t.TempDir(), "base.yaml")
	if err := os.WriteFile(basePath, []byte(base), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(basePath); err != nil {
		t.Fatal(err)
	}

	remote := `
app:
  name: remote-app
server:
  port: 9090
`
	cfg, err := LoadRemote([]byte(remote))
	if err != nil {
		t.Fatalf("LoadRemote: %v", err)
	}
	if cfg.App.Name != "remote-app" {
		t.Errorf("remote app.name: %q", cfg.App.Name)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("remote server.port: %d", cfg.Server.Port)
	}
	// config_center 应被保留（不被远程数据覆盖）
	if cfg.ConfigCenter.Type != "local" {
		t.Errorf("config_center should be preserved, got %+v", cfg.ConfigCenter)
	}
}

func TestLoadRemote_InvalidYAML(t *testing.T) {
	if _, err := LoadRemote([]byte("\tnot: : valid: yaml: [")); err == nil {
		t.Fatal("want error for invalid YAML")
	}
}
