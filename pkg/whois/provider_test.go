package whois

import (
	"context"
	"errors"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

// stubWhoisQueryProvider 用于测试的 WhoisQueryProvider 桩实现。
type stubWhoisQueryProvider struct {
	raw   string
	info  whoisparser.WhoisInfo
	queryErr error
	parseErr error
	queried  bool
	parsed   bool
}

func (s *stubWhoisQueryProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	s.queried = true
	if s.queryErr != nil {
		return "", s.queryErr
	}
	return s.raw, nil
}

func (s *stubWhoisQueryProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	s.parsed = true
	if s.parseErr != nil {
		return whoisparser.WhoisInfo{}, s.parseErr
	}
	return s.info, nil
}

// TestProviderInjection 验证注入自定义 WhoisQueryProvider 后查询走自定义实现。
func TestProviderInjection(t *testing.T) {
	// 保存并恢复全局 provider
	original := globalWhoisQueryProvider
	defer func() { globalWhoisQueryProvider = original }()

	stub := &stubWhoisQueryProvider{
		raw: "raw stub response",
		info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{Domain: "stub.example"},
		},
	}
	SetWhoisQueryProvider(stub)
	defer SetWhoisQueryProvider(nil) // 恢复默认

	info, err := ExecuteQueryWithContext(context.Background(), &QueryOptions{
		Domain: "stub.example",
	})
	if err != nil {
		t.Fatalf("ExecuteQueryWithContext 返回错误: %v", err)
	}
	if !stub.queried {
		t.Error("未调用注入 provider 的 Query")
	}
	if !stub.parsed {
		t.Error("未调用注入 provider 的 Parse")
	}
	if info == nil || info.Domain == nil || info.Domain.Domain != "stub.example" {
		t.Errorf("返回信息不匹配: %+v", info)
	}
}

// TestProviderInjectionError 验证注入 provider 的查询错误能正确传播。
func TestProviderInjectionError(t *testing.T) {
	original := globalWhoisQueryProvider
	defer func() { globalWhoisQueryProvider = original }()

	stub := &stubWhoisQueryProvider{
		queryErr: errors.New("stub query failure"),
	}
	SetWhoisQueryProvider(stub)
	defer SetWhoisQueryProvider(nil)

	_, err := ExecuteQueryWithContext(context.Background(), &QueryOptions{
		Domain: "stub.example",
	})
	if err == nil {
		t.Fatal("期望返回查询错误，得到 nil")
	}
}

// TestProviderNilRestoresDefault 验证 Set(nil) 恢复默认 provider。
func TestProviderNilRestoresDefault(t *testing.T) {
	original := globalWhoisQueryProvider
	defer func() { globalWhoisQueryProvider = original }()

	SetWhoisQueryProvider(&stubWhoisQueryProvider{})
	SetWhoisQueryProvider(nil)
	p := GetWhoisQueryProvider()
	if _, ok := p.(*DefaultWhoisQueryProvider); !ok {
		t.Errorf("Set(nil) 后应恢复 DefaultWhoisQueryProvider，得到 %T", p)
	}
}

// TestProviderGettersLazyInit 验证各 provider getter 懒加载默认实现。
func TestProviderGettersLazyInit(t *testing.T) {
	// 临时清空全局变量（保存原值）
	oW, oI, oA, oR := globalWhoisQueryProvider, globalIPWhoisProvider, globalASNProvider, globalRDAPProvider
	globalWhoisQueryProvider, globalIPWhoisProvider, globalASNProvider, globalRDAPProvider = nil, nil, nil, nil
	defer func() {
		globalWhoisQueryProvider, globalIPWhoisProvider, globalASNProvider, globalRDAPProvider = oW, oI, oA, oR
	}()

	if _, ok := GetWhoisQueryProvider().(*DefaultWhoisQueryProvider); !ok {
		t.Error("GetWhoisQueryProvider 未懒加载默认实现")
	}
	if _, ok := GetIPWhoisProvider().(*DefaultIPWhoisProvider); !ok {
		t.Error("GetIPWhoisProvider 未懒加载默认实现")
	}
	if _, ok := GetASNProvider().(*DefaultASNProvider); !ok {
		t.Error("GetASNProvider 未懒加载默认实现")
	}
	if _, ok := GetRDAPProvider().(*DefaultRDAPProvider); !ok {
		t.Error("GetRDAPProvider 未懒加载默认实现")
	}
}
