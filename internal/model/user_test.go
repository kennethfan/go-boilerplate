package model

import "testing"

func TestUser_TableName(t *testing.T) {
	if got := (&User{}).TableName(); got != "users" {
		t.Errorf("TableName() = %q, want %q", got, "users")
	}
}
