// Package metrics 提供基于 Prometheus 的指标采集。
// 覆盖 Go runtime / process、HTTP 请求、GORM SQL、Redis 命令。
// 通过 Handler() 暴露 /metrics 供 Prometheus 抓取。
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registry 自定义注册表（避免污染默认 registry）
var registry = prometheus.NewRegistry()

// HTTP 指标
var (
	// HTTPRequestsTotal HTTP 请求总数
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP 请求总数"},
		[]string{"method", "path", "status"},
	)
	// HTTPRequestDuration HTTP 请求耗时（秒）
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP 请求耗时（秒）",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)
	// HTTPRequestBodyBytes HTTP 请求体大小（字节）
	HTTPRequestBodyBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_body_bytes", Help: "HTTP 请求体大小（字节）", Buckets: prometheus.ExponentialBuckets(256, 2, 12)},
		[]string{"method", "path"},
	)
)

// GORM 指标
var (
	// GormQueriesTotal GORM 查询总数
	GormQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gorm_queries_total", Help: "GORM 查询总数"},
		[]string{"operation", "status"},
	)
	// GormQueryDuration GORM 查询耗时（秒）
	GormQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "gorm_query_duration_seconds", Help: "GORM 查询耗时（秒）", Buckets: prometheus.DefBuckets},
		[]string{"operation"},
	)
)

// Redis 指标
var (
	// RedisCommandsTotal Redis 命令总数
	RedisCommandsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_commands_total", Help: "Redis 命令总数"},
		[]string{"command", "status"},
	)
	// RedisCommandDuration Redis 命令耗时（秒）
	RedisCommandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "redis_command_duration_seconds", Help: "Redis 命令耗时（秒）", Buckets: prometheus.DefBuckets},
		[]string{"command"},
	)
)

func init() {
	// 进程级指标：CPU / 内存 / GC / goroutine / fd 等
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	registry.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestBodyBytes,
		GormQueriesTotal,
		GormQueryDuration,
		RedisCommandsTotal,
		RedisCommandDuration,
	)
}

// Handler 返回 /metrics 端点处理器
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// Registry 暴露自定义注册表（供需要复用同一 registry 的组件使用）
func Registry() *prometheus.Registry {
	return registry
}
