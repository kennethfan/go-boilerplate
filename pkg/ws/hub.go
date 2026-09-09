package ws

import (
	"net/http"
)

// Hub 维护所有活跃连接，处理注册 / 注销 / 广播。
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
}

// NewHub 创建并启动 Hub 主循环
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
	go h.run()
	return h
}

// run Hub 主循环：串行处理注册/注销/广播，避免并发读写 map
func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default: // 缓冲满，视为慢连接，移除
					delete(h.clients, c)
					close(c.send)
				}
			}
		}
	}
}

// Register 升级 HTTP 连接为 WebSocket 并注册到 Hub，启动读写协程。
func (h *Hub) Register(w http.ResponseWriter, r *http.Request) error {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	c := &Client{conn: conn, send: make(chan []byte, 256), hub: h}
	h.register <- c
	go c.writePump()
	go c.readPump()
	return nil
}

// Broadcast 向所有连接广播消息
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

// Len 当前活跃连接数
func (h *Hub) Len() int {
	return len(h.clients)
}
