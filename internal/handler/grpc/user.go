//go:build grpc

package grpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	pb "github.com/kenneth/go-boilerplate/proto/gen/go"

	"github.com/kenneth/go-boilerplate/internal/model"
	"github.com/kenneth/go-boilerplate/internal/service"
	"github.com/kenneth/go-boilerplate/pkg/errcode"
)

// UserGRPC UserService 的 gRPC 实现（复用 service.UserService）
type UserGRPC struct {
	pb.UnimplementedUserServiceServer
	svc *service.UserService
}

// NewUserGRPC 构造
func NewUserGRPC(svc *service.UserService) *UserGRPC {
	return &UserGRPC{svc: svc}
}

// Register 注册 UserService 到 grpc.Server
func Register(s *grpc.Server, svc *service.UserService) {
	pb.RegisterUserServiceServer(s, NewUserGRPC(svc))
}

// RegisterGreeter 注册 Greeter 演示服务
func RegisterGreeter(s *grpc.Server) {
	pb.RegisterGreeterServer(s, &GreeterGRPC{})
}

func toPB(u *model.User) *pb.User {
	return &pb.User{
		Id:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Role:      u.Role,
		CreatedAt: timePtr(u.CreatedAt),
		UpdatedAt: timePtr(u.UpdatedAt),
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// GetUser gRPC
func (g *UserGRPC) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	u, err := g.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, toGRPCErr(err)
	}
	return toPB(u), nil
}

// ListUsers gRPC
func (g *UserGRPC) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	page := int(req.GetPage())
	size := int(req.GetSize())
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	list, total, err := g.svc.List(ctx, page, size)
	if err != nil {
		return nil, toGRPCErr(err)
	}
	users := make([]*pb.User, 0, len(list))
	for _, u := range list {
		users = append(users, toPB(u))
	}
	return &pb.ListUsersResponse{Users: users, Total: total}, nil
}

// CreateUser gRPC
func (g *UserGRPC) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	u, err := g.svc.Create(ctx, service.CreateUserRequest{
		Username: req.GetUsername(),
		Password: req.GetPassword(),
		Nickname: req.GetNickname(),
		Role:     req.GetRole(),
	})
	if err != nil {
		return nil, toGRPCErr(err)
	}
	return toPB(u), nil
}

// DeleteUser gRPC
func (g *UserGRPC) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	if err := g.svc.Delete(ctx, req.GetId()); err != nil {
		return nil, toGRPCErr(err)
	}
	return &pb.DeleteUserResponse{Ok: true}, nil
}

// GreeterGRPC 演示服务
type GreeterGRPC struct {
	pb.UnimplementedGreeterServer
}

func (g *GreeterGRPC) SayHello(_ context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	name := req.GetName()
	if name == "" {
		name = "world"
	}
	return &pb.HelloReply{Message: "hello, " + name}, nil
}

// toGRPCErr 将业务错误映射为 gRPC status；未知错误按 Internal 处理
func toGRPCErr(err error) error {
	var e *errcode.E
	if errors.As(err, &e) {
		switch e.Code {
		case errcode.ErrUserNotFound.Code, errcode.ErrNotFound.Code:
			return status.Error(codes.NotFound, e.Msg)
		case errcode.ErrUserExists.Code:
			return status.Error(codes.AlreadyExists, e.Msg)
		case errcode.ErrInvalidParams.Code:
			return status.Error(codes.InvalidArgument, e.Msg)
		case errcode.ErrUnauthorized.Code, errcode.ErrRefreshTokenUsed.Code:
			return status.Error(codes.Unauthenticated, e.Msg)
		case errcode.ErrForbidden.Code:
			return status.Error(codes.PermissionDenied, e.Msg)
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status.Error(codes.NotFound, "not found")
	}
	return status.Error(codes.Internal, errcode.ErrInternal.Msg)
}
