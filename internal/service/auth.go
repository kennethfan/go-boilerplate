// Package service 业务逻辑层。
package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/kenneth/go-boilerplate/internal/repository"
	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
	"github.com/kenneth/go-boilerplate/pkg/errcode"
)

// AuthService 认证服务：登录签发与令牌刷新
type AuthService struct {
	repo    *repository.UserRepository
	jwtOpts jwtpkg.Options
}

// NewAuthService 构造
func NewAuthService(repo *repository.UserRepository, jwtOpts jwtpkg.Options) *AuthService {
	return &AuthService{repo: repo, jwtOpts: jwtOpts}
}

// LoginRequest 登录入参
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=64"`
}

// Login 校验用户名密码并签发令牌对
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*jwtpkg.TokenPair, error) {
	u, err := s.repo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrPasswordWrong // 不暴露"用户不存在"，统一模糊提示
		}
		return nil, fmt.Errorf("auth service: query user: %w", err)
	}

	// bcrypt 恒定时间比较
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, errcode.ErrPasswordWrong
	}

	pair, err := jwtpkg.GenerateTokenPair(s.jwtOpts, u.ID, u.Username, u.Role)
	if err != nil {
		return nil, fmt.Errorf("auth service: generate token: %w", err)
	}
	return pair, nil
}

// Refresh 用 refresh token 换发新的令牌对
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*jwtpkg.TokenPair, error) {
	claims, err := jwtpkg.ParseToken(s.jwtOpts, refreshToken)
	if err != nil {
		return nil, errcode.ErrRefreshTokenUsed
	}
	if claims.TokenType != jwtpkg.TokenTypeRefresh {
		return nil, errcode.ErrRefreshTokenUsed
	}

	// 校验用户仍然存在（防止已删除用户继续刷新）
	u, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, errcode.ErrRefreshTokenUsed
	}

	pair, err := jwtpkg.GenerateTokenPair(s.jwtOpts, u.ID, u.Username, u.Role)
	if err != nil {
		return nil, fmt.Errorf("auth service: generate token: %w", err)
	}
	return pair, nil
}

// HashPassword bcrypt 加密（CreateUser / seed 复用）
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
