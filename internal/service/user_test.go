package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kenneth/go-boilerplate/internal/repository"
	"github.com/kenneth/go-boilerplate/internal/testutil"
	"github.com/kenneth/go-boilerplate/pkg/errcode"
)

func newUserService(t *testing.T) *UserService {
	t.Helper()
	db := testutil.NewSQLiteMemory(t)
	return NewUserService(repository.NewUserRepository(db))
}

func TestUserService_Create(t *testing.T) {
	svc := newUserService(t)
	ctx := context.Background()

	u, err := svc.Create(ctx, CreateUserRequest{
		Username: "bob", Password: "secret1", Nickname: "Bob", Role: "user",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected ID")
	}
	if u.Password == "secret1" {
		t.Fatal("password must be bcrypt-hashed, not plaintext")
	}

	// 重复用户名 → ErrUserExists
	_, err = svc.Create(ctx, CreateUserRequest{
		Username: "bob", Password: "secret1", Nickname: "Bob", Role: "user",
	})
	if !errors.Is(err, errcode.ErrUserExists) {
		t.Errorf("duplicate: want ErrUserExists, got %v", err)
	}
}

func TestUserService_Get(t *testing.T) {
	svc := newUserService(t)
	ctx := context.Background()
	u, _ := svc.Create(ctx, CreateUserRequest{Username: "carol", Password: "secret1", Role: "user"})

	got, err := svc.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Username != "carol" {
		t.Errorf("Get mismatch: %+v", got)
	}

	// 不存在 → ErrUserNotFound
	_, err = svc.Get(ctx, 12345)
	if !errors.Is(err, errcode.ErrUserNotFound) {
		t.Errorf("missing: want ErrUserNotFound, got %v", err)
	}
}

func TestUserService_List(t *testing.T) {
	svc := newUserService(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, CreateUserRequest{Username: fmt.Sprintf("u%d", i), Password: "secret1", Role: "user"}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// 边界归一化：page/size 非法 → 默认 page=1, size=20
	list, total, err := svc.List(ctx, 0, 0)
	if err != nil {
		t.Fatalf("List default: %v", err)
	}
	if total != 5 || len(list) != 5 {
		t.Errorf("List default want total=5 len=5, got total=%d len=%d", total, len(list))
	}

	// 第二页 size=2
	list, total, err = svc.List(ctx, 2, 2)
	if err != nil {
		t.Fatalf("List p2: %v", err)
	}
	if total != 5 {
		t.Errorf("List p2 total want 5, got %d", total)
	}
	if len(list) != 2 {
		t.Errorf("List p2 want len 2, got %d", len(list))
	}
}

func TestUserService_Update(t *testing.T) {
	svc := newUserService(t)
	ctx := context.Background()
	u, _ := svc.Create(ctx, CreateUserRequest{Username: "dave", Password: "secret1", Nickname: "Dave", Role: "user"})

	// 空更新 → no-op 且无 error
	if err := svc.Update(ctx, u.ID, UpdateUserRequest{}); err != nil {
		t.Fatalf("empty update: %v", err)
	}
	got, _ := svc.Get(ctx, u.ID)
	if got.Nickname != "Dave" {
		t.Errorf("empty update should not change nickname, got %q", got.Nickname)
	}

	nick := "DaveX"
	role := "admin"
	if err := svc.Update(ctx, u.ID, UpdateUserRequest{Nickname: &nick, Role: &role}); err != nil {
		t.Fatalf("partial update: %v", err)
	}
	got, _ = svc.Get(ctx, u.ID)
	if got.Nickname != "DaveX" || got.Role != "admin" {
		t.Errorf("partial update not applied: %+v", got)
	}
}

func TestUserService_Delete(t *testing.T) {
	svc := newUserService(t)
	ctx := context.Background()
	u, _ := svc.Create(ctx, CreateUserRequest{Username: "eve", Password: "secret1", Role: "user"})
	if err := svc.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// 删除后查询 → ErrUserNotFound
	if _, err := svc.Get(ctx, u.ID); !errors.Is(err, errcode.ErrUserNotFound) {
		t.Errorf("after delete: want ErrUserNotFound, got %v", err)
	}
}
