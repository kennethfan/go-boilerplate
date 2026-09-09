// Package errcode 定义业务错误码，统一错误语义。
package errcode

import "fmt"

// E 业务错误
type E struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (e *E) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Msg) }

// New 创建业务错误
func New(code int, msg string) *E { return &E{Code: code, Msg: msg} }

// 通用错误码：0 成功；10000-19999 通用；20000-29999 用户模块
var (
	ErrInvalidParams = New(10001, "参数错误")
	ErrUnauthorized  = New(10002, "未授权或令牌失效")
	ErrForbidden     = New(10003, "无权限访问")
	ErrNotFound      = New(10004, "资源不存在")
	ErrInternal      = New(10005, "服务器内部错误")

	ErrUserNotFound     = New(20001, "用户不存在")
	ErrUserExists       = New(20002, "用户已存在")
	ErrPasswordWrong    = New(20003, "用户名或密码错误")
	ErrRefreshTokenUsed = New(20004, "刷新令牌无效")
)
