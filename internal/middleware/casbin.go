package middleware

import (
	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/pkg/response"
)

// Casbin RBAC 授权中间件。
// 依赖 JWTAuth 已写入 role；对 (role, path, method) 做 enforce 判定。
func Casbin(enforcer *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(CtxKeyRole)
		if !exists {
			response.Unauthorized(c)
			c.Abort()
			return
		}

		obj := c.Request.URL.Path // 资源：API 路径
		act := c.Request.Method   // 动作：HTTP 方法

		ok, err := enforcer.Enforce(role, obj, act)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}
		if !ok {
			response.Forbidden(c)
			c.Abort()
			return
		}
		c.Next()
	}
}
