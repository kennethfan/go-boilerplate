package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/pkg/response"
)

// UserHandler 用户管理接口
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler 构造
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// ListQuery 列表查询参数
type ListQuery struct {
	Page int `form:"page" binding:"omitempty,min=1"`
	Size int `form:"size" binding:"omitempty,min=1,max=100"`
}

// ListUsers godoc
// @Summary      分页查询用户列表
// @Tags         users
// @Produce      json
// @Param        page  query     int  false  "页码（默认 1）"
// @Param        size  query     int  false  "每页数量（默认 20，最大 100）"
// @Success      200   {object}  response.Body
// @Failure      403   {object}  response.Body
// @Security     BearerAuth
// @Router       /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.InvalidParams(c, err)
		return
	}

	list, total, err := h.svc.List(c.Request.Context(), q.Page, q.Size)
	if err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

// GetUser godoc
// @Summary      查询用户详情
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "用户 ID"
// @Success      200  {object}  response.Body
// @Failure      404  {object}  response.Body
// @Security     BearerAuth
// @Router       /users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.InvalidParams(c, err)
		return
	}

	u, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err)
		return
	}
	response.OK(c, u)
}

// CreateUser godoc
// @Summary      创建用户（仅 admin）
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        req  body      service.CreateUserRequest  true  "用户参数"
// @Success      200  {object}  response.Body
// @Failure      400  {object}  response.Body
// @Failure      403  {object}  response.Body
// @Security     BearerAuth
// @Router       /users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidParams(c, err)
		return
	}

	u, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err)
		return
	}
	response.OK(c, u)
}

// UpdateUser godoc
// @Summary      更新用户（仅 admin）
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      int                       true  "用户 ID"
// @Param        req  body      service.UpdateUserRequest  true  "更新字段"
// @Success      200  {object}  response.Body
// @Failure      400  {object}  response.Body
// @Security     BearerAuth
// @Router       /users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.InvalidParams(c, err)
		return
	}

	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidParams(c, err)
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		response.Fail(c, http.StatusBadRequest, err)
		return
	}
	response.OK(c, nil)
}

// DeleteUser godoc
// @Summary      删除用户（仅 admin）
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "用户 ID"
// @Success      200  {object}  response.Body
// @Failure      403  {object}  response.Body
// @Security     BearerAuth
// @Router       /users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.InvalidParams(c, err)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusBadRequest, err)
		return
	}
	response.OK(c, nil)
}
