// Package sse 提供 Server-Sent Events（SSE）服务端推送封装。
// 基于 HTTP 流式响应，单向推送，客户端自动重连。
package sse

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Event 一个 SSE 事件
type Event struct {
	ID    string        // 事件 ID（用于断线重连 Last-Event-ID）
	Event string        // 事件名（默认 message）
	Data  string        // 数据（支持多行，按 \n 拆分）
	Retry time.Duration // 重连间隔（毫秒）
}

// Write 向 gin.Context 写入一个 SSE 事件并立即 flush
func (e Event) Write(c *gin.Context) {
	w := c.Writer
	if e.ID != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", e.ID)
	}
	if e.Event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", e.Event)
	}
	for _, line := range strings.Split(e.Data, "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	if e.Retry > 0 {
		_, _ = fmt.Fprintf(w, "retry: %d\n", e.Retry.Milliseconds())
	}
	_, _ = fmt.Fprint(w, "\n")
	w.Flush()
}

// Init 设置 SSE 响应头，返回一个在客户端断开时关闭的 channel。
// 调用方应在 select 中监听该 channel 以结束推送循环。
func Init(c *gin.Context) <-chan struct{} {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // 关闭 nginx 缓冲
	c.Status(200)
	c.Writer.Flush()

	done := make(chan struct{})
	go func() {
		<-c.Request.Context().Done()
		close(done)
	}()
	return done
}

// Send 便捷发送一个默认事件（event=message）
func Send(c *gin.Context, data string) {
	Event{Data: data}.Write(c)
}
