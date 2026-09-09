//go:build grpc

// Package goboilerplatev1 是 proto 生成的 Go 代码包名（与 proto/user.proto 中
// go_package = "..../proto/gen/go;goboilerplatev1" 一致）。
//
// 执行 `make proto` 后会生成：
//   - user.pb.go          消息类型
//   - user_grpc.pb.go     gRPC server / client stub
//
// 工具链安装（macOS）：
//   brew install protobuf
//   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//
// 注意：本目录下的生成 .go 文件均使用 `//go:build grpc` 标签，
// 必须用 `go build -tags grpc` 编译；默认 `go build` 不启用 gRPC。
//
// 本占位文件同时打 `//go:build grpc` 标签，仅在 grpc tag 下与生成代码
// 同处一个包；默认 build 时完全屏蔽，避免与未来生成的 user.pb.go 包名
// 冲突（doc.go 内容负责描述生成目录用途）。
package goboilerplatev1
