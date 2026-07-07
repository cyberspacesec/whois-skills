package whois

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"golang.org/x/net/proxy"
)

// fakeWhoisServer 启动一个本地 TCP 伪服务器：对每个连接回写 response 后关闭。
// 返回 host:port 形式地址与 cleanup。responseFunc 可为 nil（使用固定 response）。
func fakeWhoisServer(t *testing.T, response string) (string, func()) {
	t.Helper()
	return fakeWhoisServerFunc(t, func(string) string { return response })
}

// fakeWhoisServerFunc 启动本地伪服务器，按请求内容返回响应。
func fakeWhoisServerFunc(t *testing.T, responseFunc func(req string) string) (string, func()) {
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
				req, _ := io.ReadAll(c)
				resp := responseFunc(string(req))
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

// redirectDialer 是一个 proxy.Dialer 实现：无论请求哪个 addr，都连接固定的本地地址。
// 用于绕过 rawWhoisQuery 中 "server+:43" 的硬编码端口拼接，将 WHOIS 查询重定向到本地伪服务器。
type redirectDialer struct {
	addr string
}

func (d *redirectDialer) Dial(network, addr string) (net.Conn, error) {
	return net.DialTimeout("tcp", d.addr, 2*time.Second)
}

// newRedirectDialer 返回一个重定向到本地 addr 的 proxy.Dialer。
func newRedirectDialer(addr string) proxy.Dialer {
	return &redirectDialer{addr: addr}
}

// registerLocalWhoisServer 把 tld 映射到 server，并预置健康状态，返回恢复函数。
// 注意：rawWhoisQuery 会拼接 "server+:43"，故实际查询需配合 newRedirectDialer 注入 client。
// 本 helper 仅用于让 GetWhoisServer 返回非空且健康。
func registerLocalWhoisServer(tld, server string) func() {
	mgr := GetServerManager()
	mgr.mu.Lock()
	origServer := mgr.servers[tld]
	origHealth := mgr.serverHealth[server]
	origDefault := mgr.defaultServer
	mgr.mu.Unlock()

	mgr.UpdateServer(tld, server)
	mgr.mu.Lock()
	mgr.serverHealth[server] = &ServerHealth{IsHealthy: true, maxResponseRecords: 10}
	mgr.mu.Unlock()

	return func() {
		mgr.mu.Lock()
		mgr.servers[tld] = origServer
		if origHealth == nil {
			delete(mgr.serverHealth, server)
		} else {
			mgr.serverHealth[server] = origHealth
		}
		mgr.defaultServer = origDefault
		mgr.mu.Unlock()
	}
}

// withStubQueryProvider 临时替换全局 WhoisQueryProvider，返回恢复函数。
func withStubQueryProvider(p WhoisQueryProvider) func() {
	orig := globalWhoisQueryProvider
	globalWhoisQueryProvider = p
	return func() { globalWhoisQueryProvider = orig }
}

// withDefaultClient 临时替换 defaultClient，返回恢复函数。
func withDefaultClient(c *WhoisClient) func() {
	orig := defaultClient
	defaultClient = c
	return func() { defaultClient = orig }
}

// newMiniredis 启动内存 Redis，返回地址与 cleanup。
func newMiniredis(t *testing.T) (string, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	return mr.Addr(), mr.Close
}
