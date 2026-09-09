//go:build grpc

// Package grpcstart 启动 gRPC server（需 build tag grpc 启用）。
package grpcstart

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/kenneth/go-boilerplate/internal/config"
	grpchandler "github.com/kenneth/go-boilerplate/internal/handler/grpc"
	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/pkg/grpcserver"
)

// Start 启动 gRPC server；如未启用返回 nil
func Start(ctx context.Context, cfg *config.Config, l *zap.Logger, userSvc *service.UserService) error {
	if cfg == nil || !cfg.GRPC.Enable {
		return nil
	}
	s, err := grpcserver.New(cfg.GRPC.Addr, func(s *grpc.Server) {
		grpchandler.Register(s, userSvc)
		// 演示服务（可删除）
		grpchandler.RegisterGreeter(s)
	})
	if err != nil {
		return err
	}
	l.Info("grpc server started", zap.String("addr", s.Addr()))

	go func() {
		if err := s.Serve(); err != nil {
			l.Error("grpc serve error", zap.Error(err))
		}
	}()
	go func() {
		<-ctx.Done()
		l.Info("grpc server shutting down")
		s.Shutdown()
	}()
	return nil
}
