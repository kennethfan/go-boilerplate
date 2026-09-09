//go:build grpc

// Package grpcserver 提供通用 gRPC server 启动封装：监听 / 注册服务 / reflection / 优雅退出。
package grpcserver

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RegisterFunc 将服务注册到 grpc.Server
type RegisterFunc func(*grpc.Server)

// Server gRPC server 封装
type Server struct {
	srv      *grpc.Server
	listener net.Listener
	addr     string
}

// New 创建并初始化 gRPC server（未启动）
func New(addr string, registers ...RegisterFunc) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("grpcserver: listen %s failed: %w", addr, err)
	}
	s := grpc.NewServer()
	for _, reg := range registers {
		reg(s)
	}
	// 启用 reflection，配合 grpcurl 可直接调试
	reflection.Register(s)
	return &Server{srv: s, listener: lis, addr: addr}, nil
}

// Serve 启动服务（阻塞）
func (s *Server) Serve() error { return s.srv.Serve(s.listener) }

// Shutdown 优雅退出
func (s *Server) Shutdown() { s.srv.GracefulStop() }

// Addr 实际监听地址
func (s *Server) Addr() string { return s.listener.Addr().String() }
