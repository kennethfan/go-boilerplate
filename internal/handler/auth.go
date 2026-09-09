package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/pkg/response"
)

// AuthHandler 认证接口
type AuthHandler struct {
	svc *service.AuthService
}

// NewAuthHandler 构造
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login godoc
// @Summary      用户登录
// @Description  校验用户名密码，签发 access + refresh 双令牌
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        req  body      service.LoginRequest  true  "登录参数"
// @Success      200  {object}  response.Body{data=jwt.TokenPair}
// @Failure      400  {object}  response.Body
// @Failure      401  {object}  response.Body
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidParams(c, err)
		return
	}

	pair, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err)
		return
	}
	response.OK(c, pair)
}

// RefreshRequest 刷新入参
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh godoc
// @Summary      刷新令牌
// @Description  使用 refresh token 换发新的令牌对
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        req  body      RefreshRequest  true  "刷新参数"
// @Success      200  {object}  response.Body{data=jwt.TokenPair}
// @Failure      401  {object}  response.Body
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidParams(c, err)
		return
	}

	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err)
		return
	}
	response.OK(c, pair)
}
