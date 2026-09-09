package router_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/internal/handler"
	"github.com/kenneth/go-boilerplate/internal/router"
	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
)

// newRouter 给定 toggles 构造一个最小 router（handler 字段不依赖 DB，只为通过路由注册阶段）
func newRouter(toggles router.Toggles) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return router.New(router.Dependencies{
		Mode:    gin.TestMode,
		Toggles: toggles,
		JWTOpts: jwtpkg.Options{},
		// 零值 handler 指针：仅用于方法注册，不命中请求，避免 nil receiver panic
		AuthHandler:   &handler.AuthHandler{},
		UserHandler:   &handler.UserHandler{},
		HealthHandler: &handler.HealthHandler{},
		WSHandler:     &handler.WSHandler{},
		SSEHandler:    &handler.SSEHandler{},
	})
}

func routeSet(r *gin.Engine) map[string]bool {
	paths := map[string]bool{}
	for _, ri := range r.Routes() {
		paths[ri.Method+" "+ri.Path] = true
	}
	return paths
}

// TestRouter_TogglesAllOn 全开：metrics/docs/realtime 全部挂载，health 公开
func TestRouter_TogglesAllOn(t *testing.T) {
	r := newRouter(router.Toggles{Metrics: true, Auth: true, Realtime: true, Docs: true})
	got := routeSet(r)

	for _, want := range []string{
		"GET /metrics",
		"GET /swagger/*any",
		"GET /api/v1/ws",
		"GET /api/v1/sse",
		"GET /api/v1/healthz",
		"GET /api/v1/readyz",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/refresh",
		"GET /api/v1/users",
		"POST /api/v1/users",
	} {
		if !got[want] {
			t.Errorf("all-on 缺失路由: %s", want)
		}
	}
}

// TestRouter_TogglesAllOff 全关：metrics/docs/realtime 全不挂；业务路由仍存在（仅无中间件）
func TestRouter_TogglesAllOff(t *testing.T) {
	r := newRouter(router.Toggles{})
	got := routeSet(r)

	for _, missing := range []string{
		"GET /metrics",
		"GET /swagger/*any",
		"GET /api/v1/ws",
		"GET /api/v1/sse",
	} {
		if got[missing] {
			t.Errorf("all-off 不应挂载: %s", missing)
		}
	}

	// 公开路径必须保留
	for _, keep := range []string{
		"GET /api/v1/healthz",
		"GET /api/v1/readyz",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/refresh",
		"GET /api/v1/users",
	} {
		if !got[keep] {
			t.Errorf("all-off 应保留公开路径: %s", keep)
		}
	}
}

// TestRouter_EachToggle 单点验证每个 toggle 各自独立
func TestRouter_EachToggle(t *testing.T) {
	cases := []struct {
		name     string
		toggle   router.Toggles
		mount    string
		noMount  string
	}{
		{"metrics only off", router.Toggles{Auth: true, Realtime: true, Docs: true}, "", "GET /metrics"},
		{"docs only off", router.Toggles{Metrics: true, Auth: true, Realtime: true}, "", "GET /swagger/*any"},
		{"realtime only off", router.Toggles{Metrics: true, Auth: true, Docs: true}, "", "GET /api/v1/ws"},
		{"metrics and ws both", router.Toggles{Metrics: true, Auth: true, Realtime: true, Docs: true}, "GET /api/v1/ws", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRouter(tc.toggle)
			got := routeSet(r)
			if tc.mount != "" && !got[tc.mount] {
				t.Errorf("%s: 期望挂载 %s", tc.name, tc.mount)
			}
			if tc.noMount != "" && got[tc.noMount] {
				t.Errorf("%s: 不应挂载 %s", tc.name, tc.noMount)
			}
		})
	}
}
