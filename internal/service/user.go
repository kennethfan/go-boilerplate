package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/internal/repository"
	"github.com/kenneth/go-boilerplate/pkg/errcode"
)

// UserService 用户管理服务
type UserService struct {
	repo *repository.UserRepository
}

// NewUserService 构造
func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// CreateUserRequest 创建用户入参
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Nickname string `json:"nickname" binding:"max=64"`
	Role     string `json:"role" binding:"required,oneof=admin user"`
}

// UpdateUserRequest 更新用户入参（全部可选）
type UpdateUserRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,max=64"`
	Role     *string `json:"role" binding:"omitempty,oneof=admin user"`
}

// Create 创建用户（密码 bcrypt 加密）
func (s *UserService) Create(ctx context.Context, req CreateUserRequest) (*model.User, error) {
	if _, err := s.repo.GetByUsername(ctx, req.Username); err == nil {
		return nil, errcode.ErrUserExists
	}

	hashed, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("user service: hash password: %w", err)
	}

	u := &model.User{
		Username: req.Username,
		Password: hashed,
		Nickname: req.Nickname,
		Role:     req.Role,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("user service: create: %w", err)
	}
	return u, nil
}

// Get 查询用户详情
func (s *UserService) Get(ctx context.Context, id uint64) (*model.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrUserNotFound
		}
		return nil, fmt.Errorf("user service: get: %w", err)
	}
	return u, nil
}

// List 分页查询
func (s *UserService) List(ctx context.Context, page, size int) ([]*model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return s.repo.ListPage(ctx, page, size)
}

// Update 更新用户（零值跳过）
func (s *UserService) Update(ctx context.Context, id uint64, req UpdateUserRequest) error {
	fields := map[string]any{}
	if req.Nickname != nil {
		fields["nickname"] = *req.Nickname
	}
	if req.Role != nil {
		fields["role"] = *req.Role
	}
	if len(fields) == 0 {
		return nil
	}
	if err := s.repo.Update(ctx, id, fields); err != nil {
		return fmt.Errorf("user service: update: %w", err)
	}
	return nil
}

// Delete 删除用户
func (s *UserService) Delete(ctx context.Context, id uint64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("user service: delete: %w", err)
	}
	return nil
}
