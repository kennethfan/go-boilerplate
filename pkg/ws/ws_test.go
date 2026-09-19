package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestHub_RegisterUnregisterBroadcast 直接驱动 Hub 主循环（包内可访问未导出 channel）
func TestHub_RegisterUnregisterBroadcast(t *testing.T) {
	h := NewHub()

	c1 := &Client{send: make(chan []byte, 8), hub: h}
	h.register <- c1

	if h.Len() != 1 {
		t.Fatalf("after register Len want 1, got %d", h.Len())
	}

	// 广播应送达 c1.send
	h.broadcast <- []byte("broadcast-msg")
	select {
	case got := <-c1.send:
		if string(got) != "broadcast-msg" {
			t.Errorf("broadcast got %q", string(got))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast did not reach client")
	}

	// 注销应关闭 c1.send
	h.unregister <- c1
	select {
	case _, ok := <-c1.send:
		if ok {
			t.Error("c1.send should be closed after unregister")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("unregister did not close client send channel")
	}
	if h.Len() != 0 {
		t.Errorf("after unregister Len want 0, got %d", h.Len())
	}

	// 向空 Hub 广播不应 panic
	h.broadcast <- []byte("orphan")
	time.Sleep(50 * time.Millisecond)
}

// TestClient_Send 验证 Send 非阻塞写入缓冲
func TestClient_Send(t *testing.T) {
	c := &Client{send: make(chan []byte, 1)}
	c.Send([]byte("a"))
	select {
	case got := <-c.send:
		if string(got) != "a" {
			t.Errorf("Send got %q", string(got))
		}
	default:
		t.Fatal("Send should have queued message")
	}
}

// TestClient_Send_DropWhenFull 验证缓冲满时 Send 丢弃（default 分支）
func TestClient_Send_DropWhenFull(t *testing.T) {
	c := &Client{send: make(chan []byte, 1)}
	c.send <- []byte("full") // 占满缓冲
	c.Send([]byte("dropped")) // 不应阻塞

	if len(c.send) != 1 {
		t.Errorf("buffer should still hold 1 (drop branch), got %d", len(c.send))
	}
	if string(<-c.send) != "full" {
		t.Error("original message should remain")
	}
}

// TestRegister_ReadWritePumpEcho 真实 websocket 往返，覆盖 Register/readPump/writePump。
// 客户端发消息 → 服务端 readPump 读到并广播 → 同一客户端 writePump 写回（echo）。
func TestRegister_ReadWritePumpEcho(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.Register(w, r); err != nil {
			t.Errorf("Register: %v", err)
		}
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 等待服务端完成 register（run 循环异步处理）
	deadline := time.Now().Add(2 * time.Second)
	for h.Len() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("hub did not register client in time, Len=%d", h.Len())
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 客户端发送，期望 echo 回来
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("echo got %q, want hello", string(got))
	}

	// 关闭连接，readPump defer 应触发 unregister
	conn.Close()
	deadline = time.Now().Add(2 * time.Second)
	for h.Len() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("hub did not unregister client in time, Len=%d", h.Len())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
