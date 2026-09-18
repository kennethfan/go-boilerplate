package casbinx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/kenneth/go-boilerplate/internal/testutil"
)

const rbacModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) || r.sub == p.sub && r.obj == p.obj && r.act == p.act
`

// newTestEnforcer 用临时 model 文件 + :memory: SQLite 构造 Enforcer（覆盖 NewEnforcer 路径）。
func newTestEnforcer(t *testing.T) *casbin.SyncedEnforcer {
	t.Helper()
	db := testutil.NewSQLiteMemoryDB(t)
	modelPath := filepath.Join(t.TempDir(), "model.conf")
	if err := os.WriteFile(modelPath, []byte(rbacModel), 0o600); err != nil {
		t.Fatalf("write model conf: %v", err)
	}
	e, err := NewEnforcer(db, modelPath)
	if err != nil {
		t.Fatalf("NewEnforcer: %v", err)
	}
	return e
}

func TestNewEnforcer_BadModelPath(t *testing.T) {
	db := testutil.NewSQLiteMemoryDB(t)
	// 不存在的 model 文件 → NewEnforcer 必须返回 error（覆盖错误分支）
	if _, err := NewEnforcer(db, "/nonexistent/model.conf"); err == nil {
		t.Fatal("expected error for bad model path")
	}
}

func TestSeedPolicies_Idempotent(t *testing.T) {
	e := newTestEnforcer(t)
	if err := SeedPolicies(e); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, p := range [][]string{
		{"admin", "/api/v1/*", "*"},
		{"user", "/api/v1/users/:id", "GET"},
	} {
		ok, err := e.HasPolicy(p[0], p[1], p[2])
		if err != nil {
			t.Fatalf("has policy %v: %v", p, err)
		}
		if !ok {
			t.Errorf("policy missing: %v", p)
		}
	}

	// 幂等：再 seed 不应新增策略
	if err := SeedPolicies(e); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	policies, err := e.GetPolicy()
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if len(policies) != 2 {
		t.Errorf("policy count want 2, got %d", len(policies))
	}
}
