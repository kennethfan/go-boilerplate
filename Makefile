# Makefile —— 常用开发命令
# 前置要求：Go 1.26+；热重载需 air，Swagger 需 swag

APP      := go-boilerplate
MAIN     := ./cmd/server
BIN      := bin/server
CONFIG   := configs/config.yaml

.PHONY: run build tidy lint test swagger proto migrate-up migrate-down docker-up docker-down grpc-build grpc-run clean help

## 开发：热重载启动（需 go install github.com/air-verse/air@latest）
run:
	air -c .air.toml

## 构建二进制
build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o $(BIN) $(MAIN)

## 整理依赖（首次克隆后执行）
tidy:
	go mod tidy

## 静态检查
lint:
	go vet ./...

## 单元测试
test:
	go test ./... -count=1

## 生成/更新 Swagger 文档（需 go install github.com/swaggo/swag/cmd/swag@latest）
swagger:
	swag init -g $(MAIN)/main.go -o api/docs --parseDependency

## 生成 gRPC stub（需本地 brew install protobuf + go install protoc-gen-go + protoc-gen-go-grpc）
proto:
	protoc --proto_path=proto \
	  --go_out=proto/gen/go --go_opt=paths=source_relative \
	  --go-grpc_out=proto/gen/go --go-grpc_opt=paths=source_relative \
	  proto/*.proto

## 启用 gRPC 编译与运行（需先 make proto）
grpc-build:
	CGO_ENABLED=0 go build -tags grpc -ldflags "-s -w" -o $(BIN) $(MAIN)

grpc-run:
	go run -tags grpc ./cmd/server

## 数据库迁移
migrate-up:
	go run ./cmd/migrate -config $(CONFIG) -up

migrate-down:
	go run ./cmd/migrate -config $(CONFIG) -down 1

## 本地依赖容器（MySQL/Redis/RocketMQ/Jaeger）
docker-up:
	docker compose up -d

docker-down:
	docker compose down

## 清理构建产物
clean:
	rm -rf bin logs

help:
	@grep -E '^## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = "## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$2, $$3}'
