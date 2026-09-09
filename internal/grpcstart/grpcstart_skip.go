//go:build !grpc

// Package grpcstart 非 grpc build tag 下的占位实现（默认 go build 不含 gRPC）。
package grpcstart

import (
	"context"

	"go.uber.org/zap"

	"github.com/kenneth/go-boilerplate/internal/config"
	"github.com/kenneth/go-boilerplate/internal/service"
)

// Start 默认构建下直接返回（gRPC 未启用）
func Start(_ context.Context, _ *config.Config, _ *zap.Logger, _ *service.UserService) error {
	return nil
}
