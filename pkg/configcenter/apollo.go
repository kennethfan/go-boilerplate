package configcenter

import (
	"context"
	"fmt"
	"strings"

	agollo "github.com/apolloconfig/agollo/v4"
	"github.com/apolloconfig/agollo/v4/env/config"
	"github.com/apolloconfig/agollo/v4/storage"
)

// apolloProvider Apollo 数据源，基于 apolloconfig/agollo/v4（长轮询监听）。
// 约定：主 namespace 中一个 key（默认 content）存放完整 YAML 文本。
type apolloProvider struct {
	client     agollo.Client
	namespace  string
	contentKey string
	onChange   func([]byte)
}

func newApollo(opts Options) (*apolloProvider, error) {
	if opts.ApolloAppID == "" {
		return nil, fmt.Errorf("configcenter: apollo provider needs apollo_app_id")
	}
	if opts.ApolloMetaAddr == "" {
		return nil, fmt.Errorf("configcenter: apollo provider needs apollo_meta_addr")
	}

	cluster := opts.ApolloCluster
	if cluster == "" {
		cluster = "default"
	}
	namespaces := opts.ApolloNamespaces
	if len(namespaces) == 0 {
		namespaces = []string{"application"}
	}
	contentKey := opts.ApolloContentKey
	if contentKey == "" {
		contentKey = "content"
	}

	appConfig := &config.AppConfig{
		AppID:         opts.ApolloAppID,
		Cluster:       cluster,
		NamespaceName: strings.Join(namespaces, ","),
		IP:            opts.ApolloMetaAddr,
		Secret:        opts.ApolloSecret,
	}

	client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return appConfig, nil
	})
	if err != nil {
		return nil, fmt.Errorf("configcenter: apollo client start failed: %w", err)
	}

	return &apolloProvider{
		client:     client,
		namespace:  namespaces[0],
		contentKey: contentKey,
	}, nil
}

func (a *apolloProvider) Name() string { return "apollo" }

func (a *apolloProvider) Fetch(_ context.Context) ([]byte, error) {
	content := a.client.GetStringValue(a.contentKey, "")
	if content == "" {
		return nil, fmt.Errorf("configcenter: apollo key %q is empty in namespace %q", a.contentKey, a.namespace)
	}
	return []byte(content), nil
}

func (a *apolloProvider) Watch(_ context.Context, onChange func(data []byte)) error {
	a.onChange = onChange
	a.client.AddChangeListener(&apolloListener{p: a})
	return nil
}

func (a *apolloProvider) reload() {
	data, err := a.Fetch(context.Background())
	if err != nil {
		return
	}
	if a.onChange != nil {
		a.onChange(data)
	}
}

func (a *apolloProvider) Close() error {
	a.client.Close()
	return nil
}

// apolloListener 实现 storage.ChangeListener，content key 变化时触发重载
type apolloListener struct {
	p *apolloProvider
}

func (l *apolloListener) OnChange(e *storage.ChangeEvent) {
	if e != nil {
		if _, ok := e.Changes[l.p.contentKey]; ok {
			l.p.reload()
		}
	}
}

func (l *apolloListener) OnNewestChange(_ *storage.FullChangeEvent) {
	l.p.reload()
}
