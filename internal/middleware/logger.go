package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// AccessLog 结构化访问日志中间件。
// 记录 method / path / status / latency / ip / trace_id，需置于 otelgin 之后以拿到 trace_id。
func AccessLog(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		traceID := ""
		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.HasTraceID() {
			traceID = sc.TraceID().String()
		}

		fields := []zap.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.Int("body_size", c.Writer.Size()),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("trace_id", traceID),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.Strings("errors", c.Errors.Errors()))
		}

		// 5xx 记 error，4xx 记 warn，其余 info
		switch {
		case c.Writer.Status() >= 500:
			l.Error("access", fields...)
		case c.Writer.Status() >= 400:
			l.Warn("access", fields...)
		default:
			l.Info("access", fields...)
		}
	}
}
