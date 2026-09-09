// Package router 路由注册与中间件装配。
package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"

	"github.com/kenneth/go-boilerplate/internal/handler"
	"github.com/kenneth/go-boilerplate/internal/middleware"
	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
	"github.com/kenneth/go-boilerplate/pkg/metrics"
)

// Toggles 组件开关集合（与 config.Config 同名段位语义一致）
type Toggles struct {
	Metrics  bool // Prometheus HTTP 中间件 + /metrics 端点
	Auth     bool // JWTAuth + Casbin 中间件
	Realtime bool // WS / SSE 路由
	Docs     bool // Swagger 路由
}

// Dependencies 路由层依赖
type Dependencies struct {
	Mode          string // gin mode: debug / release
	Toggles       Toggles
	JWTOpts       jwtpkg.Options
	Enforcer      *casbin.SyncedEnforcer
	Logger        *zap.Logger
	AuthHandler   *handler.AuthHandler
	UserHandler   *handler.UserHandler
	HealthHandler *handler.HealthHandler
	WSHandler     *handler.WSHandler
	SSEHandler    *handler.SSEHandler
}

// New 构建 gin.Engine。
// 中间件顺序：otelgin（生成 trace）→ recovery → （metrics）→ accessLog → cors
// 认证路由：/api/v1 下 JWTAuth + Casbin（受 auth.enable 控制）
func New(d Dependencies) *gin.Engine {
	gin.SetMode(d.Mode)
	r := gin.New()

	// 必备中间件
	common := []gin.HandlerFunc{
		otelgin.Middleware("go-boilerplate"), // HTTP 入口 span，生成 trace_id
		middleware.Recovery(d.Logger),
		middleware.AccessLog(d.Logger),
		middleware.CORS(),
	}
	// 可选中间件：Prometheus HTTP 指标
	if d.Toggles.Metrics {
		common = append(common, middleware.Prometheus())
	}
	r.Use(common...)

	// 指标暴露端点（Prometheus 抓取）
	if d.Toggles.Metrics {
		r.GET("/metrics", gin.WrapH(metrics.Handler()))
	}

	// Swagger 文档（需先执行 make swagger 生成 api/docs）
	if d.Toggles.Docs {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api/v1")
	{
		// 公开接口
		health := api.Group("")
		{
			health.GET("/healthz", d.HealthHandler.Healthz)
			health.GET("/readyz", d.HealthHandler.Ready)
		}

		// 登录 / 刷新：始终公开（认证关闭后不再可用，但路径保留便于按需启用）
		auth := api.Group("/auth")
		{
			auth.POST("/login", d.AuthHandler.Login)
			auth.POST("/refresh", d.AuthHandler.Refresh)
		}

		// 实时通信（WS 内部做 query token 鉴权；SSE 公开）
		if d.Toggles.Realtime {
			realtime := api.Group("")
			{
				realtime.GET("/ws", d.WSHandler.Serve)
				realtime.GET("/sse", d.SSEHandler.Serve)
			}
		}

		// 认证 + 授权接口
		users := api.Group("/users")
		if d.Toggles.Auth {
			users.Use(middleware.JWTAuth(d.JWTOpts), middleware.Casbin(d.Enforcer))
		}
		{
			users.GET("", d.UserHandler.ListUsers)
			users.POST("", d.UserHandler.CreateUser)
			users.GET("/:id", d.UserHandler.GetUser)
			users.PUT("/:id", d.UserHandler.UpdateUser)
			users.DELETE("/:id", d.UserHandler.DeleteUser)
		}
	}

	return r
}
