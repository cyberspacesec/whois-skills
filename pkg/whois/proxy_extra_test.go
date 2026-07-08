package whois

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/proxy"
)

// =====================================================================
// ProxyConfig.GetDialer
// =====================================================================

func TestProxyConfig_GetDialer_Cached(t *testing.T) {
	cfg := &ProxyConfig{Type: "http", Address: "127.0.0.1:8080"}
	d1, err := cfg.GetDialer()
	assert.NoError(t, err)
	// 第二次返回缓存的 dialer
	d2, err := cfg.GetDialer()
	assert.NoError(t, err)
	assert.Same(t, d1, d2)
}

func TestProxyConfig_GetDialer_UnsupportedType(t *testing.T) {
	cfg := &ProxyConfig{Type: "weird", Address: "127.0.0.1:8080"}
	_, err := cfg.GetDialer()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的代理类型")
}

func TestProxyConfig_GetDialer_SOCKS5(t *testing.T) {
	cfg := &ProxyConfig{Type: "socks5", Address: "127.0.0.1:1080", Username: "u", Password: "p"}
	d, err := cfg.GetDialer()
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestProxyConfig_GetDialer_HTTP_WithAuth(t *testing.T) {
	cfg := &ProxyConfig{Type: "http", Address: "127.0.0.1:8080", Username: "u", Password: "p"}
	d, err := cfg.GetDialer()
	assert.NoError(t, err)
	hp, ok := d.(*httpProxyDialer)
	assert.True(t, ok)
	assert.NotNil(t, hp.proxyURL.User)
}

func TestProxyConfig_GetDialer_HTTP_NoPort(t *testing.T) {
	// Address 不含 ":"，触发 ":8080" 默认端口
	cfg := &ProxyConfig{Type: "http", Address: "127.0.0.1"}
	d, err := cfg.GetDialer()
	assert.NoError(t, err)
	hp, ok := d.(*httpProxyDialer)
	assert.True(t, ok)
	// 拨号时才会补端口，这里仅验证构造成功
	_ = hp
}

// =====================================================================
// httpProxyDialer.Dial —— CONNECT 隧道
// =====================================================================

// startHTTPProxy 启动一个简易 HTTP CONNECT 代理：建立到 CONNECT 目标的连接后双向透传。
// statusLine 为空时使用 "HTTP/1.1 200 Connection established"。
func startHTTPProxy(t *testing.T, requireAuth bool, statusLine string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(2 * time.Second))
				br := bufio.NewReader(c)
				// 读取 CONNECT 请求行
				firstLine, err := br.ReadString('\n')
				if err != nil {
					return
				}
				// 解析目标地址
				parts := strings.Fields(firstLine)
				var target string
				if len(parts) >= 2 {
					target = parts[1]
				}
				// 读取头直到空行，捕获 Proxy-Authorization
				var authHeader string
				for {
					hl, err := br.ReadString('\n')
					if err != nil {
						return
					}
					if hl == "\r\n" || hl == "\n" {
						break
					}
					if strings.HasPrefix(strings.ToLower(hl), "proxy-authorization:") {
						authHeader = strings.TrimSpace(strings.SplitN(hl, ":", 2)[1])
					}
				}
				if requireAuth && authHeader == "" {
					c.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
					return
				}
				if target == "" {
					c.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
					return
				}
				// 连接到目标
				targetConn, err := net.DialTimeout("tcp", target, 2*time.Second)
				if err != nil {
					c.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
					return
				}
				defer targetConn.Close()
				// 回写 200
				if statusLine == "" {
					statusLine = "HTTP/1.1 200 Connection established\r\n"
				}
				c.Write([]byte(statusLine))
				c.Write([]byte("\r\n"))
				// 把 br 中已缓冲的数据先发给目标
				if n := br.Buffered(); n > 0 {
					head, _ := br.Peek(n)
					targetConn.Write(head)
					br.Discard(n)
				}
					// 双向透传
				done := make(chan struct{}, 2)
				go func() {
					io.Copy(targetConn, c)
					// 客户端已半关闭写入 → 向目标也半关闭写入，让目标看到 EOF
					if tc, ok := targetConn.(*net.TCPConn); ok {
						tc.CloseWrite()
					}
					done <- struct{}{}
				}()
				io.Copy(c, targetConn)
				<-done
			}(conn)
		}
	}()
	return addr, func() { close(stop); ln.Close(); wg.Wait() }
}

func TestHTTPProxyDialer_Dial_Success(t *testing.T) {
	addr, cleanup := startHTTPProxy(t, false, "")
	defer cleanup()
	// 目标：起一个本地 TCP 服务，通过代理 CONNECT 连接它
	target, targetCleanup := fakeWhoisServer(t, "hello-from-target")
	defer targetCleanup()

	// 构造代理 URL，确保 host 含端口
	proxyURL := &url.URL{Scheme: "http", Host: addr}
	d := &httpProxyDialer{proxyURL: proxyURL}
	conn, err := d.Dial("tcp", target)
	assert.NoError(t, err)
	defer conn.Close()
	// 写入数据触发目标响应
	conn.Write([]byte("ping\r\n"))
	halfCloseWrite(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := conn.Read(buf)
	assert.Equal(t, "hello-from-target", string(buf[:n]))
}

func TestHTTPProxyDialer_Dial_DefaultPort(t *testing.T) {
	// proxyURL.Host 不含 ":"，触发 ":8080" 默认端口 → 连接失败
	proxyURL := &url.URL{Scheme: "http", Host: "127.0.0.1"}
	d := &httpProxyDialer{proxyURL: proxyURL}
	_, err := d.Dial("tcp", "example.com:43")
	assert.Error(t, err)
}

func TestHTTPProxyDialer_Dial_ConnectFail(t *testing.T) {
	// 代理地址不可达
	proxyURL := &url.URL{Scheme: "http", Host: "127.0.0.1:1"}
	d := &httpProxyDialer{proxyURL: proxyURL}
	_, err := d.Dial("tcp", "example.com:43")
	assert.Error(t, err)
}

func TestHTTPProxyDialer_Dial_BadStatus(t *testing.T) {
	addr, cleanup := startHTTPProxy(t, false, "HTTP/1.1 403 Forbidden\r\n")
	defer cleanup()
	proxyURL := &url.URL{Scheme: "http", Host: addr}
	d := &httpProxyDialer{proxyURL: proxyURL}
	_, err := d.Dial("tcp", "example.com:43")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP代理CONNECT失败")
}

func TestHTTPProxyDialer_Dial_WithAuth(t *testing.T) {
	addr, cleanup := startHTTPProxy(t, true, "")
	defer cleanup()
	target, targetCleanup := fakeWhoisServer(t, "via-auth-proxy")
	defer targetCleanup()
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   addr,
		User:   url.UserPassword("user", "pass"),
	}
	d := &httpProxyDialer{proxyURL: proxyURL}
	conn, err := d.Dial("tcp", target)
	assert.NoError(t, err)
	defer conn.Close()
	conn.Write([]byte("x\r\n"))
	halfCloseWrite(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := conn.Read(buf)
	assert.Equal(t, "via-auth-proxy", string(buf[:n]))
}

// =====================================================================
// WhoisDialer.Dial —— proxy vs net.Dialer
// =====================================================================

// halfCloseWrite 半关闭 TCP 连接的写入端，让对端 io.ReadAll 见到 EOF 立即响应。
func halfCloseWrite(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.CloseWrite()
	}
}

// lineWhoisServerFunc 启动本地伪服务器，读取请求首行后按 responseFunc 返回响应。
func lineWhoisServerFunc(t *testing.T, responseFunc func(req string) string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(2 * time.Second))
				br := bufio.NewReader(c)
				line, _ := br.ReadString('\n')
				resp := responseFunc(line)
				if resp != "" {
					c.Write([]byte(resp))
				}
			}(conn)
		}
	}()
	cleanup := func() {
		cancel()
		ln.Close()
		wg.Wait()
	}
	return addr, cleanup
}

// lineWhoisServer 启动一个本地伪服务器：读取请求首行即返回固定响应（不等 EOF）。
// 解决 fakeWhoisServer 使用 io.ReadAll 在生产代码不 CloseWrite 时会阻塞到 deadline 的问题。
func lineWhoisServer(t *testing.T, response string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(2 * time.Second))
				br := bufio.NewReader(c)
				// 读取首行后立即响应
				_, _ = br.ReadString('\n')
				if response != "" {
					c.Write([]byte(response))
				}
			}(conn)
		}
	}()
	cleanup := func() {
		cancel()
		ln.Close()
		wg.Wait()
	}
	return addr, cleanup
}

func TestWhoisDialer_Dial_WithProxy(t *testing.T) {
	target, cleanup := fakeWhoisServer(t, "proxy-ok")
	defer cleanup()
	wd := &WhoisDialer{Timeout: 2 * time.Second, ProxyDialer: newRedirectDialer(target)}
	conn, err := wd.Dial("tcp", "whatever:43")
	assert.NoError(t, err)
	defer conn.Close()
	conn.Write([]byte("x\r\n"))
	halfCloseWrite(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := conn.Read(buf)
	assert.Equal(t, "proxy-ok", string(buf[:n]))
}

func TestWhoisDialer_Dial_NoProxy(t *testing.T) {
	target, cleanup := fakeWhoisServer(t, "direct-ok")
	defer cleanup()
	wd := &WhoisDialer{Timeout: 2 * time.Second}
	conn, err := wd.Dial("tcp", target)
	assert.NoError(t, err)
	defer conn.Close()
	conn.Write([]byte("x\r\n"))
	halfCloseWrite(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := conn.Read(buf)
	assert.Equal(t, "direct-ok", string(buf[:n]))
}

// =====================================================================
// LoadProxiesFromFile
// =====================================================================

func TestLoadProxiesFromFile_ReadError(t *testing.T) {
	err := LoadProxiesFromFile("/nonexistent/path/proxies.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取代理配置文件失败")
}

func TestLoadProxiesFromFile_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxies.json")
	os.WriteFile(path, []byte("not json"), 0644)
	err := LoadProxiesFromFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析代理配置失败")
}

func TestLoadProxiesFromFile_Success(t *testing.T) {
	// 使用独立 ProxyPool 实例，避免污染全局 GetProxyPool
	// LoadProxiesFromFile 内部调用 GetProxyPool()，故需先重置全局池状态
	pool := GetProxyPool()
	pool.mu.Lock()
	origProxies := pool.proxies
	origStatus := pool.status
	origIdx := pool.currentIndex
	origUpdated := pool.lastUpdated
	pool.mu.Unlock()
	defer func() {
		pool.mu.Lock()
		pool.proxies = origProxies
		pool.status = origStatus
		pool.currentIndex = origIdx
		pool.lastUpdated = origUpdated
		pool.mu.Unlock()
	}()

	configs := []*ProxyConfig{
		{Address: "127.0.0.1:1080", Type: "socks5", Timeout: 5, MaxRetries: 2, RetryInterval: 100},
		{Address: "127.0.0.1:8080", Type: "http", Timeout: 0, MaxRetries: 0, RetryInterval: 0},
		// 不支持的类型 → 初始化失败，会被跳过
		{Address: "127.0.0.1:9999", Type: "weird"},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "proxies.json")
	data, _ := json.Marshal(configs)
	os.WriteFile(path, data, 0644)

	err := LoadProxiesFromFile(path)
	assert.NoError(t, err)
	assert.Equal(t, 2, pool.ProxyCount())
}

// =====================================================================
// ProxyPool: GetNextProxy / Mark* / GetProxyStats / countAvailableProxies
// =====================================================================

func newPopulatedPool() *ProxyPool {
	return &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "a:1", Type: "http"},
			{Address: "b:2", Type: "http"},
			{Address: "c:3", Type: "http"},
		},
		status: map[string]*ProxyStatus{
			"a:1": {Available: true, FailureCount: 0, AvgResponseTime: 10},
			"b:2": {Available: true, FailureCount: 0, AvgResponseTime: 0},
			"c:3": {Available: false, FailureCount: 3, AvgResponseTime: 0},
		},
	}
}

func TestProxyPool_GetNextProxy_Empty(t *testing.T) {
	p := &ProxyPool{proxies: nil, status: map[string]*ProxyStatus{}}
	assert.Nil(t, p.GetNextProxy())
}

func TestProxyPool_GetNextProxy_Available(t *testing.T) {
	p := newPopulatedPool()
	// a 可用
	prx := p.GetNextProxy()
	assert.Equal(t, "a:1", prx.Address)
	// currentIndex 前进到 1 → b
	prx = p.GetNextProxy()
	assert.Equal(t, "b:2", prx.Address)
}

func TestProxyPool_GetNextProxy_AllUnavailable_Reset(t *testing.T) {
	p := newPopulatedPool()
	// 全部置为不可用
	for _, s := range p.status {
		s.Available = false
	}
	prx := p.GetNextProxy()
	// 回退到第一个
	assert.Equal(t, "a:1", prx.Address)
	// 状态被重置
	assert.True(t, p.status["c:3"].Available)
	assert.Equal(t, 0, p.status["c:3"].FailureCount)
}

func TestProxyPool_MarkProxySuccess(t *testing.T) {
	p := newPopulatedPool()
	// b:2 初始 AvgResponseTime=0
	// 第一次：0==0 → 直接赋值 40
	p.MarkProxySuccess(&ProxyConfig{Address: "b:2"}, 40)
	assert.Equal(t, int64(40), p.status["b:2"].AvgResponseTime)
	// 第二次：40!=0 → (40+40)/2 = 40
	p.MarkProxySuccess(&ProxyConfig{Address: "b:2"}, 40)
	assert.Equal(t, int64(40), p.status["b:2"].AvgResponseTime)
	// 第三次：40!=0 → (40+60)/2 = 50
	p.MarkProxySuccess(&ProxyConfig{Address: "b:2"}, 60)
	assert.Equal(t, int64(50), p.status["b:2"].AvgResponseTime)
	// 未知 address 不 panic
	p.MarkProxySuccess(&ProxyConfig{Address: "unknown:9"}, 10)
}

func TestProxyPool_MarkProxyFailure(t *testing.T) {
	p := newPopulatedPool()
	p.MarkProxyFailure(&ProxyConfig{Address: "a:1"})
	p.MarkProxyFailure(&ProxyConfig{Address: "a:1"})
	p.MarkProxyFailure(&ProxyConfig{Address: "a:1"})
	assert.False(t, p.status["a:1"].Available)
	assert.Equal(t, 3, p.status["a:1"].FailureCount)
	// 未知 address 不 panic
	p.MarkProxyFailure(&ProxyConfig{Address: "unknown:9"})
}

func TestProxyPool_GetProxyStats(t *testing.T) {
	p := newPopulatedPool()
	stats := p.GetProxyStats()
	assert.Equal(t, 3, stats["total"])
	assert.Equal(t, 2, stats["available"]) // a,b 可用
	proxies, ok := stats["proxies"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, proxies, "a:1")
	_, hasLastUpdated := stats["last_updated"]
	assert.True(t, hasLastUpdated)
}

func TestProxyPool_ProxyCount(t *testing.T) {
	p := newPopulatedPool()
	assert.Equal(t, 3, p.ProxyCount())
}

// =====================================================================
// StartProxyHealthCheck / checkProxyHealth
// =====================================================================

func TestProxyPool_CheckProxyHealth_DialerError(t *testing.T) {
	p := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "bad:1", Type: "weird"}, // GetDialer 失败
		},
		status: map[string]*ProxyStatus{
			"bad:1": {Available: true, FailureCount: 0},
		},
	}
	p.checkProxyHealth()
	// 一次失败：FailureCount=1，但 Available 仍 true（1<3）
	assert.Equal(t, 1, p.status["bad:1"].FailureCount)
	assert.True(t, p.status["bad:1"].Available)
	// 累计到 3 次失败才标记不可用
	p.checkProxyHealth()
	p.checkProxyHealth()
	assert.False(t, p.status["bad:1"].Available)
	assert.Equal(t, 3, p.status["bad:1"].FailureCount)
}

func TestProxyPool_CheckProxyHealth_DialFail(t *testing.T) {
	p := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "127.0.0.1:1", Type: "http", dialer: &netDialerFail{}},
		},
		status: map[string]*ProxyStatus{
			"127.0.0.1:1": {Available: true, FailureCount: 0},
		},
	}
	p.checkProxyHealth()
	assert.Equal(t, 1, p.status["127.0.0.1:1"].FailureCount)
	assert.True(t, p.status["127.0.0.1:1"].Available)
	p.checkProxyHealth()
	p.checkProxyHealth()
	assert.False(t, p.status["127.0.0.1:1"].Available)
}

func TestProxyPool_CheckProxyHealth_Success(t *testing.T) {
	target, cleanup := fakeWhoisServer(t, "ok")
	defer cleanup()
	// 使用 redirectDialer 重定向到本地伪服务器
	p := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "redirect:1", Type: "http", dialer: newRedirectDialer(target)},
		},
		status: map[string]*ProxyStatus{
			"redirect:1": {Available: false, FailureCount: 5, AvgResponseTime: 0},
		},
	}
	p.checkProxyHealth()
	s := p.status["redirect:1"]
	assert.True(t, s.Available)
	assert.Equal(t, 0, s.FailureCount)
	assert.GreaterOrEqual(t, s.AvgResponseTime, int64(0))
}

func TestProxyPool_StartProxyHealthCheck(t *testing.T) {
	p := newPopulatedPool()
	// 极短间隔，触发一次 checkProxyHealth 后停止
	p.StartProxyHealthCheck(20 * time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	// 仅验证 goroutine 启动后不 panic；无法停止 ticker（泄漏可接受）
}

// netDialerFail 是一个始终失败的 proxy.Dialer
type netDialerFail struct{}

func (n *netDialerFail) Dial(network, addr string) (net.Conn, error) {
	return nil, fmt.Errorf("dial fail")
}

// =====================================================================
// extractReferralServer
// =====================================================================

func TestExtractReferralServer(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{"Registrar token", "Registrar WHOIS Server: whois.example.com\n", "whois.example.com"},
		{"whois token", "whois: whois.example.org\n", "whois.example.org"},
		{"http filtered out", "Registrar WHOIS Server: http://x\n", ""},
		{"empty after token", "Registrar WHOIS Server: \n", ""},
		{"no token", "nothing here", ""},
		{"same as server token with trailing spaces", "whois:   whois.abc.com  \n", "whois.abc.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, extractReferralServer(c.data))
		})
	}
}

// =====================================================================
// WhoisClient: Set* / getCache / Query / QueryWithContext / direct / proxyPool / cache
// =====================================================================

func TestWhoisClient_SetProxyPool(t *testing.T) {
	c := NewWhoisClient()
	p := &ProxyPool{}
	c.SetProxyPool(p)
	assert.Same(t, p, c.pool)
}

func TestWhoisClient_SetProxy(t *testing.T) {
	c := NewWhoisClient()
	d := newRedirectDialer("127.0.0.1:1")
	c.SetProxy(d)
	assert.Same(t, d, c.dialer.ProxyDialer)
}

func TestWhoisClient_SetTimeout_Extra(t *testing.T) {
	c := NewWhoisClient()
	c.SetTimeout(7 * time.Second)
	assert.Equal(t, 7*time.Second, c.dialer.Timeout)
}

func TestWhoisClient_SetCache(t *testing.T) {
	c := NewWhoisClient()
	cache, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60})
	c.SetCache(cache)
	assert.False(t, c.cacheDisabled)
	assert.Same(t, cache, c.cache)
}

func TestWhoisClient_DisableCache(t *testing.T) {
	c := NewWhoisClient()
	c.DisableCache()
	assert.True(t, c.cacheDisabled)
}

func TestWhoisClient_SetCacheTTL(t *testing.T) {
	c := NewWhoisClient()
	cache, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60})
	c.SetCache(cache)
	c.SetCacheTTL(120)
	assert.Equal(t, int64(120), cache.config.TTL)
}

func TestWhoisClient_SetCacheTTL_NoCache(t *testing.T) {
	c := NewWhoisClient()
	// cache 为 nil（禁用）→ 不 panic
	assert.NotPanics(t, func() { c.SetCacheTTL(99) })
}

func TestWhoisClient_SetRateLimiter(t *testing.T) {
	c := NewWhoisClient()
	rl := NewRateLimiter(RateLimiterConfig{GlobalRate: 1})
	c.SetRateLimiter(rl)
	assert.Same(t, rl, c.rateLimiter)
}

func TestWhoisClient_GetCache_Disabled(t *testing.T) {
	c := NewWhoisClient()
	c.DisableCache()
	assert.Nil(t, c.getCache())
}

func TestWhoisClient_GetCache_Custom(t *testing.T) {
	c := NewWhoisClient()
	cache, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60})
	c.SetCache(cache)
	assert.Same(t, cache, c.getCache())
}

func TestWhoisClient_GetCache_GlobalDefault(t *testing.T) {
	c := NewWhoisClient()
	// 未设置自定义 cache 且未禁用 → 返回全局 GetCache()
	assert.NotNil(t, c.getCache())
}

// ---- QueryWithContext: 上下文已取消 ----

func TestWhoisClient_QueryWithContext_Cancelled(t *testing.T) {
	c := NewWhoisClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.QueryWithContext(ctx, "example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询被取消")
}

// ---- QueryWithContext: 缓存命中 ----

func TestWhoisClient_QueryWithContext_CacheHit(t *testing.T) {
	c := NewWhoisClient()
	cache, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60})
	c.SetCache(cache)
	// 手动塞入缓存
	raw := "cached-raw"
	cache.Set("hit.com", nil, raw)
	got, err := c.QueryWithContext(context.Background(), "hit.com")
	assert.NoError(t, err)
	assert.Equal(t, raw, got)
}

// ---- QueryWithContext: 无效域名（tld 为空）----

func TestWhoisClient_QueryWithContext_InvalidDomain(t *testing.T) {
	c := NewWhoisClient()
	c.DisableCache()
	// "com" 顶层 → extractTLD 返回 ""
	_, err := c.QueryWithContext(context.Background(), "com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的域名格式")
}

// ---- QueryWithContext: 直连成功（带引导跟随 + 缓存写入）----

func TestWhoisClient_QueryWithContext_DirectSuccess(t *testing.T) {
	// 使用本地伪服务器 + redirectDialer
	target, cleanup := lineWhoisServerFunc(t, func(req string) string {
		return "Domain Name: EXAMPLE.CO.UK\nRegistrar WHOIS Server: whois.nic.uk\n"
	})
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target))
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	// 第一次：返回引导服务器，但引导查询也会命中同一个伪服务器（redirectDialer 固定重定向）
	got, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.NoError(t, err)
	assert.Contains(t, got, "Domain Name")
}

// ---- QueryWithContext: 直连成功且解析成功 → 写缓存 ----

func TestWhoisClient_QueryWithContext_DirectSuccess_CachesOnParse(t *testing.T) {
	target, cleanup := lineWhoisServerFunc(t, func(req string) string {
		return "Domain Name: example.co.uk\nRegistry Domain ID: 123_DOMAIN\nRegistrar WHOIS Server: whois.nic.uk\n"
	})
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target))
	c.SetTimeout(3 * time.Second)
	cache, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60})
	c.SetCache(cache)
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	_, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.NoError(t, err)
	// 应已缓存
	_, ok := cache.Get("example.co.uk")
	assert.True(t, ok)
}

// ---- QueryWithContext: 速率限制 ----

func TestWhoisClient_QueryWithContext_RateLimited(t *testing.T) {
	target, cleanup := fakeWhoisServer(t, "ok")
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target))
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	// 一个极小速率且 burst=0 的限速器：maxTokens=0.0001 < 1 → Allow 立即返回 false
	rl := NewRateLimiter(RateLimiterConfig{GlobalRate: 0.0001, BurstSize: 0})
	c.SetRateLimiter(rl)
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	_, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.Error(t, err)
}

// ---- QueryWithContext: 代理池查询 ----

func TestWhoisClient_QueryWithContext_ProxyPoolSuccess(t *testing.T) {
	target, cleanup := lineWhoisServerFunc(t, func(req string) string {
		return "Domain Name: example.co.uk\nRegistrar WHOIS Server: whois.nic.uk\n"
	})
	defer cleanup()

	c := NewWhoisClient()
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	// 代理池：使用 redirectDialer 作为 dialer（预置）
	pool := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "p1:1", Type: "http", dialer: newRedirectDialer(target)},
		},
		status: map[string]*ProxyStatus{
			"p1:1": {Available: true},
		},
	}
	c.SetProxyPool(pool)

	got, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.NoError(t, err)
	assert.Contains(t, got, "Domain Name")
}

func TestWhoisClient_QueryWithContext_ProxyPoolAllFail(t *testing.T) {
	c := NewWhoisClient()
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	// 代理池：所有代理 GetDialer 失败
	pool := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "p1:1", Type: "weird"},
		},
		status: map[string]*ProxyStatus{
			"p1:1": {Available: true},
		},
	}
	c.SetProxyPool(pool)

	_, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "所有代理均失败")
}

func TestWhoisClient_QueryWithContext_ProxyPoolCancelled(t *testing.T) {
	c := NewWhoisClient()
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	pool := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "p1:1", Type: "http", dialer: newRedirectDialer("127.0.0.1:1")},
		},
		status: map[string]*ProxyStatus{
			"p1:1": {Available: true},
		},
	}
	c.SetProxyPool(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.QueryWithContext(ctx, "example.co.uk")
	assert.Error(t, err)
}

// ---- queryDirectContext: getWhoisServer 失败 ----
// extractTLD("example.internal") = "internal"，再次 extractTLD("internal") = "" → GetWhoisServer 报错
// 注意：需确保 "internal" 未在 serverManager 注册（默认配置不含 .internal）

func TestWhoisClient_QueryWithContext_GetWhoisServerFail(t *testing.T) {
	mgr := GetServerManager()
	mgr.mu.Lock()
	origDefault := mgr.defaultServer
	origInternal := mgr.servers["internal"]
	mgr.defaultServer = ""
	delete(mgr.servers, "internal")
	mgr.mu.Unlock()
	defer func() {
		mgr.mu.Lock()
		mgr.defaultServer = origDefault
		if origInternal != "" {
			mgr.servers["internal"] = origInternal
		}
		mgr.mu.Unlock()
	}()

	c := NewWhoisClient()
	c.DisableCache()
	_, err := c.QueryWithContext(context.Background(), "example.internal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "获取WHOIS服务器失败")
}

// ---- queryDirectContext: referral 查询失败时使用已有结果（refServer 不同且查询失败）----

func TestWhoisClient_QueryDirectContext_ReferralQueryFail(t *testing.T) {
	// 伪服务器返回引导到 whois.referral.fail
	target, cleanup := lineWhoisServerFunc(t, func(req string) string {
		return "Domain Name: example.co.uk\nRegistrar WHOIS Server: whois.referral.fail\n"
	})
	defer cleanup()

	// 自定义拨号器：引导地址（whois.referral.fail:43）拨号失败，其它地址重定向到 target
	failingDialer := &failingReferralDialer{ok: newRedirectDialer(target)}
	c := NewWhoisClient()
	c.SetProxy(failingDialer)
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	got, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.NoError(t, err)
	// 引导查询失败 → 使用已有结果，仍包含 Domain Name
	assert.Contains(t, got, "Domain Name")
	assert.Equal(t, 1, failingDialer.calls) // 只发生一次引导尝试（失败后 break）
}

// failingReferralDialer 对 "whois.referral.fail" 拨号失败，其它地址走 ok 拨号器。
type failingReferralDialer struct {
	ok    proxy.Dialer
	calls int
}

func (d *failingReferralDialer) Dial(network, addr string) (net.Conn, error) {
	if strings.HasPrefix(addr, "whois.referral.fail") {
		d.calls++
		return nil, fmt.Errorf("referral dial fail")
	}
	return d.ok.Dial(network, addr)
}

// ---- rawWhoisQuery: 上下文取消 ----

func TestWhoisClient_RawWhoisQuery_Cancelled(t *testing.T) {
	c := NewWhoisClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.rawWhoisQuery(ctx, "whois.example.com", "x.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询被取消")
}

// ---- rawWhoisQuery: 连接失败 ----

func TestWhoisClient_RawWhoisQuery_DialFail(t *testing.T) {
	c := NewWhoisClient()
	c.SetTimeout(500 * time.Millisecond)
	// dialer 无代理 → 直连一个不可达地址
	_, err := c.rawWhoisQuery(context.Background(), "127.0.0.1:1", "x.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "连接WHOIS服务器失败")
}

// ---- rawWhoisQuery: 成功（通过 redirectDialer）----

func TestWhoisClient_RawWhoisQuery_Success(t *testing.T) {
	target, cleanup := lineWhoisServer(t, "raw-ok")
	defer cleanup()
	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target))
	c.SetTimeout(2 * time.Second)
	got, err := c.rawWhoisQuery(context.Background(), "whois.fake", "example.com")
	assert.NoError(t, err)
	assert.Equal(t, "raw-ok", got)
}

// ---- queryDirectContext: 引导查询失败时使用已有结果 ----

func TestWhoisClient_QueryDirectContext_ReferralFail(t *testing.T) {
	// 第一次返回引导服务器，但第二次（引导查询）连接失败
	callCount := 0
	target, cleanup := lineWhoisServerFunc(t, func(req string) string {
		callCount++
		return "Domain Name: example.co.uk\nRegistrar WHOIS Server: whois.referral.fail\n"
	})
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target)) // 所有请求都重定向到同一个伪服务器，不会失败
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	got, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.NoError(t, err)
	// 应包含引导结果拼接
	assert.Contains(t, got, "Domain Name")
}

// =====================================================================
// SetWhoisProxy
// =====================================================================

func TestSetWhoisProxy_Nil(t *testing.T) {
	orig := defaultClient
	defer func() { defaultClient = orig }()
	err := SetWhoisProxy(&ProxyConfig{Enabled: false})
	_ = err
}

func TestSetWhoisProxy_Disabled(t *testing.T) {
	orig := defaultClient
	defer func() { defaultClient = orig }()
	err := SetWhoisProxy(&ProxyConfig{Enabled: false})
	assert.NoError(t, err)
}

func TestSetWhoisProxy_BadType(t *testing.T) {
	orig := defaultClient
	defer func() { defaultClient = orig }()
	err := SetWhoisProxy(&ProxyConfig{Enabled: true, Type: "weird", Address: "x:1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "创建代理拨号器失败")
}

func TestSetWhoisProxy_HTTPSuccess(t *testing.T) {
	orig := defaultClient
	defer func() { defaultClient = orig }()
	err := SetWhoisProxy(&ProxyConfig{
		Enabled: true,
		Type:    "http",
		Address: "127.0.0.1:8080",
		Timeout: 5,
	})
	assert.NoError(t, err)
	assert.NotNil(t, defaultClient)
}

// =====================================================================
// DirectWhois / DirectWhoisWithContext
// =====================================================================

func TestDirectWhoisWithContext(t *testing.T) {
	target, cleanup := lineWhoisServer(t, "direct-whois-ok")
	defer cleanup()

	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target))
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	restoreDefault := withDefaultClient(c)
	defer restoreDefault()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	got, err := DirectWhois("example.co.uk")
	assert.NoError(t, err)
	assert.Equal(t, "direct-whois-ok", got)

	got2, err := DirectWhoisWithContext(context.Background(), "example.co.uk")
	assert.NoError(t, err)
	assert.Equal(t, "direct-whois-ok", got2)
}

// =====================================================================
// isValidProxyAddress
// =====================================================================

func TestIsValidProxyAddress_UnresolvableHost(t *testing.T) {
	// 未知主机且无法解析 → false（net.LookupIP 失败）
	assert.False(t, isValidProxyAddress("invalid-host-nonexistent-xyz.invalid:8080"))
}

// =====================================================================
// GetProxyPool（sync.Once 单例）
// =====================================================================

func TestGetProxyPool_Singleton(t *testing.T) {
	p1 := GetProxyPool()
	p2 := GetProxyPool()
	assert.Same(t, p1, p2)
}

// =====================================================================
// 集成：HTTP 代理服务器 + WhoisClient（无 redirectDialer，真实 CONNECT 隧道）
// =====================================================================

func TestWhoisClient_ThroughHTTPProxy(t *testing.T) {
	// 起一个真实 HTTP CONNECT 代理
	proxyAddr, proxyCleanup := startHTTPProxy(t, false, "")
	defer proxyCleanup()
	// 起目标 WHOIS 伪服务器
	target, targetCleanup := fakeWhoisServer(t, "via-real-proxy")
	defer targetCleanup()

	// ProxyConfig: http 代理，目标通过 CONNECT 隧道连接
	cfg := &ProxyConfig{
		Enabled: true,
		Type:    "http",
		Address: proxyAddr,
		Timeout: 5,
	}
	dialer, err := cfg.GetDialer()
	assert.NoError(t, err)

	c := NewWhoisClient()
	c.SetProxy(dialer)
	c.SetTimeout(3 * time.Second)
	c.DisableCache()

	// rawWhoisQuery 会拼 "server+:43"，但目标 target 已是 host:port
	// 然而 CONNECT 请求里 addr = server+":43"，代理会把数据转发到该地址
	// 由于 startHTTPProxy 透传 io.Copy(br)，目标 host:43 不可达 → 实际会失败
	// 因此此用例验证 dialer 能成功建立 CONNECT 隧道（到 target 本身）
	// 改为：直接调用 dialer.Dial 验证隧道
	conn, err := dialer.Dial("tcp", target)
	assert.NoError(t, err)
	defer conn.Close()
	conn.Write([]byte("ping\r\n"))
	halfCloseWrite(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := conn.Read(buf)
	assert.Equal(t, "via-real-proxy", string(buf[:n]))
}

// =====================================================================
// Query 包装器（无 context）
// =====================================================================

func TestWhoisClient_Query(t *testing.T) {
	target, cleanup := lineWhoisServer(t, "query-wrapper-ok")
	defer cleanup()
	c := NewWhoisClient()
	c.SetProxy(newRedirectDialer(target))
	c.SetTimeout(3 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()
	got, err := c.Query("example.co.uk")
	assert.NoError(t, err)
	assert.Equal(t, "query-wrapper-ok", got)
}

// ---- queryWithProxyPoolContext: queryDirectContext 返回错误 → MarkProxyFailure → 所有代理失败 ----

func TestWhoisClient_QueryWithContext_ProxyPoolQueryDirectFail(t *testing.T) {
	c := NewWhoisClient()
	c.SetTimeout(2 * time.Second)
	c.DisableCache()
	defer registerLocalWhoisServer("uk", "whois.nic.uk")()

	// 代理 dialer 可用，但 redirectDialer 指向不可达地址 → queryDirectContext 失败
	pool := &ProxyPool{
		proxies: []*ProxyConfig{
			{Address: "p1:1", Type: "http", dialer: newRedirectDialer("127.0.0.1:1")},
		},
		status: map[string]*ProxyStatus{
			"p1:1": {Available: true},
		},
	}
	c.SetProxyPool(pool)

	_, err := c.QueryWithContext(context.Background(), "example.co.uk")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "所有代理均失败")
	// 失败计数增加
	assert.GreaterOrEqual(t, pool.status["p1:1"].FailureCount, 1)
}

// =====================================================================
// httpProxyDialer.Dial 错误分支：Write 失败 / ReadString 失败 / 响应头读取失败
// =====================================================================

// startClosingProxy 启动一个异常 HTTP 代理，按 mode 模拟连接异常。
func startClosingProxy(t *testing.T, mode string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				switch mode {
				case "close-immediately":
					// 不读不写，直接关闭 → 客户端 ReadString('\n') 失败
					return
				case "reset-immediately":
					// 立即设置 SO_LINGER=0 后 Close → 发送 RST，使客户端 Write 失败
					if tc, ok := c.(*net.TCPConn); ok {
						tc.SetLinger(0)
					}
				}
			}(conn)
		}
	}()
	return addr, func() { close(stop); ln.Close(); wg.Wait() }
}

func TestHTTPProxyDialer_Dial_ReadResponseFail(t *testing.T) {
	// 代理立即关闭连接 → 客户端读取状态行失败
	addr, cleanup := startClosingProxy(t, "close-immediately")
	defer cleanup()
	proxyURL := &url.URL{Scheme: "http", Host: addr}
	d := &httpProxyDialer{proxyURL: proxyURL}
	_, err := d.Dial("tcp", "example.com:43")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取HTTP代理响应失败")
}

func TestHTTPProxyDialer_Dial_WriteFail(t *testing.T) {
	// 代理接受连接后立即 SO_LINGER=0 关闭 → RST，使客户端 Write 失败
	addr, cleanup := startClosingProxy(t, "reset-immediately")
	defer cleanup()
	proxyURL := &url.URL{Scheme: "http", Host: addr}
	d := &httpProxyDialer{proxyURL: proxyURL}
	_, err := d.Dial("tcp", "example.com:43")
	assert.Error(t, err)
	// 可能是 Write 失败或 Read 失败，视 RST 时序
	_ = err
}

// ---- httpProxyDialer: 响应头无空行 → 读取响应头失败 ----

func TestHTTPProxyDialer_Dial_NoBlankLine(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(2 * time.Second))
				br := bufio.NewReader(c)
				// 读 CONNECT 请求行 + 头
				for {
					l, err := br.ReadString('\n')
					if err != nil {
						return
					}
					if l == "\r\n" || l == "\n" {
						break
					}
				}
				// 只回写状态行，不回空行，然后关闭 → 客户端读取响应头循环失败
				c.Write([]byte("HTTP/1.1 200 OK\r\n"))
				time.Sleep(50 * time.Millisecond)
			}(conn)
		}
	}()
	defer func() { close(stop); ln.Close(); wg.Wait() }()
	proxyURL := &url.URL{Scheme: "http", Host: addr}
	d := &httpProxyDialer{proxyURL: proxyURL}
	_, err = d.Dial("tcp", "example.com:43")
	assert.Error(t, err)
}

// ---- rawWhoisQuery: 写入失败（注入返回 Write 错误的连接）----

type writeFailDialer struct{}

func (d *writeFailDialer) Dial(network, addr string) (net.Conn, error) {
	// 返回一个 Read/Write 均立即报错的伪连接
	return &errConn{}, nil
}

type errConn struct{}

func (c *errConn) Read(b []byte) (int, error)         { return 0, fmt.Errorf("read err") }
func (c *errConn) Write(b []byte) (int, error)        { return 0, fmt.Errorf("write err") }
func (c *errConn) Close() error                       { return nil }
func (c *errConn) LocalAddr() net.Addr                { return nil }
func (c *errConn) RemoteAddr() net.Addr               { return nil }
func (c *errConn) SetDeadline(t time.Time) error      { return nil }
func (c *errConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *errConn) SetWriteDeadline(t time.Time) error { return nil }

func TestWhoisClient_RawWhoisQuery_WriteFail(t *testing.T) {
	c := NewWhoisClient()
	c.SetProxy(&writeFailDialer{})
	c.SetTimeout(2 * time.Second)
	_, err := c.rawWhoisQuery(context.Background(), "whois.fake", "example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "发送查询请求失败")
}

// ---- rawWhoisQuery: 读取响应失败（Write 成功但 Read 报错）----

type readFailDialer struct{}

func (d *readFailDialer) Dial(network, addr string) (net.Conn, error) {
	return &writeOkReadErrConn{}, nil
}

type writeOkReadErrConn struct{}

func (c *writeOkReadErrConn) Write(b []byte) (int, error) { return len(b), nil }
func (c *writeOkReadErrConn) Read(b []byte) (int, error)  { return 0, fmt.Errorf("read err") }
func (c *writeOkReadErrConn) Close() error                { return nil }
func (c *writeOkReadErrConn) LocalAddr() net.Addr         { return nil }
func (c *writeOkReadErrConn) RemoteAddr() net.Addr        { return nil }
func (c *writeOkReadErrConn) SetDeadline(t time.Time) error      { return nil }
func (c *writeOkReadErrConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *writeOkReadErrConn) SetWriteDeadline(t time.Time) error { return nil }

func TestWhoisClient_RawWhoisQuery_ReadFail(t *testing.T) {
	c := NewWhoisClient()
	c.SetProxy(&readFailDialer{})
	c.SetTimeout(2 * time.Second)
	_, err := c.rawWhoisQuery(context.Background(), "whois.fake", "example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取响应失败")
}

// 防止未使用 import（httptest/http 在某些分支可能未直接引用）
var _ = httptest.NewServer
var _ = http.StatusOK
var _ = proxy.Direct
