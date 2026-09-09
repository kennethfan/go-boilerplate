// Package cron 提供基于 robfig/cron/v3 的定时任务调度器封装。
//
// 设计要点：
//   - 强类型 Options（Location / Logger / Enable）
//   - Start/Stop 配合 ctx，支持优雅退出
//   - 内部用标准 cron 表达式 + robfig 的 @every / @daily 描述符
//   - panic 由链式 cron.Recover 包装 + 用户提供 Logger 输出
//   - 同名 job 重复注册自动覆盖（Remove 旧条目）
//
// 典型用法：
//
//	sched := cron.New(cron.Options{
//	    Enable:   true,
//	    Location: time.Local,
//	    Logger:   cron.ZapCronLogger(l),
//	})
//	defer sched.Stop(30 * time.Second)
//	sched.MustAddFunc("@every 30s", "heartbeat", func(ctx context.Context) error {
//	    l.Info("heartbeat")
//	    return nil
//	})
//	sched.Start(ctx)
package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Options 调度器配置
type Options struct {
	// Enable 总开关；false 时 New/Start 均为 no-op
	Enable bool
	// Location 时区（默认 Local）；影响 cron 表达式解析
	Location *time.Location
	// Logger 自定义 logger；nil 时内部不打印任务生命周期日志（建议始终提供）
	Logger cron.Logger
}

// Manager 调度器实例
type Manager struct {
	opts Options

	mu       sync.Mutex
	started  bool
	stopped  bool
	internal *cron.Cron
	entries  map[string]cron.EntryID // name -> entry id
	specs    map[string]string       // name -> spec（robfig Entry 没 Spec 字段）
}

// New 构造调度器。enable=false 时所有方法为 no-op。
func New(opts Options) *Manager {
	if opts.Location == nil {
		opts.Location = time.Local
	}
	return &Manager{
		opts:    opts,
		entries: make(map[string]cron.EntryID),
		specs:   make(map[string]string),
	}
}

// Enabled 返回是否启用
func (m *Manager) Enabled() bool { return m.opts.Enable }

// AddFunc 注册一个具名定时函数，spec 为标准 cron 表达式或 @every 描述符。
// 同一 name 重复注册会覆盖（先 Remove 旧条目再注册）。
// 必须 Start() 后才会真正开始调度。
func (m *Manager) AddFunc(spec, name string, fn func(ctx context.Context) error) error {
	if !m.opts.Enable {
		return nil
	}
	c := m.ensureCron()
	id, err := c.AddFunc(spec, func() {
		m.runJob(name, fn)
	})
	if err != nil {
		return fmt.Errorf("cron: register %s spec=%q: %w", name, spec, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.entries[name]; ok {
		c.Remove(old)
	}
	m.entries[name] = id
	m.specs[name] = spec
	return nil
}

// MustAddFunc AddFunc 出错时 panic；适合静态初始化路径
func (m *Manager) MustAddFunc(spec, name string, fn func(ctx context.Context) error) {
	if err := m.AddFunc(spec, name, fn); err != nil {
		panic(err)
	}
}

// Start 启动调度循环。重复调用幂等。
func (m *Manager) Start(_ context.Context) {
	if !m.opts.Enable {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started || m.stopped {
		return
	}
	c := m.ensureCron()
	c.Start()
	m.started = true
}

// Stop 停止调度并等待在飞任务结束（最长 wait）。
// wait <= 0 时立即返回；Stop 后不能再 Start。
func (m *Manager) Stop(wait time.Duration) {
	if !m.opts.Enable {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped || !m.started {
		return
	}
	ctx := m.internal.Stop()
	m.stopped = true
	if wait <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(wait):
	}
}

// Entry 调度条目快照
type Entry struct {
	Name string
	ID   int
	Spec string
	Next time.Time
	Prev time.Time
}

// Entries 列出已注册条目（name + spec + 上次/下次触发时间）
func (m *Manager) Entries() []Entry {
	if !m.opts.Enable || m.internal == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, 0, len(m.entries))
	for name, id := range m.entries {
		e := m.internal.Entry(id)
		out = append(out, Entry{
			Name: name,
			ID:   int(id),
			Spec: m.specs[name],
			Next: e.Next,
			Prev: e.Prev,
		})
	}
	return out
}

// runJob 单次任务执行包装：计时 + panic recover
func (m *Manager) runJob(name string, fn func(ctx context.Context) error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error
	defer func() {
		if r := recover(); r != nil {
			m.logPanic(name, r, time.Since(start))
		} else if err != nil {
			m.logError(name, err, time.Since(start))
		} else {
			m.logDone(name, time.Since(start))
		}
	}()
	err = fn(ctx)
}

func (m *Manager) ensureCron() *cron.Cron {
	if m.internal != nil {
		return m.internal
	}
	m.internal = cron.New(
		cron.WithLocation(m.opts.Location),
		cron.WithLogger(m.opts.Logger),
		// 任务还在跑就不再触发新一轮；panic 转给 cron.Recover 链路
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)
	return m.internal
}

// 生命周期日志（done / error / panic）：仅在调用方提供 Logger 时输出
func (m *Manager) logDone(name string, dur time.Duration) {
	if m.opts.Logger == nil {
		return
	}
	m.opts.Logger.Info("cron job done",
		"name", name,
		"duration", dur,
	)
}

func (m *Manager) logError(name string, err error, dur time.Duration) {
	if m.opts.Logger == nil {
		return
	}
	m.opts.Logger.Error(err, "cron job failed",
		"name", name,
		"duration", dur,
	)
}

func (m *Manager) logPanic(name string, r interface{}, dur time.Duration) {
	if m.opts.Logger == nil {
		return
	}
	var pe error
	if e, ok := r.(error); ok {
		pe = e
	} else {
		pe = fmt.Errorf("%v", r)
	}
	m.opts.Logger.Error(pe, "cron job panic",
		"name", name,
		"duration", dur,
	)
}
