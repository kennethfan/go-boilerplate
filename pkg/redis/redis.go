// Package redis 提供 go-redis v9 客户端初始化，内置 OTel tracing 埋点。
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/kenneth/go-boilerplate/pkg/metrics"
)

// Options Redis 连接配置
type Options struct {
	Addr          string
	Password      string
	DB            int
	PoolSize      int
	EnableTracing bool // 是否启用 OTel redis 埋点
	EnableMetrics bool // 是否启用 Prometheus redis 指标
}

// New 建立 Redis 客户端并 Ping 验证连通性
func New(ctx context.Context, opts Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
		PoolSize: opts.PoolSize,
	})

	if opts.EnableTracing {
		// OTel 埋点：每条 redis 命令生成 span（db.system=redis）
		if err := redisotel.InstrumentTracing(rdb); err != nil {
			return nil, fmt.Errorf("redis: enable otel failed: %w", err)
		}
	}

	if opts.EnableMetrics {
		// Prometheus 指标：命令量 / 耗时 / 错误
		rdb.AddHook(&metrics.RedisHook{})
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}
	return rdb, nil
}

// Close 关闭连接池
func Close(rdb *redis.Client) error {
	if rdb == nil {
		return nil
	}
	return rdb.Close()
}
