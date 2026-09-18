package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/internal/testutil"
	"gorm.io/gorm"
)

func TestUserRepository_CRUDAndQueries(t *testing.T) {
	db := testutil.NewSQLiteMemory(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create
	u := &model.User{Username: "alice", Password: "hashed", Nickname: "Alice", Role: "admin"}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected auto-increment ID > 0")
	}

	// GetByID
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Username != "alice" || got.Role != "admin" {
		t.Errorf("GetByID mismatch: %+v", got)
	}

	// GetByID 不存在 → ErrRecordNotFound
	if _, err := repo.GetByID(ctx, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByID missing: want ErrRecordNotFound, got %v", err)
	}

	// GetByUsername
	byName, err := repo.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if byName.ID != u.ID {
		t.Errorf("GetByUsername id mismatch: got %d want %d", byName.ID, u.ID)
	}

	// GetByUsername 不存在 → ErrRecordNotFound
	if _, err := repo.GetByUsername(ctx, "nobody"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByUsername missing: want ErrRecordNotFound, got %v", err)
	}

	// CountAll
	n, err := repo.CountAll(ctx)
	if err != nil {
		t.Fatalf("CountAll: %v", err)
	}
	if n != 1 {
		t.Errorf("CountAll want 1, got %d", n)
	}

	// Update
	if err := repo.Update(ctx, u.ID, map[string]any{"nickname": "Alicia", "role": "user"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.GetByID(ctx, u.ID)
	if got.Nickname != "Alicia" || got.Role != "user" {
		t.Errorf("Update not applied: %+v", got)
	}

	// ListPage：补种 5 条
	for i := 0; i < 5; i++ {
		if err := repo.Create(ctx, &model.User{Username: fmt.Sprintf("u%d", i), Password: "x", Role: "user"}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	list, total, err := repo.ListPage(ctx, 1, 2)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if total != 6 {
		t.Errorf("ListPage total want 6, got %d", total)
	}
	if len(list) != 2 {
		t.Errorf("ListPage page size want 2, got %d", len(list))
	}
	if len(list) == 2 && list[0].ID < list[1].ID {
		t.Errorf("ListPage should be id DESC, got %d then %d", list[0].ID, list[1].ID)
	}

	// Delete
	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	n, _ = repo.CountAll(ctx)
	if n != 5 {
		t.Errorf("CountAll after delete want 5, got %d", n)
	}
}
