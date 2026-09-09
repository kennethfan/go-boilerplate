// Package ws 提供基于 gorilla/websocket 的 WebSocket 连接封装与连接管理（Hub）。
// 支持广播、单发、心跳保活与优雅断开。
package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Upgrader WebSocket 升级器（生产可按需收紧 CheckOrigin）
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const (
	writeWait      = 10 * time.Second // 写超时
	pongWait       = 60 * time.Second // 等待 pong 超时
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Client 单个 WebSocket 连接
type Client struct {
	conn *websocket.Conn
	send chan []byte
	hub  *Hub
}

// Send 向该连接发送消息（非阻塞，缓冲满则丢弃）
func (c *Client) Send(msg []byte) {
	select {
	case c.send <- msg:
	default:
	}
}

// Close 关闭连接
func (c *Client) Close() {
	c.conn.Close()
}

// readPump 读消息并广播到 Hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.hub.broadcast <- msg // echo：收到的消息广播给所有连接
	}
}

// writePump 写消息 + 心跳 ping
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
