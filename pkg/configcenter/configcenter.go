// Package configcenter 提供统一的配置中心抽象，支持 local / nacos / apollo 三种数据源。
//
// 统一约定：三种源最终都以「完整 YAML 文本」的形式产出业务配置，
// 由上层（internal/config）用 viper 反序列化为 Config 结构。
//   - local：直接读本地 YAML 文件
//   - nacos：dataId 内容为完整 YAML
//   - apollo：namespace 中约定一个 key（默认 content）存放完整 YAML 文本
package configcenter

import (
	"context"
	"fmt"
)

// Provider 配置中心统一接口
type Provider interface {
	// Name 数据源名称
	Name() string
	// Fetch 拉取当前完整配置内容（YAML 字节）
	Fetch(ctx context.Context) ([]byte, error)
	// Watch 监听变更，数据变化时回调 onChange（传最新内容）
	Watch(ctx context.Context, onChange func(data []byte)) error
	// Close 释放资源（取消监听、关闭连接）
	Close() error
}

// Options 数据源连接配置
type Options struct {
	Type     string // local / nacos / apollo
	FilePath string // local：配置文件路径

	// nacos
	NacosAddrs     []string // 服务地址 host:port
	NacosNamespace string   // 命名空间（默认 public 留空）
	NacosDataID    string   // dataId
	NacosGroup     string   // group（默认 DEFAULT_GROUP）

	// apollo
	ApolloAppID       string   // 应用 ID
	ApolloCluster     string   // 集群（默认 default）
	ApolloMetaAddr    string   // config service 地址，如 http://127.0.0.1:8080
	ApolloNamespaces  []string // 命名空间（默认 ["application"]）
	ApolloContentKey  string   // 存放完整 YAML 的 key（默认 content）
	ApolloSecret      string   // 访问密钥（默认留空）
}

// New 根据类型创建对应的 Provider
func New(opts Options) (Provider, error) {
	switch opts.Type {
	case "local", "":
		return newLocal(opts.FilePath)
	case "nacos":
		return newNacos(opts)
	case "apollo":
		return newApollo(opts)
	default:
		return nil, fmt.Errorf("configcenter: unsupported type %q (support local / nacos / apollo)", opts.Type)
	}
}
