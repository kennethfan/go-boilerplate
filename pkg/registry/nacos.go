package registry

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// nacosProvider Nacos 注册中心实现
type nacosProvider struct {
	client naming_client.INamingClient
}

func newNacos(opts Options) (*nacosProvider, error) {
	if len(opts.NacosAddrs) == 0 {
		return nil, fmt.Errorf("registry: nacos provider needs nacos_addrs")
	}
	serverConfigs := make([]constant.ServerConfig, 0, len(opts.NacosAddrs))
	for _, addr := range opts.NacosAddrs {
		host, port, err := splitHostPort(addr, 8848)
		if err != nil {
			return nil, fmt.Errorf("registry: invalid nacos addr %q: %w", addr, err)
		}
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: host, Port: port,
		})
	}
	cc := constant.ClientConfig{
		NamespaceId:         opts.NacosNamespace,
		TimeoutMs:           5000,
		LogLevel:            "warn",
		NotLoadCacheAtStart: true,
	}
	client, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("registry: nacos naming client init failed: %w", err)
	}
	return &nacosProvider{client: client}, nil
}

func (n *nacosProvider) Name() string { return "nacos" }

func (n *nacosProvider) Register(ctx context.Context, ins Instance) error {
	if ins.Weight == 0 {
		ins.Weight = 1
	}
	if ins.Group == "" {
		ins.Group = constant.DEFAULT_GROUP
	}
	ip := ins.IP
	if ip == "" {
		ip = localIP()
	}
	ok, err := n.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        uint64(ins.Port),
		ServiceName: ins.ServiceName,
		GroupName:   ins.Group,
		Weight:      ins.Weight,
		Enable:      ins.Enable,
		Healthy:     ins.Healthy,
		Ephemeral:   ins.Ephemeral,
		Metadata:    ins.Metadata,
	})
	if err != nil {
		return fmt.Errorf("registry: nacos register failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("registry: nacos register returned false")
	}
	return nil
}

func (n *nacosProvider) Deregister(ctx context.Context, ins Instance) error {
	if ins.Group == "" {
		ins.Group = constant.DEFAULT_GROUP
	}
	_, err := n.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ins.IP,
		Port:        uint64(ins.Port),
		ServiceName: ins.ServiceName,
		GroupName:   ins.Group,
		Ephemeral:   ins.Ephemeral,
	})
	if err != nil {
		return fmt.Errorf("registry: nacos deregister failed: %w", err)
	}
	return nil
}

func (n *nacosProvider) GetHealthy(ctx context.Context, serviceName, group string) ([]Instance, error) {
	if group == "" {
		group = constant.DEFAULT_GROUP
	}
	svc, err := n.client.GetService(vo.GetServiceParam{
		ServiceName: serviceName,
		GroupName:   group,
	})
	if err != nil {
		return nil, fmt.Errorf("registry: nacos get service failed: %w", err)
	}
	out := make([]Instance, 0, len(svc.Hosts))
	for _, h := range svc.Hosts {
		if !h.Healthy {
			continue
		}
		out = append(out, Instance{
			ServiceName: serviceName,
			Group:       group,
			IP:          h.Ip,
			Port:        int(h.Port),
			Weight:      h.Weight,
			Enable:      h.Enable,
			Healthy:     h.Healthy,
			Ephemeral:   h.Ephemeral,
			Metadata:    h.Metadata,
		})
	}
	return out, nil
}

func (n *nacosProvider) Subscribe(ctx context.Context, serviceName, group string, onChange func([]Instance)) error {
	if group == "" {
		group = constant.DEFAULT_GROUP
	}
	return n.client.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   group,
		SubscribeCallback: func(services []model.Instance, _ error) {
			out := make([]Instance, 0, len(services))
			for _, s := range services {
				if !s.Healthy {
					continue
				}
				out = append(out, Instance{
					ServiceName: serviceName,
					Group:       group,
					IP:          s.Ip,
					Port:        int(s.Port),
					Weight:      s.Weight,
					Enable:      s.Enable,
					Healthy:     s.Healthy,
					Ephemeral:   s.Ephemeral,
					Metadata:    s.Metadata,
				})
			}
			onChange(out)
		},
	})
}

func (n *nacosProvider) Close() error {
	n.client.CloseClient()
	return nil
}

// splitHostPort 解析 host:port；缺端口时使用 fallback
func splitHostPort(addr string, fallback uint64) (string, uint64, error) {
	if !strings.Contains(addr, ":") {
		return addr, fallback, nil
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	var port uint64
	_, err = fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}
	return host, port, nil
}

// localIP 获取本机非回环 IPv4（仅用于服务注册时上报）
var localIPOnce sync.Once
var localIPValue string

func localIP() string {
	localIPOnce.Do(func() {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				localIPValue = ipnet.IP.String()
				return
			}
		}
	})
	return localIPValue
}
