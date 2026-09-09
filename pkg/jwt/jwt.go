// Package jwt 提供 JWT 签发与校验（golang-jwt/v5）。
// 双令牌机制：access（访问接口）+ refresh（换取新 access）。
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 令牌用途标记
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims 自定义载荷
type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TokenType string `json:"token_type"` // access / refresh
	jwt.RegisteredClaims
}

// Options JWT 配置
type Options struct {
	Secret        string        // 签名密钥，至少 32 字节
	Issuer        string
	AccessExpire  time.Duration // access token 有效期
	RefreshExpire time.Duration // refresh token 有效期
}

// TokenPair 一次签发的令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access token 剩余秒数
}

var (
	ErrTokenInvalid = errors.New("token 无效")
	ErrTokenExpired = errors.New("token 已过期")
	ErrWrongType    = errors.New("token 类型不匹配")
)

// GenerateTokenPair 为指定用户签发 access + refresh 令牌对
func GenerateTokenPair(opts Options, userID uint64, username, role string) (*TokenPair, error) {
	now := time.Now()

	access, err := signToken(opts, userID, username, role, TokenTypeAccess, now, opts.AccessExpire)
	if err != nil {
		return nil, err
	}
	refresh, err := signToken(opts, userID, username, role, TokenTypeRefresh, now, opts.RefreshExpire)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(opts.AccessExpire.Seconds()),
	}, nil
}

func signToken(opts Options, userID uint64, username, role, tokenType string, now time.Time, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    opts.Issuer,
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(opts.Secret))
	if err != nil {
		return "", err
	}
	return s, nil
}

// ParseToken 校验签名与有效期并返回 Claims
func ParseToken(opts Options, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return []byte(opts.Secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
