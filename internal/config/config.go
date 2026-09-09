// Package config 基于 viper 加载结构化配置（configs/config.yaml）。
// 支持本地文件与配置中心（nacos / apollo）远程拉取 + 热更新：
// 全局配置通过 atomic 指针持有，业务侧经 Get() 读取最新值。
package config

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"
)

// global 当前生效配置（热更新时原子替换）
var global atomic.Pointer[Config]

// Get 返回当前生效配置；热更新后返回最新值（并发安全）。
func Get() *Config { return global.Load() }

// Config 应用总配置
type Config struct {
	App          App          `mapstructure:"app"`
	Server       Server       `mapstructure:"server"`
	Database     Database     `mapstructure:"database"`
	Redis        Redis        `mapstructure:"redis"`
	RocketMQ     RocketMQ     `mapstructure:"rocketmq"`
	JWT          JWT          `mapstructure:"jwt"`
	Log          Log          `mapstructure:"log"`
	Otel         Otel         `mapstructure:"otel"`
	Casbin       Casbin       `mapstructure:"casbin"`
	Metrics      Metrics      `mapstructure:"metrics"`
	GRPC         GRPC         `mapstructure:"grpc"`
	Auth         Auth         `mapstructure:"auth"`
	Realtime     Realtime     `mapstructure:"realtime"`
	Docs         Docs         `mapstructure:"docs"`
	Cron         Cron         `mapstructure:"cron"`
	Registry     Registry     `mapstructure:"registry"`
	ConfigCenter Center       `mapstructure:"config_center"`
}

type App struct {
	Name  string `mapstructure:"name"`
	Env   string `mapstructure:"env"` // dev / test / prod
	Debug bool   `mapstructure:"debug"`
}

type Server struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// Database 数据库配置（mysql / postgres / sqlite 三选一）
type Database struct {
	Type     string `mapstructure:"type"` // mysql / postgres / sqlite
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"` // 数据库名；sqlite 时为文件路径或 :memory:
	RawDSN   string `mapstructure:"dsn"`  // 可选：直接指定完整 DSN，覆盖上述字段

	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	LogLevel        string        `mapstructure:"log_level"`
	AutoMigrate     bool          `mapstructure:"auto_migrate"` // 启动时 GORM 自动建表
}

// DSN 按类型拼接连接串；显式配置 dsn 时优先生效
func (d Database) DSN() string {
	if d.RawDSN != "" {
		return d.RawDSN
	}
	switch strings.ToLower(d.Type) {
	case "postgres", "postgresql", "pgsql":
		return fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			d.Host, d.Port, d.Username, d.Password, d.Name,
		)
	case "sqlite", "sqlite3":
		return d.Name // 文件路径或 :memory:
	default: // mysql
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Name,
		)
	}
}

// Redis Redis 客户端配置
type Redis struct {
	Enable  bool   `mapstructure:"enable"` // 总开关：关闭则不初始化客户端，跳过 redis 相关健康检查
	Addr    string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB      int    `mapstructure:"db"`
	PoolSize int   `mapstructure:"pool_size"`
}

// Auth 鉴权配置：关闭则不挂 JWT/Casbin 中间件，业务路由放行
type Auth struct {
	Enable bool `mapstructure:"enable"`
}

// Realtime 实时通信配置：关闭则不挂 WS/SSE 路由
type Realtime struct {
	Enable bool `mapstructure:"enable"`
}

// Docs API 文档配置：关闭则不挂 /swagger
type Docs struct {
	Enable bool `mapstructure:"enable"`
}

// Cron 定时任务调度器配置
type Cron struct {
	Enable   bool       `mapstructure:"enable"`    // 总开关
	Location string     `mapstructure:"location"`  // 时区，如 "Asia/Shanghai"；默认 Local
	Jobs     []CronJob  `mapstructure:"jobs"`      // 声明式任务（无外部依赖），由 main 一次性注册
}

// CronJob 声明式任务（用于无外部依赖、纯函数型任务）
type CronJob struct {
	Name        string `mapstructure:"name"`         // 任务唯一名
	Spec        string `mapstructure:"spec"`         // cron 表达式或 @every 30s、@daily 等
	Description string `mapstructure:"description"` // 描述（仅日志/文档用）
	Log         string `mapstructure:"log"`          // 触发时打印的日志文本，支持 {name} / {time} 占位
}

type RocketMQ struct {
	Endpoints []string     `mapstructure:"endpoints"`
	Producer  MQProducer   `mapstructure:"producer"`
	Consumer  MQConsumer   `mapstructure:"consumer"`
}

type MQProducer struct {
	Enable bool `mapstructure:"enable"`
	Retry  int  `mapstructure:"retry"`
}

type MQConsumer struct {
	Enable bool   `mapstructure:"enable"`
	Group  string `mapstructure:"group"`
	Topic  string `mapstructure:"topic"`
}

type JWT struct {
	Secret        string        `mapstructure:"secret"`
	Issuer        string        `mapstructure:"issuer"`
	AccessExpire  time.Duration `mapstructure:"access_expire"`
	RefreshExpire time.Duration `mapstructure:"refresh_expire"`
}

type Log struct {
	Level      string `mapstructure:"level"`
	Dir        string `mapstructure:"dir"`
	FileName   string `mapstructure:"filename"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

type Otel struct {
	Enable      bool    `mapstructure:"enable"`
	Endpoint    string  `mapstructure:"endpoint"`
	Insecure    bool    `mapstructure:"insecure"`
	ServiceName string  `mapstructure:"service_name"`
	SampleRatio float64 `mapstructure:"sample_ratio"`
}

type Casbin struct {
	ModelPath string `mapstructure:"model_path"`
}

// Metrics Prometheus 指标配置
type Metrics struct {
	Enable bool `mapstructure:"enable"` // 是否启用 GORM/Redis 指标采集（HTTP 指标与 /metrics 端点始终可用）
}

// GRPC gRPC server 配置（启用需 build tag grpc 并先 make proto 生成 stub）
type GRPC struct {
	Enable bool   `mapstructure:"enable"`
	Addr   string `mapstructure:"addr"`
}

// Registry 服务注册中心配置（启动时注册、退出时注销）
type Registry struct {
	Enable         bool              `mapstructure:"enable"`
	Type           string            `mapstructure:"type"` // nacos
	NacosAddrs     []string          `mapstructure:"nacos_addrs"`
	NacosNamespace string            `mapstructure:"nacos_namespace"`
	ServiceName    string            `mapstructure:"service_name"`
	Group          string            `mapstructure:"group"`
	IP             string            `mapstructure:"ip"`     // 留空自动获取本机 IPv4
	Port           int               `mapstructure:"port"`   // 留 0 默认为 cfg.Server.Port
	Ephemeral      bool              `mapstructure:"ephemeral"`
	Metadata       map[string]string `mapstructure:"metadata"`
}

// Center 配置中心连接配置（bootstrap 阶段从本地读取）
type Center struct {
	Enable bool   `mapstructure:"enable"`
	Type   string `mapstructure:"type"` // local / nacos / apollo

	// local
	FilePath string `mapstructure:"file_path"`

	// nacos
	NacosAddrs     []string `mapstructure:"nacos_addrs"`
	NacosNamespace string   `mapstructure:"nacos_namespace"`
	NacosDataID    string   `mapstructure:"nacos_data_id"`
	NacosGroup     string   `mapstructure:"nacos_group"`

	// apollo
	ApolloAppID       string   `mapstructure:"apollo_app_id"`
	ApolloCluster     string   `mapstructure:"apollo_cluster"`
	ApolloMetaAddr    string   `mapstructure:"apollo_meta_addr"`
	ApolloNamespaces  []string `mapstructure:"apollo_namespaces"`
	ApolloContentKey  string   `mapstructure:"apollo_content_key"`
	ApolloSecret      string   `mapstructure:"apollo_secret"`
}

// Load 从本地 bootstrap 文件读取配置并写入全局（含 config_center 连接信息）。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read %s failed: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal failed: %w", err)
	}
	global.Store(&cfg)
	return &cfg, nil
}

// LoadRemote 用配置中心拉取的完整 YAML 覆盖业务配置并原子更新全局。
// 注意：config_center 连接信息（bootstrap 中）不会被远程数据覆盖。
func LoadRemote(data []byte) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("config: remote read failed: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: remote unmarshal failed: %w", err)
	}

	// 保留 bootstrap 中的 config_center 连接信息
	if old := global.Load(); old != nil {
		cfg.ConfigCenter = old.ConfigCenter
	}
	global.Store(&cfg)
	return &cfg, nil
}
