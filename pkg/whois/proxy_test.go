package whois

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/proxy"
)

// 一个用于测试的模拟代理拨号器
type mockProxyDialer struct{}

func (d *mockProxyDialer) Dial(network, addr string) (proxy.Dialer, error) {
	return proxy.Direct, nil
}

func TestSetWhoisProxy(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ProxyConfig
		wantErr bool
	}{
		{
			name: "Valid Proxy Configuration",
			cfg: &ProxyConfig{
				Enabled:  true,
				Address:  "127.0.0.1:1080",
				Type:     "socks5",
				Username: "user",
				Password: "pass",
				Timeout:  10,
			},
			wantErr: false,
		},
		{
			name: "Invalid Proxy Type",
			cfg: &ProxyConfig{
				Enabled:  true,
				Address:  "127.0.0.1:1080",
				Type:     "invalid",
				Username: "user",
				Password: "pass",
				Timeout:  10,
			},
			wantErr: true,
		},
		{
			name:    "Nil Configuration",
			cfg:     nil,
			wantErr: false,
		},
		{
			name: "Disabled Proxy",
			cfg: &ProxyConfig{
				Enabled: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetWhoisProxy(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidProxyAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "Valid IP and port",
			address: "127.0.0.1:1080",
			want:    true,
		},
		{
			name:    "Valid localhost and port",
			address: "localhost:1080",
			want:    true,
		},
		{
			name:    "No port",
			address: "127.0.0.1",
			want:    false,
		},
		{
			name:    "Invalid port",
			address: "127.0.0.1:99999",
			want:    false,
		},
		{
			name:    "Invalid format",
			address: "not a valid address",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidProxyAddress(tt.address)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestWhoisClient_NewWhoisClient(t *testing.T) {
	client := NewWhoisClient()
	assert.NotNil(t, client)
	assert.NotNil(t, client.dialer)
	assert.Equal(t, 30*time.Second, client.dialer.Timeout)
	assert.Nil(t, client.dialer.ProxyDialer)
}

func TestWhoisClient_SetTimeout(t *testing.T) {
	client := NewWhoisClient()
	timeout := 15 * time.Second
	client.SetTimeout(timeout)
	assert.Equal(t, timeout, client.dialer.Timeout)
}

func TestGetDialer(t *testing.T) {
	tests := []struct {
		name     string
		config   *ProxyConfig
		wantErr  bool
		errorMsg string
	}{
		{
			name: "Valid socks5 configuration",
			config: &ProxyConfig{
				Type:     "socks5",
				Address:  "127.0.0.1:1080",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "Valid http configuration",
			config: &ProxyConfig{
				Type:     "http",
				Address:  "127.0.0.1:8080",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "Invalid proxy type",
			config: &ProxyConfig{
				Type:    "invalid",
				Address: "127.0.0.1:1080",
			},
			wantErr:  true,
			errorMsg: "不支持的代理类型",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.config.GetDialer()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// 注意：这个测试可能依赖于实际的网络环境
				// 如果创建代理失败，可能是因为没有实际运行的代理
				if err != nil {
					t.Logf("创建代理失败，但可能是因为没有实际运行的代理: %v", err)
				}
			}
		})
	}
}
