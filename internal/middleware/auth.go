// Package middleware Gin 中间件集合。
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/pkg/errcode"
	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
	"github.com/kenneth/go-boilerplate/pkg/response"
)

// 上下文键：认证通过后写入，供 handler / 后续中间件取用
const (
	CtxKeyUserID   = "user_id"
	CtxKeyUsername = "username"
	CtxKeyRole     = "role"
)

// JWTAuth 校验 Authorization: Bearer <access_token>。
// 通过后将 user_id / username / role 写入 gin.Context。
func JWTAuth(opts jwtpkg.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(401, response.Body{Code: errcode.ErrUnauthorized.Code, Message: "缺少 Bearer 令牌"})
			return
		}

		claims, err := jwtpkg.ParseToken(opts, token)
		if err != nil {
			c.AbortWithStatusJSON(401, response.Body{Code: errcode.ErrUnauthorized.Code, Message: err.Error()})
			return
		}
		if claims.TokenType != jwtpkg.TokenTypeAccess {
			c.AbortWithStatusJSON(401, response.Body{Code: errcode.ErrUnauthorized.Code, Message: "需要访问令牌而非刷新令牌"})
			return
		}

		c.Set(CtxKeyUserID, claims.UserID)
		c.Set(CtxKeyUsername, claims.Username)
		c.Set(CtxKeyRole, claims.Role)
		c.Next()
	}
}
