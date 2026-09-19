package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestEvent_Write(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	e := Event{
		ID:    "1",
		Event: "greeting",
		Data:  "line1\nline2",
		Retry: 3 * time.Second,
	}
	e.Write(c)

	body := w.Body.String()
	if !strings.Contains(body, "id: 1\n") {
		t.Errorf("missing id field: %q", body)
	}
	if !strings.Contains(body, "event: greeting\n") {
		t.Errorf("missing event field: %q", body)
	}
	// 多行 data 应拆成多个 data: 行
	if !strings.Contains(body, "data: line1\n") || !strings.Contains(body, "data: line2\n") {
		t.Errorf("missing multiline data: %q", body)
	}
	if !strings.Contains(body, "retry: 3000\n") {
		t.Errorf("missing retry field: %q", body)
	}
	// 事件以空行结束
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("event should end with blank line: %q", body)
	}
}

func TestEvent_Write_Minimal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Event{Data: "hello"}.Write(c)
	body := w.Body.String()
	if !strings.Contains(body, "data: hello\n") {
		t.Errorf("missing data: %q", body)
	}
	if strings.Contains(body, "id:") || strings.Contains(body, "event:") {
		t.Errorf("should not contain id/event for minimal event: %q", body)
	}
}

func TestInit_HeadersAndDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	// 用带可取消 context 的 Request 替换，触发 done 通道
	c.Request = httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctx)

	done := Init(c)

	if c.Writer.Status() != http.StatusOK {
		t.Errorf("status want 200, got %d", c.Writer.Status())
	}
	h := c.Writer.Header()
	if h.Get("Content-Type") != "text/event-stream; charset=utf-8" {
		t.Errorf("Content-Type = %q", h.Get("Content-Type"))
	}
	if h.Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %q", h.Get("Cache-Control"))
	}
	if h.Get("Connection") != "keep-alive" {
		t.Errorf("Connection = %q", h.Get("Connection"))
	}
	if h.Get("X-Accel-Buffering") != "no" {
		t.Errorf("X-Accel-Buffering = %q", h.Get("X-Accel-Buffering"))
	}

	// 取消请求 context 后，done 应关闭
	cancel()
	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("done channel did not close after ctx cancel")
	}
}

func TestSend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Send(c, "quick message")
	body := w.Body.String()
	if !strings.Contains(body, "data: quick message\n") {
		t.Errorf("Send missing data: %q", body)
	}
}
