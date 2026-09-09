package configcenter

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// nacosProvider Nacos 数据源，基于 nacos-sdk-go/v2。
// dataId 内容约定为完整 YAML 文本。
type nacosProvider struct {
	client   config_client.IConfigClient
	dataID   string
	group    string
	onChange func([]byte)
}

func newNacos(opts Options) (*nacosProvider, error) {
	if opts.NacosDataID == "" {
		return nil, fmt.Errorf("configcenter: nacos provider needs nacos_data_id")
	}
	if len(opts.NacosAddrs) == 0 {
		return nil, fmt.Errorf("configcenter: nacos provider needs nacos_addrs")
	}

	group := opts.NacosGroup
	if group == "" {
		group = constant.DEFAULT_GROUP
	}

	serverConfigs := make([]constant.ServerConfig, 0, len(opts.NacosAddrs))
	for _, addr := range opts.NacosAddrs {
		host, port, err := splitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("configcenter: invalid nacos addr %q: %w", addr, err)
		}
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: host,
			Port:   port,
		})
	}

	cc := constant.ClientConfig{
		NamespaceId:         opts.NacosNamespace,
		TimeoutMs:           5000,
		LogLevel:            "warn",
		NotLoadCacheAtStart: true,
	}

	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("configcenter: nacos client init failed: %w", err)
	}

	return &nacosProvider{client: client, dataID: opts.NacosDataID, group: group}, nil
}

func (n *nacosProvider) Name() string { return "nacos" }

func (n *nacosProvider) Fetch(_ context.Context) ([]byte, error) {
	content, err := n.client.GetConfig(vo.ConfigParam{DataId: n.dataID, Group: n.group})
	if err != nil {
		return nil, fmt.Errorf("configcenter: nacos get config failed: %w", err)
	}
	return []byte(content), nil
}

func (n *nacosProvider) Watch(_ context.Context, onChange func(data []byte)) error {
	n.onChange = onChange
	err := n.client.ListenConfig(vo.ConfigParam{
		DataId: n.dataID,
		Group:  n.group,
		OnChange: func(namespace, group, dataId, data string) {
			if n.onChange != nil {
				n.onChange([]byte(data))
			}
		},
	})
	if err != nil {
		return fmt.Errorf("configcenter: nacos listen failed: %w", err)
	}
	return nil
}

func (n *nacosProvider) Close() error {
	n.client.CloseClient()
	return nil
}

// splitHostPort 解析 host:port
func splitHostPort(addr string) (string, uint64, error) {
	if !strings.Contains(addr, ":") {
		return addr, 8848, nil
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}
	return host, port, nil
}
