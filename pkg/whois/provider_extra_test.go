package whois

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---- Set/Get nil 恢复默认 ----

func TestSetIPWhoisProvider_NilRestoresDefault(t *testing.T) {
	orig := globalIPWhoisProvider
	defer func() { globalIPWhoisProvider = orig }()

	SetIPWhoisProvider(nil)
	_, ok := GetIPWhoisProvider().(*DefaultIPWhoisProvider)
	assert.True(t, ok)
}

func TestSetASNProvider_NilRestoresDefault(t *testing.T) {
	orig := globalASNProvider
	defer func() { globalASNProvider = orig }()

	SetASNProvider(nil)
	_, ok := GetASNProvider().(*DefaultASNProvider)
	assert.True(t, ok)
}

func TestSetRDAPProvider_NilRestoresDefault(t *testing.T) {
	orig := globalRDAPProvider
	defer func() { globalRDAPProvider = orig }()

	SetRDAPProvider(nil)
	_, ok := GetRDAPProvider().(*DefaultRDAPProvider)
	assert.True(t, ok)
}

// ---- Set 注入自定义 provider（非 nil） ----

type stubIPWhoisProvider struct {
	result *IPWhoisResult
	err    error
	called bool
}

func (s *stubIPWhoisProvider) QueryIP(ctx context.Context, ip string, opts *IPWhoisOptions) (*IPWhoisResult, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type stubASNProvider struct {
	result *ASNDetail
	err    error
	called bool
}

func (s *stubASNProvider) QueryASN(ctx context.Context, opts *ASNQueryOptions) (*ASNDetail, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type stubRDAPProvider struct {
	domainResult  *RDAPResult
	ipResult      *RDAPIPResult
	asnResult     *RDAPASNResult
	entityResult  *RDAPEntityResult
	err           error
	calledDomain  bool
	calledIP      bool
	calledASN     bool
	calledEntity  bool
}

func (s *stubRDAPProvider) QueryDomain(ctx context.Context, opts *RDAPQueryOptions) (*RDAPResult, error) {
	s.calledDomain = true
	if s.err != nil {
		return nil, s.err
	}
	return s.domainResult, nil
}
func (s *stubRDAPProvider) QueryIP(ctx context.Context, opts *RDAPQueryOptions) (*RDAPIPResult, error) {
	s.calledIP = true
	if s.err != nil {
		return nil, s.err
	}
	return s.ipResult, nil
}
func (s *stubRDAPProvider) QueryASN(ctx context.Context, opts *RDAPQueryOptions) (*RDAPASNResult, error) {
	s.calledASN = true
	if s.err != nil {
		return nil, s.err
	}
	return s.asnResult, nil
}
func (s *stubRDAPProvider) QueryEntity(ctx context.Context, opts *RDAPQueryOptions) (*RDAPEntityResult, error) {
	s.calledEntity = true
	if s.err != nil {
		return nil, s.err
	}
	return s.entityResult, nil
}

func TestSetIPWhoisProvider_Inject(t *testing.T) {
	orig := globalIPWhoisProvider
	defer func() { globalIPWhoisProvider = orig }()

	stub := &stubIPWhoisProvider{result: &IPWhoisResult{IP: "1.2.3.4"}}
	SetIPWhoisProvider(stub)
	defer SetIPWhoisProvider(nil)

	res, err := GetIPWhoisProvider().QueryIP(context.Background(), "1.2.3.4", nil)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", res.IP)
	assert.True(t, stub.called)
}

func TestSetASNProvider_Inject(t *testing.T) {
	orig := globalASNProvider
	defer func() { globalASNProvider = orig }()

	stub := &stubASNProvider{result: &ASNDetail{ASN: 12345}}
	SetASNProvider(stub)
	defer SetASNProvider(nil)

	res, err := GetASNProvider().QueryASN(context.Background(), &ASNQueryOptions{ASN: 12345})
	assert.NoError(t, err)
	assert.Equal(t, 12345, res.ASN)
	assert.True(t, stub.called)
}

func TestSetRDAPProvider_Inject(t *testing.T) {
	orig := globalRDAPProvider
	defer func() { globalRDAPProvider = orig }()

	stub := &stubRDAPProvider{domainResult: &RDAPResult{}}
	SetRDAPProvider(stub)
	defer SetRDAPProvider(nil)

	p := GetRDAPProvider()
	_, err := p.QueryDomain(context.Background(), &RDAPQueryOptions{})
	assert.NoError(t, err)
	assert.True(t, stub.calledDomain)

	_, err = p.QueryIP(context.Background(), &RDAPQueryOptions{})
	assert.NoError(t, err)
	assert.True(t, stub.calledIP)

	_, err = p.QueryASN(context.Background(), &RDAPQueryOptions{})
	assert.NoError(t, err)
	assert.True(t, stub.calledASN)

	_, err = p.QueryEntity(context.Background(), &RDAPQueryOptions{})
	assert.NoError(t, err)
	assert.True(t, stub.calledEntity)
}

// ---- DefaultIPWhoisProvider.QueryIP nil opts ----

func TestDefaultIPWhoisProvider_QueryIP_NilOpts(t *testing.T) {
	// opts==nil 时应初始化；但 QueryIPWithContext 会走真实网络（IANA bootstrap）。
	// 用 withDefaultClient + redirectDialer 重定向到本地伪服务器。
	addr, cleanup := fakeWhoisServer(t, "% This is the IANA whois server.\r\n")
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(addr))
	c.SetTimeout(time.Second)
	restoreDefault := withDefaultClient(c)
	defer restoreDefault()

	p := &DefaultIPWhoisProvider{}
	// 用一个超时上下文，确保不会挂死
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.QueryIP(ctx, "8.8.8.8", nil)
	// 真实路径会尝试解析，可能返回错误或空结果；只要不 panic、不挂死即可
	_ = err
}

// ---- DefaultASNProvider.QueryASN ----

func TestDefaultASNProvider_QueryASN(t *testing.T) {
	p := &DefaultASNProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// 走真实网络，期望超时/错误，不 panic
	_, err := p.QueryASN(ctx, &ASNQueryOptions{ASN: 15169})
	_ = err
}

// ---- DefaultRDAPProvider 各方法（默认走 HTTP） ----

func TestDefaultRDAPProvider_QueryDomain(t *testing.T) {
	p := &DefaultRDAPProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := p.QueryDomain(ctx, &RDAPQueryOptions{Domain: "invalid.example"})
	// 空 query 走校验错误分支
	_ = err
}

func TestDefaultRDAPProvider_QueryIP(t *testing.T) {
	p := &DefaultRDAPProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := p.QueryIP(ctx, &RDAPQueryOptions{})
	_ = err
}

func TestDefaultRDAPProvider_QueryASN(t *testing.T) {
	p := &DefaultRDAPProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := p.QueryASN(ctx, &RDAPQueryOptions{})
	_ = err
}

func TestDefaultRDAPProvider_QueryEntity(t *testing.T) {
	p := &DefaultRDAPProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := p.QueryEntity(ctx, &RDAPQueryOptions{})
	_ = err
}

// ---- DefaultWhoisQueryProvider.Query useProxy 分支 ----

func TestDefaultWhoisQueryProvider_Query_UseProxy(t *testing.T) {
	// 用 co.uk 域名：extractTLD("example.co.uk")="co.uk"，
	// getWhoisServer("co.uk")→GetWhoisServer("co.uk")→extractTLD("co.uk")="uk"，
	// 注册 servers["uk"] 健康即可走通 redirectDialer。
	addr, cleanup := fakeWhoisServer(t, "domain: example.co.uk\r\n")
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(addr))
	c.SetTimeout(3 * time.Second)
	restoreDefault := withDefaultClient(c)
	defer restoreDefault()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	p := &DefaultWhoisQueryProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := p.Query(ctx, "example.co.uk", "whois.example.com", true)
	assert.NoError(t, err)
	assert.Contains(t, resp, "example.co.uk")
}

func TestDefaultWhoisQueryProvider_Query_ContextCancelled(t *testing.T) {
	p := &DefaultWhoisQueryProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Query(ctx, "example.com", "", false)
	assert.Error(t, err)
}

// ---- DefaultWhoisQueryProvider.Parse ----

func TestDefaultWhoisQueryProvider_Parse(t *testing.T) {
	p := &DefaultWhoisQueryProvider{}
	// 空响应 → 解析错误
	_, err := p.Parse("")
	assert.Error(t, err)
}

// ---- stub error propagation ----

func TestStubIPWhoisProvider_Error(t *testing.T) {
	s := &stubIPWhoisProvider{err: errors.New("boom")}
	_, err := s.QueryIP(context.Background(), "1.2.3.4", nil)
	assert.Error(t, err)
}

func TestStubASNProvider_Error(t *testing.T) {
	s := &stubASNProvider{err: errors.New("boom")}
	_, err := s.QueryASN(context.Background(), nil)
	assert.Error(t, err)
}

func TestStubRDAPProvider_Error(t *testing.T) {
	s := &stubRDAPProvider{err: errors.New("boom")}
	_, err := s.QueryDomain(context.Background(), nil)
	assert.Error(t, err)
	_, err = s.QueryIP(context.Background(), nil)
	assert.Error(t, err)
	_, err = s.QueryASN(context.Background(), nil)
	assert.Error(t, err)
	_, err = s.QueryEntity(context.Background(), nil)
	assert.Error(t, err)
}
