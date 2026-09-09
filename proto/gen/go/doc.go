// Package gen 是 proto 生成的 Go 代码占位目录。
//
// 执行 `make proto` 后会生成：
//   - user.pb.go         消息类型
//   - user_grpc.pb.go     gRPC server / client stub
//
// 工具链安装（macOS）：
//   brew install protobuf
//   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//
// 注意：本目录下的生成 .go 文件均使用 `//go:build grpc` 标签，
// 必须用 `go build -tags grpc` 编译；默认 `go build` 不启用 gRPC。
package gen
