package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/kenneth/go-boilerplate/internal/repository"
	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/internal/testutil"
	"github.com/kenneth/go-boilerplate/pkg/jwt"
	wspkg "github.com/kenneth/go-boilerplate/pkg/ws"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// jwtOpts 测试用 JWT 配置（密钥 ≥ 32 字节，有效期用 time.Duration）。
func jwtOpts() jwt.Options {
	return jwt.Options{
		Secret:        "test-secret-needs-32-bytes-minimum-xxxx",
		Issuer:        "test",
		AccessExpire:  900 * time.Second,
		RefreshExpire: 86400 * time.Second,
	}
}

// deadRedis 指向几乎不可能监听的地址，使 Ping 必然失败（用于测试 Ready 的 redis-down 分支）。
func deadRedis() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
}

// setupEngine 用最小路由装配各 handler（等价于 router 的 /api/v1 子树），便于端到端验证绑定与状态码。
func setupEngine(t *testing.T) (*gin.Engine, *service.UserService, *service.AuthService) {
	t.Helper()
	db := testutil.NewSQLiteMemory(t)
	repo := repository.NewUserRepository(db)
	userSvc := service.NewUserService(repo)
	authSvc := service.NewAuthService(repo, jwtOpts())
	uh := NewUserHandler(userSvc)
	ah := NewAuthHandler(authSvc)
	hh := NewHealthHandler(db, deadRedis())

	r := gin.New()
	r.POST("/api/v1/auth/login", ah.Login)
	r.POST("/api/v1/auth/refresh", ah.Refresh)
	r.GET("/api/v1/healthz", hh.Healthz)
	r.GET("/api/v1/readyz", hh.Ready)

	users := r.Group("/api/v1/users")
	{
		users.GET("", uh.ListUsers)
		users.POST("", uh.CreateUser)
		users.GET("/:id", uh.GetUser)
		users.PUT("/:id", uh.UpdateUser)
		users.DELETE("/:id", uh.DeleteUser)
	}
	return r, userSvc, authSvc
}

// seedUser 直接通过 service 创建测试用户（绕过 HTTP 便于构造前置数据）。
func seedUser(t *testing.T, svc *service.UserService, username, password, role string) {
	t.Helper()
	if _, err := svc.Create(context.Background(), service.CreateUserRequest{
		Username: username,
		Password: password,
		Role:     role,
	}); err != nil {
		t.Fatalf("seed user %q: %v", username, err)
	}
}

func jsonBody(v any) *bytes.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (raw=%s)", err, w.Body.String())
	}
	return body
}

type userData struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
}

// ---------- UserHandler ----------

func TestUserHandler_CreateUser_Success(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", jsonBody(map[string]any{
		"username": "bob", "password": "secret1", "nickname": "Bob", "role": "user",
	}))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	data, _ := body["data"].(map[string]any)
	if data["username"] != "bob" || data["role"] != "user" {
		t.Errorf("unexpected user data: %v", data)
	}
}

func TestUserHandler_CreateUser_Duplicate(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", jsonBody(map[string]any{
		"username": "bob", "password": "secret1", "role": "user",
	}))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate want 400, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestUserHandler_CreateUser_InvalidJSON(t *testing.T) {
	r, _, _ := setupEngine(t)

	// 缺必填 username / role
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", jsonBody(map[string]any{
		"password": "secret1",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing field want 400, got %d", w.Code)
	}
}

func TestUserHandler_CreateUser_InvalidRole(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", jsonBody(map[string]any{
		"username": "bob", "password": "secret1", "role": "super",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad role want 400, got %d", w.Code)
	}
}

func TestUserHandler_GetUser_Success(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	// seed 后自增 id 从 1 起
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if data["username"] != "bob" {
		t.Errorf("unexpected user: %v", data)
	}
}

func TestUserHandler_GetUser_NotFound(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/999", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("not found want 404, got %d", w.Code)
	}
}

func TestUserHandler_GetUser_InvalidID(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id want 400, got %d", w.Code)
	}
}

func TestUserHandler_ListUsers(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "u1", "secret1", "user")
	seedUser(t, userSvc, "u2", "secret1", "user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=1&size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if int(data["total"].(float64)) < 2 {
		t.Errorf("total want >=2, got %v", data["total"])
	}
}

func TestUserHandler_ListUsers_InvalidQuery(t *testing.T) {
	r, _, _ := setupEngine(t)

	// size 超过 max=100 → 绑定失败
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?size=200", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized size want 400, got %d", w.Code)
	}
}

func TestUserHandler_UpdateUser_Success(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", jsonBody(map[string]any{
		"nickname": "Bobby",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	// 验证生效
	gw := httptest.NewRecorder()
	greq := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	r.ServeHTTP(gw, greq)
	gbody := decodeBody(t, gw)
	gdata := gbody["data"].(map[string]any)
	if gdata["nickname"] != "Bobby" {
		t.Errorf("nickname not updated: %v", gdata)
	}
}

func TestUserHandler_UpdateUser_InvalidID(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/abc", jsonBody(map[string]any{
		"nickname": "x",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id want 400, got %d", w.Code)
	}
}

func TestUserHandler_DeleteUser_Success(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// 删除后查不到
	gw := httptest.NewRecorder()
	greq := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	r.ServeHTTP(gw, greq)
	if gw.Code != http.StatusNotFound {
		t.Errorf("after delete want 404, got %d", gw.Code)
	}
}

func TestUserHandler_DeleteUser_InvalidID(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id want 400, got %d", w.Code)
	}
}

// ---------- AuthHandler ----------

func TestAuthHandler_Login_Success(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", jsonBody(map[string]any{
		"username": "bob", "password": "secret1",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if data["access_token"] == nil || data["refresh_token"] == nil {
		t.Errorf("missing tokens: %v", data)
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", jsonBody(map[string]any{
		"username": "bob", "password": "wrongpw",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password want 401, got %d", w.Code)
	}
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", jsonBody(map[string]any{
		"username": "ghost", "password": "whatever",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("nonexistent user want 401, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", jsonBody(map[string]any{
		"username": "bob", // 缺 password
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid json want 400, got %d", w.Code)
	}
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	// 登录拿 refresh token
	lw := httptest.NewRecorder()
	lreq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", jsonBody(map[string]any{
		"username": "bob", "password": "secret1",
	}))
	r.ServeHTTP(lw, lreq)
	ldata := decodeBody(t, lw)["data"].(map[string]any)
	refresh := ldata["refresh_token"].(string)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBody(map[string]any{
		"refresh_token": refresh,
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["data"].(map[string]any)["access_token"] == nil {
		t.Error("refresh did not return new token pair")
	}
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBody(map[string]any{
		"refresh_token": "not-a-valid-token",
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid refresh want 401, got %d", w.Code)
	}
}

func TestAuthHandler_Refresh_AccessTokenRejected(t *testing.T) {
	r, userSvc, _ := setupEngine(t)
	seedUser(t, userSvc, "bob", "secret1", "user")

	lw := httptest.NewRecorder()
	lreq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", jsonBody(map[string]any{
		"username": "bob", "password": "secret1",
	}))
	r.ServeHTTP(lw, lreq)
	ldata := decodeBody(t, lw)["data"].(map[string]any)
	access := ldata["access_token"].(string)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBody(map[string]any{
		"refresh_token": access, // 用 access 当 refresh
	}))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("access-as-refresh want 401, got %d", w.Code)
	}
}

// ---------- HealthHandler ----------

func TestHealthHandler_Healthz(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["data"].(map[string]any)["status"] != "ok" {
		t.Errorf("unexpected healthz body: %v", body)
	}
}

func TestHealthHandler_Ready_RedisDown(t *testing.T) {
	r, _, _ := setupEngine(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
	r.ServeHTTP(w, req)
	// sqlite(mysql) 正常 + redis(死地址) 失败 → 503
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 (redis down), got %d (body=%s)", w.Code, w.Body.String())
	}
}

// ---------- Realtime handlers (ws / sse) ----------

func TestWSHandler_Serve_NoToken(t *testing.T) {
	r := gin.New()
	r.GET("/api/v1/ws", NewWSHandler(wspkg.NewHub(), jwtOpts()).Serve)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token want 401, got %d", w.Code)
	}
}

func TestWSHandler_Serve_InvalidToken(t *testing.T) {
	r := gin.New()
	r.GET("/api/v1/ws", NewWSHandler(wspkg.NewHub(), jwtOpts()).Serve)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token=garbage", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token want 401, got %d", w.Code)
	}
}

func TestWSHandler_Serve_ValidToken_UpgradeFails(t *testing.T) {
	// httptest.ResponseRecorder 不支持 Hijack，WebSocket 升级必然失败 → 400
	pair, err := jwt.GenerateTokenPair(jwtOpts(), 1, "bob", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	r := gin.New()
	r.GET("/api/v1/ws", NewWSHandler(wspkg.NewHub(), jwtOpts()).Serve)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+pair.AccessToken, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("upgrade fail want 400, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestSSEHandler_Serve(t *testing.T) {
	r := gin.New()
	r.GET("/api/v1/sse", NewSSEHandler().Serve)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(done)
	}()

	// 等待首个 ticker 事件（2s）被发送
	time.Sleep(2300 * time.Millisecond)
	cancel() // 触发客户端断开 → handler 退出循环

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sse handler did not return after disconnect")
	}

	body := w.Body.String()
	if !strings.Contains(body, "data:") {
		t.Errorf("expected SSE data payload, got %q", body)
	}
}
