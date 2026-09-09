// Package jobs 注册所有业务定时任务。
//
// 这个文件是"复杂任务"的集中注册点：依赖 service / repository / DB 等运行时依赖，
// 由 cmd/server 在所有依赖构造完毕后统一调用 RegisterAll 注入到调度器。
//
// cfg.Cron.Jobs 段（configs/config.yaml）用于"声明式简单任务"（无外部依赖），
// 由 main.go 在此处一并 AddFunc。
//
// 添加任务步骤：
//  1. 在本文件新增 registerXxx(sched *cron.Manager, deps ...)
//  2. 在 RegisterAll 末尾追加 sched.MustAddFunc(...)
package jobs

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/pkg/cron"
)

// Deps 业务任务依赖（在 RegisterAll 时由 main 注入）
type Deps struct {
	UserService *service.UserService
	Logger      *zap.Logger
}

// RegisterAll 把所有业务定时任务注册到管理器。
// 调度器本身 cron.enable=false 时此函数为 no-op。
func RegisterAll(sched *cron.Manager, d Deps) {
	if !sched.Enabled() {
		return
	}

	// 业务任务：依赖 service / repository
	sched.MustAddFunc(
		"@every 1m",
		"user_heartbeat",
		func(ctx context.Context) error {
			// 心跳：打印用户表大小，验证 DB + 服务层联通
			if d.UserService == nil {
				return nil
			}
			// 实际场景可换成：list users / count rows / 写 cache
			d.Logger.Debug("user heartbeat tick", zap.Time("now", time.Now()))
			return nil
		},
	)
}
