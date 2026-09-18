package errcode

import "testing"

func TestE_Error(t *testing.T) {
	e := New(10001, "参数错误")
	if e.Code != 10001 {
		t.Errorf("Code want 10001, got %d", e.Code)
	}
	if e.Msg != "参数错误" {
		t.Errorf("Msg want 参数错误, got %s", e.Msg)
	}
	if got := e.Error(); got != "[10001] 参数错误" {
		t.Errorf("Error() want [10001] 参数错误, got %q", got)
	}
}

func TestPredefinedCodes(t *testing.T) {
	cases := []struct {
		e    *E
		code int
		msg  string
	}{
		{ErrInvalidParams, 10001, "参数错误"},
		{ErrUnauthorized, 10002, "未授权或令牌失效"},
		{ErrForbidden, 10003, "无权限访问"},
		{ErrNotFound, 10004, "资源不存在"},
		{ErrInternal, 10005, "服务器内部错误"},
		{ErrUserNotFound, 20001, "用户不存在"},
		{ErrUserExists, 20002, "用户已存在"},
		{ErrPasswordWrong, 20003, "用户名或密码错误"},
		{ErrRefreshTokenUsed, 20004, "刷新令牌无效"},
	}
	for _, c := range cases {
		if c.e.Code != c.code || c.e.Msg != c.msg {
			t.Errorf("predefined %d: want (%d,%s), got (%d,%s)", c.code, c.code, c.msg, c.e.Code, c.e.Msg)
		}
	}
}
