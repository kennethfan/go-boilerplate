// Package casbinx 提供 Casbin SyncedEnforcer 初始化。
// 策略（policy）持久化在 MySQL 的 casbin_rule 表（gorm-adapter 自动建表）。
package casbinx

import (
	"fmt"

	"github.com/casbin/casbin/v3"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// NewEnforcer 创建基于 MySQL 的同步 Enforcer 并加载已有策略
func NewEnforcer(db *gorm.DB, modelPath string) (*casbin.SyncedEnforcer, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("casbinx: new adapter failed: %w", err)
	}

	e, err := casbin.NewSyncedEnforcer(modelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("casbinx: new enforcer failed: %w", err)
	}

	if err := e.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("casbinx: load policy failed: %w", err)
	}
	// 策略变更自动写回 DB
	e.EnableAutoSave(true)
	return e, nil
}

// SeedPolicies 初始化内置角色策略（仅首次生效，幂等）。
// admin：全部接口；user：只读用户信息。
func SeedPolicies(e *casbin.SyncedEnforcer) error {
	policies := [][]string{
		{"admin", "/api/v1/*", "*"},
		{"user", "/api/v1/users/:id", "GET"},
	}
	for _, p := range policies {
		exists, err := e.HasPolicy(p[0], p[1], p[2])
		if err != nil {
			return fmt.Errorf("casbinx: check policy failed: %w", err)
		}
		if !exists {
			if _, err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
				return fmt.Errorf("casbinx: add policy failed: %w", err)
			}
		}
	}
	return nil
}
