package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/pkg/sse"
)

// SSEHandler Server-Sent Events 接口
type SSEHandler struct{}

// NewSSEHandler 构造
func NewSSEHandler() *SSEHandler { return &SSEHandler{} }

// Serve godoc
// @Summary      SSE 服务器推送
// @Description  建立 SSE 连接，每 2 秒推送一条服务器时间。
// @Tags         realtime
// @Produce      text/event-stream
// @Success      200
// @Router       /sse [get]
func (h *SSEHandler) Serve(c *gin.Context) {
	done := sse.Init(c)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	seq := 0
	for {
		select {
		case <-done:
			return // 客户端断开
		case t := <-ticker.C:
			seq++
			sse.Send(c, fmt.Sprintf(`{"seq": %d, "time": %q}`, seq, t.Format(time.RFC3339)))
		}
	}
}
