package uc

import (
	"sync"

	"github.com/wpajqz/linker/client"
	"github.com/wpajqz/linker/plugin/crypt"
)

var (
	defaultClient *Client
	once          = &sync.Once{}
)

// Client 用户SDK
type Client struct{ *client.Client }

// NewClient 初始化用户SDK实例
func NewClient(address []string, opts ...client.Option) (*Client, error) {
	var (
		err error
		cc  *client.Client
	)

	opts = append(opts,
		client.PluginForPacketSender(crypt.NewEncryptPlugin()),
		client.PluginForPacketReceiver(crypt.NewDecryptPlugin()),
	)

	once.Do(func() {
		cc, err = client.NewClient(address, opts...)
		if err != nil {
			return
		}

		defaultClient = &Client{cc}
	})

	return defaultClient, err
}

// GetClient 返回用户SDK实例
func GetClient() *Client {
	return defaultClient
}
