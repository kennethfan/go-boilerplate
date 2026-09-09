// Package main 服务入口：组装配置、日志、OTel、MySQL、Redis、Casbin、RocketMQ，
// 提供 seed 与优雅退出。
//
//	@title           go-boilerplate API
//	@version         1.0
//	@description     Go 后端脚手架：Gin + GORM + Redis + RocketMQ + JWT/Casbin + zap + OpenTelemetry
//	@BasePath        /api/v1
//	@securityDefinitions.apikey BearerAuth
//	@in                         header
//	@name                       Authorization
//	@description                Bearer {access_token}
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kenneth/go-boilerplate/internal/config"
	cronjobs "github.com/kenneth/go-boilerplate/internal/cron"
	"github.com/kenneth/go-boilerplate/internal/grpcstart"
	"github.com/kenneth/go-boilerplate/internal/handler"
	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/internal/repository"
	"github.com/kenneth/go-boilerplate/internal/router"
	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/pkg/casbinx"
	"github.com/kenneth/go-boilerplate/pkg/configcenter"
	"github.com/kenneth/go-boilerplate/pkg/cron"
	"github.com/kenneth/go-boilerplate/pkg/database"
	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
	"github.com/kenneth/go-boilerplate/pkg/logger"
	mqpkg "github.com/kenneth/go-boilerplate/pkg/mq"
	otelx "github.com/kenneth/go-boilerplate/pkg/otelx"
	redispkg "github.com/kenneth/go-boilerplate/pkg/redis"
	"github.com/kenneth/go-boilerplate/pkg/registry"
	wspkg "github.com/kenneth/go-boilerplate/pkg/ws"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "server exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	// 0. 退出信号根 context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 1. 加载本地 bootstrap 配置（含 config_center 连接信息）
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 1.5 配置中心：远程拉取 + 热更新（覆盖业务配置）
	if cfg.ConfigCenter.Enable {
		var provider configcenter.Provider
		provider, cfg, err = initConfigCenter(ctx, cfg)
		if err != nil {
			return fmt.Errorf("init config center: %w", err)
		}
		defer provider.Close()
	}

	// 2. 日志
	l, err := logger.New(logger.Options{
		Level:       cfg.Log.Level,
		Dir:         cfg.Log.Dir,
		FileName:    cfg.Log.FileName,
		MaxSizeMB:   cfg.Log.MaxSizeMB,
		MaxBackups:  cfg.Log.MaxBackups,
		MaxAgeDays:  cfg.Log.MaxAgeDays,
		Compress:    cfg.Log.Compress,
		Development: cfg.App.Env == "dev",
	})
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()

	// 3. OTel 链路追踪
	otelShutdown, err := otelx.Init(ctx, otelx.Options{
		Enable:      cfg.Otel.Enable,
		ServiceName: cfg.Otel.ServiceName,
		Endpoint:    cfg.Otel.Endpoint,
		Insecure:    cfg.Otel.Insecure,
		SampleRatio: cfg.Otel.SampleRatio,
	})
	if err != nil {
		return fmt.Errorf("init otel: %w", err)
	}

	// 4. 数据库（MySQL / PostgreSQL / SQLite，启用 GORM tracing + 指标）
	db, err := database.New(database.Options{
		Type:            cfg.Database.Type,
		DSN:             cfg.Database.DSN(),
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		LogLevel:        cfg.Database.LogLevel,
		EnableTracing:   cfg.Otel.Enable,
		EnableMetrics:   cfg.Metrics.Enable,
	})
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	// 可选：GORM 自动建表（跨库自动适配，无需手写方言 SQL）
	if cfg.Database.AutoMigrate {
		if err := db.AutoMigrate(&model.User{}); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
		l.Info("auto migrate completed", zap.String("db", cfg.Database.Type))
	}

	// 5. Redis（关闭时不初始化，rdb 保持 nil，HealthHandler 会跳过 Redis 检查）
	var rdb *redis.Client
	if cfg.Redis.Enable {
		rdb, err = redispkg.New(ctx, redispkg.Options{
			Addr:          cfg.Redis.Addr,
			Password:      cfg.Redis.Password,
			DB:            cfg.Redis.DB,
			PoolSize:      cfg.Redis.PoolSize,
			EnableTracing: cfg.Otel.Enable,
			EnableMetrics: cfg.Metrics.Enable,
		})
		if err != nil {
			return fmt.Errorf("init redis: %w", err)
		}
		defer func() { _ = redispkg.Close(rdb) }()
	}

	// 6. Casbin RBAC
	enforcer, err := casbinx.NewEnforcer(db, cfg.Casbin.ModelPath)
	if err != nil {
		return fmt.Errorf("init casbin: %w", err)
	}
	if err := casbinx.SeedPolicies(enforcer); err != nil {
		return fmt.Errorf("seed casbin policies: %w", err)
	}

	// 7. 首次启动 seed 内置 admin 账号
	userRepo := repository.NewUserRepository(db)
	if err := seedAdmin(ctx, userRepo); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	// 8. RocketMQ 生产者
	var producer *mqpkg.Producer
	if cfg.RocketMQ.Producer.Enable {
		producer, err = mqpkg.NewProducer(mqpkg.ProducerOptions{
			Endpoints: cfg.RocketMQ.Endpoints,
			Retry:     cfg.RocketMQ.Producer.Retry,
		})
		if err != nil {
			return fmt.Errorf("init rocketmq producer: %w", err)
		}
		defer producer.Shutdown()
	}

	// 9. RocketMQ 消费者（demo topic）
	var mqConsumer *mqpkg.Consumer
	if cfg.RocketMQ.Consumer.Enable {
		mqConsumer, err = newDemoConsumer(cfg, l)
		if err != nil {
			return fmt.Errorf("init rocketmq consumer: %w", err)
		}
		defer mqConsumer.Shutdown()
	}

	// 10. HTTP 服务
	jwtOpts := jwtpkg.Options{
		Secret:        cfg.JWT.Secret,
		Issuer:        cfg.JWT.Issuer,
		AccessExpire:  cfg.JWT.AccessExpire,
		RefreshExpire: cfg.JWT.RefreshExpire,
	}
	authSvc := service.NewAuthService(userRepo, jwtOpts)
	userSvc := service.NewUserService(userRepo)

	// WebSocket 连接管理 Hub（广播/单发）
	hub := wspkg.NewHub()

	// 9.5 定时任务调度器
	// 声明式任务（cfg.Cron.Jobs）+ 业务任务（internal/cron）一并注册
	// enable=false 时构造/Start/Stop 全是 no-op，零开销
	cronLoc := time.Local
	if cfg.Cron.Location != "" {
		if loc, err := time.LoadLocation(cfg.Cron.Location); err == nil {
			cronLoc = loc
		} else {
			l.Warn("cron: invalid location, fallback to local",
				zap.String("location", cfg.Cron.Location), zap.Error(err))
		}
	}
	sched := cron.New(cron.Options{
		Enable:   cfg.Cron.Enable,
		Location: cronLoc,
		Logger:   cron.ZapCronLogger(l),
	})
	for _, j := range cfg.Cron.Jobs {
		j := j
		if err := sched.AddFunc(j.Spec, j.Name, func(ctx context.Context) error {
			msg := j.Log
			if msg == "" {
				msg = "cron tick"
			}
			l.Info(msg,
				zap.String("job", j.Name),
				zap.String("spec", j.Spec),
				zap.Time("now", time.Now()),
			)
			return nil
		}); err != nil {
			l.Warn("cron: register declarative job failed",
				zap.String("name", j.Name), zap.String("spec", j.Spec), zap.Error(err))
		}
	}
	cronjobs.RegisterAll(sched, cronjobs.Deps{
		UserService: userSvc,
		Logger:      l,
	})
	if cfg.Cron.Enable {
		sched.Start(ctx)
		l.Info("cron started", zap.Int("jobs", len(sched.Entries())))
		for _, e := range sched.Entries() {
			l.Info("cron registered",
				zap.String("name", e.Name),
				zap.String("spec", e.Spec),
				zap.Time("next", e.Next),
			)
		}
	} else {
		l.Info("cron disabled")
	}

	// gRPC server（默认构建空实现；启用需 `go build -tags grpc` 并先 `make proto`）
	if err := grpcstart.Start(ctx, cfg, l, userSvc); err != nil {
		return fmt.Errorf("init grpc: %w", err)
	}

	r := router.New(router.Dependencies{
		Mode:          ginMode(cfg.App.Env),
		JWTOpts:       jwtOpts,
		Enforcer:      enforcer,
		Logger:        l,
		AuthHandler:   handler.NewAuthHandler(authSvc),
		UserHandler:   handler.NewUserHandler(userSvc),
		HealthHandler: handler.NewHealthHandler(db, rdb),
		WSHandler:     handler.NewWSHandler(hub, jwtOpts),
		SSEHandler:    handler.NewSSEHandler(),
	})

	// 11. 服务注册中心（启动时注册、退出时注销）
	if err := startRegistry(ctx, cfg); err != nil {
		return fmt.Errorf("init registry: %w", err)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 启动 HTTP（异步）
	errCh := make(chan error, 1)
	go func() {
		l.Info("server started", zap.String("addr", srv.Addr), zap.String("env", cfg.App.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// 11. 等待退出信号或启动错误
	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		l.Info("shutdown signal received")
	}

	// 12. 优雅退出：cron → HTTP → OTel flush
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if cfg.Cron.Enable {
		sched.Stop(cfg.Server.ShutdownTimeout) // 不再接收新触发，等待在飞任务
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		l.Error("http shutdown error", zap.Error(err))
	}
	if err := otelShutdown(shutdownCtx); err != nil {
		l.Error("otel shutdown error", zap.Error(err))
	}
	l.Info("server exited gracefully")
	return nil
}

func ginMode(env string) string {
	if env == "dev" {
		return gin.DebugMode
	}
	return gin.ReleaseMode
}

// startRegistry 启动时向注册中心注册本实例；ctx 结束时主动注销
func startRegistry(ctx context.Context, cfg *config.Config) error {
	if !cfg.Registry.Enable {
		return nil
	}
	provider, err := registry.New(registry.Options{
		Type:           cfg.Registry.Type,
		NacosAddrs:     cfg.Registry.NacosAddrs,
		NacosNamespace: cfg.Registry.NacosNamespace,
		ServiceName:    cfg.Registry.ServiceName,
		Group:          cfg.Registry.Group,
		IP:             cfg.Registry.IP,
		Port:           cfg.Registry.Port,
		Ephemeral:      cfg.Registry.Ephemeral,
		Metadata:       cfg.Registry.Metadata,
	})
	if err != nil {
		return err
	}

	port := cfg.Registry.Port
	if port == 0 {
		port = cfg.Server.Port
	}
	ins := registry.Instance{
		ServiceName: cfg.Registry.ServiceName,
		Group:       cfg.Registry.Group,
		IP:          cfg.Registry.IP,
		Port:        port,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   cfg.Registry.Ephemeral,
		Metadata:    cfg.Registry.Metadata,
	}
	if err := provider.Register(ctx, ins); err != nil {
		return err
	}

	// 监听退出信号，注销并关闭
	go func() {
		<-ctx.Done()
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = provider.Deregister(dctx, ins)
		provider.Close()
	}()
	return nil
}

// initConfigCenter 从配置中心拉取业务配置覆盖本地，并启动热更新监听。
// 日志尚未初始化，过程信息走 stderr。
func initConfigCenter(ctx context.Context, cfg *config.Config) (configcenter.Provider, *config.Config, error) {
	cc := cfg.ConfigCenter
	provider, err := configcenter.New(configcenter.Options{
		Type:              cc.Type,
		FilePath:          cc.FilePath,
		NacosAddrs:        cc.NacosAddrs,
		NacosNamespace:    cc.NacosNamespace,
		NacosDataID:       cc.NacosDataID,
		NacosGroup:        cc.NacosGroup,
		ApolloAppID:       cc.ApolloAppID,
		ApolloCluster:     cc.ApolloCluster,
		ApolloMetaAddr:    cc.ApolloMetaAddr,
		ApolloNamespaces:  cc.ApolloNamespaces,
		ApolloContentKey:  cc.ApolloContentKey,
		ApolloSecret:      cc.ApolloSecret,
	})
	if err != nil {
		return nil, nil, err
	}

	// 首次拉取：失败则降级使用本地配置，不阻塞启动
	if data, err := provider.Fetch(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[config] fetch from %s failed, fallback to local: %v\n", provider.Name(), err)
	} else if newCfg, err := config.LoadRemote(data); err != nil {
		fmt.Fprintf(os.Stderr, "[config] parse remote config failed: %v\n", err)
	} else {
		cfg = newCfg
		fmt.Fprintf(os.Stderr, "[config] loaded from %s\n", provider.Name())
	}

	// 热更新监听
	if err := provider.Watch(ctx, func(data []byte) {
		if _, err := config.LoadRemote(data); err != nil {
			fmt.Fprintf(os.Stderr, "[config] reload failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[config] reloaded from %s\n", provider.Name())
		}
	}); err != nil {
		return nil, nil, fmt.Errorf("watch config center: %w", err)
	}

	return provider, cfg, nil
}

// seedAdmin 空表时创建内置管理员 admin / admin123（仅首次）
func seedAdmin(ctx context.Context, repo *repository.UserRepository) error {
	n, err := repo.CountAll(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hashed, err := service.HashPassword("admin123")
	if err != nil {
		return err
	}
	return repo.Create(ctx, &model.User{
		Username: "admin",
		Password: hashed,
		Nickname: "Administrator",
		Role:     "admin",
	})
}

// newDemoConsumer 订阅 demo topic 并打印消息（业务接入示例）
func newDemoConsumer(cfg *config.Config, l *zap.Logger) (*mqpkg.Consumer, error) {
	c, err := mqpkg.NewPushConsumer(mqpkg.ConsumerOptions{
		Endpoints: cfg.RocketMQ.Endpoints,
		Group:     cfg.RocketMQ.Consumer.Group,
	})
	if err != nil {
		return nil, err
	}

	handler := func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, m := range msgs {
			l.Info("mq message received",
				zap.String("topic", m.Topic),
				zap.String("tag", string(m.GetTags())),
				zap.ByteString("body", m.Body),
			)
		}
		return consumer.ConsumeSuccess, nil
	}

	if err := c.Subscribe(cfg.RocketMQ.Consumer.Topic, consumer.MessageSelector{}, handler); err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	return c, nil
}
