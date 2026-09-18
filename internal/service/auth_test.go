package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/internal/repository"
	"github.com/kenneth/go-boilerplate/internal/testutil"
	"github.com/kenneth/go-boilerplate/pkg/errcode"
	jwtpkg "github.com/kenneth/go-boilerplate/pkg/jwt"
)

func newAuthService(t *testing.T) (*AuthService, *repository.UserRepository) {
	t.Helper()
	db := testutil.NewSQLiteMemory(t)
	repo := repository.NewUserRepository(db)
	jwtOpts := jwtpkg.Options{
		Secret:        "test-secret-needs-32-bytes-minimum-xxxx",
		Issuer:        "test",
		AccessExpire:  900 * time.Second,
		RefreshExpire: 86400 * time.Second,
	}
	return NewAuthService(repo, jwtOpts), repo
}

func TestAuthService_Login(t *testing.T) {
	svc, repo := newAuthService(t)
	ctx := context.Background()
	hashed, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := repo.Create(ctx, &model.User{Username: "admin", Password: hashed, Role: "admin"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// 成功
	pair, err := svc.Login(ctx, LoginRequest{Username: "admin", Password: "correct-horse"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must not be empty")
	}

	// 密码错误 → 模糊提示（不区分用户是否存在）
	_, err = svc.Login(ctx, LoginRequest{Username: "admin", Password: "wrong"})
	if !errors.Is(err, errcode.ErrPasswordWrong) {
		t.Errorf("wrong password: want ErrPasswordWrong, got %v", err)
	}

	// 用户不存在 → 同样模糊提示
	_, err = svc.Login(ctx, LoginRequest{Username: "ghost", Password: "whatever"})
	if !errors.Is(err, errcode.ErrPasswordWrong) {
		t.Errorf("unknown user: want ErrPasswordWrong, got %v", err)
	}
}

func TestAuthService_Refresh(t *testing.T) {
	svc, repo := newAuthService(t)
	ctx := context.Background()
	hashed, _ := HashPassword("pw")
	if err := repo.Create(ctx, &model.User{Username: "u", Password: hashed, Role: "user"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	pair, err := svc.Login(ctx, LoginRequest{Username: "u", Password: "pw"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// 成功换发
	newPair, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if newPair.AccessToken == "" {
		t.Fatal("new access token empty")
	}

	// 用 access token 当 refresh → 类型不符
	_, err = svc.Refresh(ctx, pair.AccessToken)
	if !errors.Is(err, errcode.ErrRefreshTokenUsed) {
		t.Errorf("wrong type: want ErrRefreshTokenUsed, got %v", err)
	}

	// 非法 token
	_, err = svc.Refresh(ctx, "not-a-token")
	if !errors.Is(err, errcode.ErrRefreshTokenUsed) {
		t.Errorf("invalid token: want ErrRefreshTokenUsed, got %v", err)
	}
}

func TestAuthService_RefreshDeletedUser(t *testing.T) {
	svc, repo := newAuthService(t)
	ctx := context.Background()
	hashed, _ := HashPassword("pw")
	u := &model.User{Username: "del", Password: hashed, Role: "user"}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("seed: %v", err)
	}
	pair, err := svc.Login(ctx, LoginRequest{Username: "del", Password: "pw"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// 已删除用户 refresh → 失败
	_, err = svc.Refresh(ctx, pair.RefreshToken)
	if !errors.Is(err, errcode.ErrRefreshTokenUsed) {
		t.Errorf("deleted user refresh: want ErrRefreshTokenUsed, got %v", err)
	}
}

func TestHashPassword(t *testing.T) {
	h, err := HashPassword("secret1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if h == "secret1" {
		t.Fatal("plaintext not hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(h), []byte("secret1")); err != nil {
		t.Fatalf("verify correct: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(h), []byte("other")); err == nil {
		t.Fatal("should not verify wrong password")
	}
}
