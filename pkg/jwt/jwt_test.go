package jwt

import (
	"testing"
	"time"
)

func testOpts() Options {
	return Options{
		Secret:        "test-secret-needs-32-bytes-minimum-xxxx",
		Issuer:        "test",
		AccessExpire:  900 * time.Second,
		RefreshExpire: 86400 * time.Second,
	}
}

func TestGenerateAndParseTokenPair(t *testing.T) {
	opts := testOpts()
	pair, err := GenerateTokenPair(opts, 42, "alice", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must not be empty")
	}
	if pair.ExpiresIn != 900 {
		t.Errorf("ExpiresIn want 900, got %d", pair.ExpiresIn)
	}

	access, err := ParseToken(opts, pair.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if access.UserID != 42 || access.Username != "alice" || access.Role != "admin" {
		t.Errorf("access claims mismatch: %+v", access)
	}
	if access.TokenType != TokenTypeAccess {
		t.Errorf("access type want %q, got %q", TokenTypeAccess, access.TokenType)
	}

	refresh, err := ParseToken(opts, pair.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh: %v", err)
	}
	if refresh.TokenType != TokenTypeRefresh {
		t.Errorf("refresh type want %q, got %q", TokenTypeRefresh, refresh.TokenType)
	}
}

func TestParseToken_Expired(t *testing.T) {
	opts := testOpts()
	opts.AccessExpire = -time.Second // 签发即过期
	pair, err := GenerateTokenPair(opts, 1, "x", "user")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := ParseToken(opts, pair.AccessToken); err != ErrTokenExpired {
		t.Errorf("expired: want ErrTokenExpired, got %v", err)
	}
}

func TestParseToken_Invalid(t *testing.T) {
	opts := testOpts()
	pair, err := GenerateTokenPair(opts, 1, "x", "user")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// 密钥不符 → 无效
	other := testOpts()
	other.Secret = "another-secret-needs-32-bytes-minimum-x"
	if _, err := ParseToken(other, pair.AccessToken); err != ErrTokenInvalid {
		t.Errorf("wrong secret: want ErrTokenInvalid, got %v", err)
	}

	// 乱码 → 无效
	if _, err := ParseToken(opts, "garbage.token.here"); err != ErrTokenInvalid {
		t.Errorf("garbage: want ErrTokenInvalid, got %v", err)
	}
}
