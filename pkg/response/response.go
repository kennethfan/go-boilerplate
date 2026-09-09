// Package response 提供统一 HTTP 响应封装。
// 格式：{"code": 0, "message": "success", "data": ...}
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/pkg/errcode"
)

// Body 统一响应体
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data})
}

// Fail 失败响应：业务错误返回其错误码；未知错误按 500 处理（不泄露内部细节）
func Fail(c *gin.Context, httpStatus int, err error) {
	var e *errcode.E
	if errors.As(err, &e) {
		c.JSON(httpStatus, Body{Code: e.Code, Message: e.Msg})
		return
	}
	c.JSON(httpStatus, Body{Code: errcode.ErrInternal.Code, Message: errcode.ErrInternal.Msg})
}

// 常用快捷方式
func InvalidParams(c *gin.Context, err error) {
	Fail(c, http.StatusBadRequest, errors.Join(errcode.ErrInvalidParams, err))
}

func Unauthorized(c *gin.Context) { Fail(c, http.StatusUnauthorized, errcode.ErrUnauthorized) }
func Forbidden(c *gin.Context)    { Fail(c, http.StatusForbidden, errcode.ErrForbidden) }
func NotFound(c *gin.Context)     { Fail(c, http.StatusNotFound, errcode.ErrNotFound) }
func Internal(c *gin.Context)     { Fail(c, http.StatusInternalServerError, errcode.ErrInternal) }
