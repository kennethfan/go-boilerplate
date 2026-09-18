package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/pkg/errcode"
)

func init() { gin.SetMode(gin.TestMode) }

func decode(t *testing.T, w *httptest.ResponseRecorder) Body {
	t.Helper()
	var b Body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, w.Body.String())
	}
	return b
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OK(c, map[string]any{"k": "v"})
	if w.Code != http.StatusOK {
		t.Errorf("status want 200, got %d", w.Code)
	}
	b := decode(t, w)
	if b.Code != 0 || b.Message != "success" {
		t.Errorf("body want code=0 msg=success, got %+v", b)
	}
	if b.Data == nil {
		t.Error("data should not be nil")
	}
}

func TestFail_BusinessError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, http.StatusBadRequest, errcode.ErrInvalidParams)
	b := decode(t, w)
	if b.Code != errcode.ErrInvalidParams.Code {
		t.Errorf("code want %d, got %d", errcode.ErrInvalidParams.Code, b.Code)
	}
	if b.Message != errcode.ErrInvalidParams.Msg {
		t.Errorf("msg want %q, got %q", errcode.ErrInvalidParams.Msg, b.Message)
	}
}

func TestFail_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, http.StatusInternalServerError, errors.New("boom"))
	b := decode(t, w)
	if b.Code != errcode.ErrInternal.Code {
		t.Errorf("code want internal %d, got %d", errcode.ErrInternal.Code, b.Code)
	}
	if b.Message != errcode.ErrInternal.Msg {
		t.Errorf("msg want %q, got %q", errcode.ErrInternal.Msg, b.Message)
	}
}

func TestShortcuts(t *testing.T) {
	cases := []struct {
		name string
		fn   func(c *gin.Context)
		want int
	}{
		{"InvalidParams", func(c *gin.Context) { InvalidParams(c, errors.New("x")) }, http.StatusBadRequest},
		{"Unauthorized", Unauthorized, http.StatusUnauthorized},
		{"Forbidden", Forbidden, http.StatusForbidden},
		{"NotFound", NotFound, http.StatusNotFound},
		{"Internal", Internal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			tc.fn(c)
			if w.Code != tc.want {
				t.Errorf("status want %d, got %d", tc.want, w.Code)
			}
		})
	}
}
