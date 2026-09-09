package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery panic 恢复中间件：记录堆栈到日志（含 trace_id），返回统一 500。
func Recovery(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger := l.With(
					zap.Any("panic", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("stack", string(debug.Stack())),
				)
				// 若请求已有 trace 上下文，一并带上
				logger.Error("panic recovered")

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    10005,
					"message": "服务器内部错误",
				})
			}
		}()
		c.Next()
	}
}
