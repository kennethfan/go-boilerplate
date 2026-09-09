package metrics

import (
	"context"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisHook 采集 Redis 命令指标（命令量 / 耗时 / 错误）。
// 实现 go-redis v9 的 Hook 接口，通过 rdb.AddHook 接入。
type RedisHook struct{}

// DialHook 建立连接（不采集指标）
func (RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook 单命令执行
func (RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		observeCommand(cmd.Name(), err, time.Since(start).Seconds())
		return err
	}
}

// ProcessPipelineHook 管道执行
func (RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		// 管道统一记为 pipeline
		observeCommand("pipeline", err, time.Since(start).Seconds())
		return err
	}
}

func observeCommand(command string, err error, seconds float64) {
	status := "ok"
	if err != nil && err != redis.Nil {
		status = "error"
	}
	RedisCommandsTotal.WithLabelValues(command, status).Inc()
	RedisCommandDuration.WithLabelValues(command).Observe(seconds)
}
