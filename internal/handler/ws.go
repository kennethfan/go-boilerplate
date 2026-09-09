package handler

import (
	"github.com/gin-gonic/gin"

	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
	"github.com/kenneth/go-boilerplate/pkg/response"
	"github.com/kenneth/go-boilerplate/pkg/ws"
)

// WSHandler WebSocket 接口
type WSHandler struct {
	hub     *ws.Hub
	jwtOpts jwtpkg.Options
}

// NewWSHandler 构造
func NewWSHandler(hub *ws.Hub, jwtOpts jwtpkg.Options) *WSHandler {
	return &WSHandler{hub: hub, jwtOpts: jwtOpts}
}

// Serve godoc
// @Summary      WebSocket 连接
// @Description  建立 WebSocket 连接，收到的消息广播给所有连接。浏览器无法携带 Authorization 头，鉴权用 query 参数 token。
// @Tags         realtime
// @Param        token  query    string  true  "JWT access_token"
// @Success      101
// @Router       /ws [get]
func (h *WSHandler) Serve(c *gin.Context) {
	// 鉴权：浏览器 WebSocket 无法带 Authorization 头，用 query 参数
	token := c.Query("token")
	if token == "" {
		response.Unauthorized(c)
		return
	}
	if _, err := jwtpkg.ParseToken(h.jwtOpts, token); err != nil {
		response.Unauthorized(c)
		return
	}

	// 升级并注册连接（后续收发在独立协程）
	if err := h.hub.Register(c.Writer, c.Request); err != nil {
		c.JSON(400, gin.H{"code": 10001, "message": "websocket upgrade failed"})
	}
}
