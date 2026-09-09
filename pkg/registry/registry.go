// Package registry 提供服务注册与发现抽象，支持 nacos（可扩展 consul/etcd）。
// 典型用法：
//
//	provider, _ := registry.New(registry.Options{...})
//	provider.Register(ctx, registry.Instance{ServiceName:"svc", IP:"1.2.3.4", Port:8080})
//	defer provider.Deregister(...)
//	instances, _ := provider.GetHealthy(ctx, "svc", "DEFAULT_GROUP")
package registry

import (
	"context"
	"fmt"
)

// Provider 注册中心接口
type Provider interface {
	Name() string
	// Register 注册一个服务实例
	Register(ctx context.Context, ins Instance) error
	// Deregister 注销一个服务实例
	Deregister(ctx context.Context, ins Instance) error
	// GetHealthy 拉取指定服务的健康实例列表
	GetHealthy(ctx context.Context, serviceName, group string) ([]Instance, error)
	// Subscribe 订阅服务实例变更（可选实现，nacos 支持；consul/etcd 需对接 watch）
	Subscribe(ctx context.Context, serviceName, group string, onChange func([]Instance)) error
	// Close 关闭客户端
	Close() error
}

// Instance 服务实例
type Instance struct {
	ServiceName string
	Group       string
	IP          string
	Port        int
	Weight      float64
	Enable      bool
	Healthy     bool
	Ephemeral   bool
	Metadata    map[string]string
}

// Options 注册中心连接配置
type Options struct {
	Type          string // nacos / consul / etcd
	ServiceName   string // 注册时使用的默认服务名
	Group         string // 默认 DEFAULT_GROUP
	IP            string
	Port          int
	Ephemeral     bool
	Metadata      map[string]string

	// nacos
	NacosAddrs     []string
	NacosNamespace string
}

// New 按类型创建 provider
func New(opts Options) (Provider, error) {
	switch opts.Type {
	case "nacos":
		return newNacos(opts)
	default:
		return nil, fmt.Errorf("registry: unsupported type %q (support nacos)", opts.Type)
	}
}
