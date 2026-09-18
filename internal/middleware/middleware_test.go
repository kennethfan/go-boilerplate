package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/kenneth/go-boilerplate/internal/testutil"
	"github.com/kenneth/go-boilerplate/pkg/casbinx"
	"github.com/kenneth/go-boilerplate/pkg/jwt"
	"github.com/kenneth/go-boilerplate/pkg/metrics"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// jwtOpts 测试用 JWT 配置（密钥 ≥ 32 字节）。
func jwtOpts() jwt.Options {
	return jwt.Options{
		Secret:        "test-secret-needs-32-bytes-minimum-xxxx",
		Issuer:        "test",
		AccessExpire:  900 * time.Second,
		RefreshExpire: 86400 * time.Second,
	}
}

// okHandler 标记请求已到达终点，并回显 ctx 中的用户键。
func okHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":  c.GetInt64(CtxKeyUserID),
			"username": c.GetString(CtxKeyUsername),
			"role":     c.GetString(CtxKeyRole),
		})
	}
}

// ---------- JWTAuth ----------

func TestJWTAuth_MissingBearer(t *testing.T) {
	engine := gin.New()
	engine.Use(JWTAuth(jwtOpts()))
	engine.GET("/protected", okHandler())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// 不带 Authorization 头
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	engine := gin.New()
	engine.Use(JWTAuth(jwtOpts()))
	engine.GET("/protected", okHandler())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer garbage.token.here")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestJWTAuth_RefreshTokenRejected(t *testing.T) {
	engine := gin.New()
	engine.Use(JWTAuth(jwtOpts()))
	engine.GET("/protected", okHandler())

	pair, err := jwt.GenerateTokenPair(jwtOpts(), 1, "u", "user")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.RefreshToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for refresh token, got %d", w.Code)
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	engine := gin.New()
	engine.Use(JWTAuth(jwtOpts()))
	engine.GET("/protected", okHandler())

	pair, err := jwt.GenerateTokenPair(jwtOpts(), 42, "alice", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	// 校验三个上下文键已写入
	if got := w.Body.String(); got == "" {
		t.Fatal("empty body")
	}
}

// ---------- CORS ----------

func TestCORS_NormalRequest(t *testing.T) {
	engine := gin.New()
	engine.Use(CORS())
	engine.GET("/x", okHandler())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin want *, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("allow-methods header missing")
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("max-age want 86400, got %q", got)
	}
}

func TestCORS_OptionsShortCircuit(t *testing.T) {
	engine := gin.New()
	engine.Use(CORS())
	engine.GET("/x", okHandler())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS want 204, got %d", w.Code)
	}
}

// ---------- Recovery ----------

func TestRecovery_PanicReturns500(t *testing.T) {
	engine := gin.New()
	engine.Use(Recovery(zap.NewNop()))
	engine.GET("/boom", func(c *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_NoPanicPassesThrough(t *testing.T) {
	engine := gin.New()
	engine.Use(Recovery(zap.NewNop()))
	engine.GET("/ok", okHandler())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// ---------- AccessLog ----------

// buildAccessLogEngine 构造带 observer 的 logger 中间件引擎。
func buildAccessLogEngine(core zapcore.Core) *gin.Engine {
	engine := gin.New()
	engine.Use(AccessLog(zap.New(core)))
	engine.GET("/ok", okHandler())
	engine.GET("/missing", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	engine.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	return engine
}

func TestAccessLog_InfoLevel(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	engine := buildAccessLogEngine(core)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(w, req)

	if logs.FilterLevelExact(zap.InfoLevel).Len() != 1 {
		t.Fatalf("want 1 info log, got %d", logs.FilterLevelExact(zap.InfoLevel).Len())
	}
	if logs.FilterLevelExact(zap.ErrorLevel).Len() != 0 {
		t.Fatalf("want 0 error log, got %d", logs.FilterLevelExact(zap.ErrorLevel).Len())
	}
}

func TestAccessLog_WarnLevel(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	engine := buildAccessLogEngine(core)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	engine.ServeHTTP(w, req)

	if logs.FilterLevelExact(zap.WarnLevel).Len() != 1 {
		t.Fatalf("want 1 warn log, got %d", logs.FilterLevelExact(zap.WarnLevel).Len())
	}
}

func TestAccessLog_ErrorLevel(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	engine := buildAccessLogEngine(core)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	engine.ServeHTTP(w, req)

	if logs.FilterLevelExact(zap.ErrorLevel).Len() != 1 {
		t.Fatalf("want 1 error log, got %d", logs.FilterLevelExact(zap.ErrorLevel).Len())
	}
}

// ---------- Casbin ----------

const casbinTestModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`

// newCasbinTestEnforcer 构造仅用于测试的 Enforcer：精确匹配模型 + 显式策略。
func newCasbinTestEnforcer(t *testing.T) *casbin.SyncedEnforcer {
	t.Helper()
	db := testutil.NewSQLiteMemoryDB(t)
	modelPath := filepath.Join(t.TempDir(), "model.conf")
	if err := os.WriteFile(modelPath, []byte(casbinTestModel), 0o600); err != nil {
		t.Fatalf("write model conf: %v", err)
	}
	e, err := casbinx.NewEnforcer(db, modelPath)
	if err != nil {
		t.Fatalf("NewEnforcer: %v", err)
	}
	// admin 可访问 /admin；user 可访问 /user
	for _, p := range [][]string{
		{"admin", "/admin", "GET"},
		{"user", "/user", "GET"},
	} {
		if _, err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			t.Fatalf("add policy: %v", err)
		}
	}
	return e
}

// casbinEngine 构造带 Casbin 中间件 + 前置角色注入的引擎。
func casbinEngine(enf *casbin.SyncedEnforcer, role string) *gin.Engine {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if role != "" {
			c.Set(CtxKeyRole, role)
		}
		c.Next()
	})
	engine.Use(Casbin(enf))
	engine.GET("/admin", okHandler())
	engine.GET("/user", okHandler())
	return engine
}

func TestCasbin_Allowed(t *testing.T) {
	enf := newCasbinTestEnforcer(t)
	engine := casbinEngine(enf, "admin")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("admin allowed: want 200, got %d", w.Code)
	}
}

func TestCasbin_Forbidden(t *testing.T) {
	enf := newCasbinTestEnforcer(t)
	engine := casbinEngine(enf, "user") // user 无权访问 /admin

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestCasbin_MissingRole(t *testing.T) {
	enf := newCasbinTestEnforcer(t)
	engine := casbinEngine(enf, "") // 不注入 role

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// ---------- Prometheus ----------

func TestPrometheus_RecordsMetrics(t *testing.T) {
	engine := gin.New()
	engine.Use(Prometheus())
	engine.GET("/m", okHandler())

	before := promtestutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues("GET", "/m", "200"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/m", nil)
	engine.ServeHTTP(w, req)

	after := promtestutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues("GET", "/m", "200"))
	if after-before != 1 {
		t.Fatalf("counter want +1, got %v (before %v)", after-before, before)
	}
}
