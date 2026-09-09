package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/pkg/metrics"
)

// Prometheus 收集 HTTP 请求指标（QPS / 延迟 / 状态码 / 请求体大小）。
// 路径使用路由模板（如 /api/v1/users/:id），避免高基数。
func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath() // 路由模板；未匹配时为空，回退到原始路径
		if path == "" {
			path = c.Request.URL.Path
		}

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path, status).
			Observe(time.Since(start).Seconds())
		metrics.HTTPRequestBodyBytes.WithLabelValues(method, path).
			Observe(float64(c.Request.ContentLength))
	}
}
