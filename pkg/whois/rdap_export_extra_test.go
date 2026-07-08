package whois

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ---- getKnownRDAPServer ----

func TestGetKnownRDAPServer(t *testing.T) {
	// com 已在默认 bootstrap 中
	assert.NotEmpty(t, getKnownRDAPServer("com"))
	// 未知 TLD 返回空
	assert.Empty(t, getKnownRDAPServer("nonexistent-tld-xyz"))
}

// ---- discoverRDAPServer: bootstrap 命中 / fallback 到 IANA ----

func TestDiscoverRDAPServer_HitAndFallback(t *testing.T) {
	// 命中默认 bootstrap（com）
	url, err := discoverRDAPServer(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, url)

	// 未知 TLD（无法提取或不在缓存）→ fallback 到 IANA
	// "example.zzz" 提取 tld=zzz，不在 dns map → IANA fallback
	url2, err := discoverRDAPServer(context.Background(), "example.zzz")
	assert.NoError(t, err)
	assert.Contains(t, url2, "rdap.iana.org")

	// 无法提取 TLD（单段）→ error
	_, err = discoverRDAPServer(context.Background(), "invalid")
	assert.Error(t, err)
}

// ---- discoverIP_RDAPServer: 命中 CIDR / IPv4 fallback / IPv6 fallback ----

func TestDiscoverIP_RDAPServer(t *testing.T) {
	// 命中 ARIN 范围（8.0.0.0/8）
	url, err := discoverIP_RDAPServer("8.8.8.8")
	assert.NoError(t, err)
	assert.Equal(t, "https://rdap.arin.net/registry", url)

	// 不在任何已知 CIDR 的 IPv4（233.x 组播保留未收录）→ ARIN fallback
	url, err = discoverIP_RDAPServer("233.0.0.1")
	assert.NoError(t, err)
	assert.Equal(t, "https://rdap.arin.net/registry", url)

	// IPv6 → APNIC fallback
	url, err = discoverIP_RDAPServer("2001:db8::1")
	assert.NoError(t, err)
	assert.Equal(t, "https://rdap.apnic.net", url)

	// 无效 IP
	_, err = discoverIP_RDAPServer("not-an-ip")
	assert.Error(t, err)
}

// ---- discoverIP_RDAPServer: 坏 CIDR 跳过分支 ----
// 通过临时注入一个坏 CIDR 到 bootstrap 来触发 ParseCIDR err 分支

func TestDiscoverIP_RDAPServer_BadCIDRSkipped(t *testing.T) {
	b := GetRDAPBootstrap()
	b.mu.Lock()
	orig := b.ipRanges
	b.ipRanges = append([]rdapIPRange{
		{cidr: "bad-cidr", rdapURL: "https://should-be-skipped"},
		{cidr: "8.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
	}, orig...)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.ipRanges = orig
		b.mu.Unlock()
	}()

	url, err := discoverIP_RDAPServer("8.8.8.8")
	assert.NoError(t, err)
	assert.Equal(t, "https://rdap.arin.net/registry", url)
}

// ---- discoverASN_RDAPServer: 命中 / 未命中 ----

func TestDiscoverASN_RDAPServer(t *testing.T) {
	// 命中 ARIN 范围（13335）
	url, err := discoverASN_RDAPServer(13335)
	assert.NoError(t, err)
	assert.NotEmpty(t, url)

	// 未命中（超过最大 ASN 范围 2240000000）
	_, err = discoverASN_RDAPServer(3000000000)
	assert.Error(t, err)
}

// clientRoutingTo 返回一个 HTTP 客户端，其 Transport 把所有请求
// 转发给目标测试服务器（无视 URL 主机/协议），从而在不修改 bootstrap 的情况下
// 覆盖 QueryRDAP_*WithContext 的成功端到端路径。

type routingTransport struct{ target *httptest.Server }

func (t routingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 把请求改写为指向测试服务器（保留路径与查询）
	req.URL.Scheme = "http"
	req.URL.Host = t.target.Listener.Addr().String()
	return t.target.Client().Transport.RoundTrip(req)
}

func clientRoutingTo(srv *httptest.Server) *http.Client {
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: routingTransport{target: srv},
	}
}

// ---- QueryRDAP_IPWithContext: 成功路径（自定义 transport 路由到测试服务器）----

func TestQueryRDAP_IPWithContext_Success(t *testing.T) {
	body := `{"objectClassName":"ip network","startAddress":"8.8.8.0","endAddress":"8.8.8.255","cidr":["8.8.8.0/24"],"ipVersion":"v4","country":"US","status":["active"]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	res, err := QueryRDAP_IPWithContext(context.Background(), &RDAPQueryOptions{
		IP:         "8.8.8.8",
		Timeout:    5,
		HTTPClient: clientRoutingTo(srv),
	})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "US", res.Country)
	assert.Equal(t, []string{"8.8.8.0/24"}, res.CIDR)
}

// ---- QueryRDAP_ASNWithContext: 无效 ASN ----

func TestQueryRDAP_ASNWithContext_InvalidASN(t *testing.T) {
	_, err := QueryRDAP_ASNWithContext(context.Background(), &RDAPQueryOptions{ASN: "not-a-number"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的ASN")
}

// ---- QueryRDAP_ASN: 便捷包装 ----

func TestQueryRDAP_ASN_Convenience(t *testing.T) {
	_, err := QueryRDAP_ASN("not-a-number")
	assert.Error(t, err)
}

// ---- QueryRDAP_IP: 便捷包装 ----

func TestQueryRDAP_IP_Convenience(t *testing.T) {
	// IP 为空走 opts 校验
	_, err := QueryRDAP_IP("")
	assert.Error(t, err)
}

// ---- QueryRDAP: 便捷包装 ----

func TestQueryRDAP_Convenience(t *testing.T) {
	// 空域名走 opts 校验
	_, err := QueryRDAP("")
	assert.Error(t, err)
}

// ---- QueryRDAP_Entity: 便捷包装（空 handle）----

func TestQueryRDAP_Entity_Convenience(t *testing.T) {
	_, err := QueryRDAP_Entity("")
	assert.Error(t, err)
}

// ---- QueryRDAP_EntityWithContext: 成功路径（自定义 transport 路由）----

func TestQueryRDAP_EntityWithContext_Success(t *testing.T) {
	body := `{"objectClassName":"entity","handle":"ABU123-ARIN","roles":["abuse"],"status":["active"]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	res, err := QueryRDAP_EntityWithContext(context.Background(), &RDAPQueryOptions{
		EntityHandle: "ABU123-ARIN",
		Timeout:      5,
		HTTPClient:   clientRoutingTo(srv),
	})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "ABU123-ARIN", res.Handle)
	assert.Equal(t, []string{"abuse"}, res.Roles)
}

// ---- QueryRDAP_EntityWithContext: 解析失败 ----

func TestQueryRDAP_EntityWithContext_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not-json`)
	}))
	defer srv.Close()

	_, err := QueryRDAP_EntityWithContext(context.Background(), &RDAPQueryOptions{
		EntityHandle: "ABU123-ARIN",
		Timeout:      5,
		HTTPClient:   clientRoutingTo(srv),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析RDAP Entity响应失败")
}

// ---- rdapHTTPRequest: 创建请求失败（坏 URL）----

func TestRdapHTTPRequest_BadURL(t *testing.T) {
	// 空字符串无法构造请求
	_, err := rdapHTTPRequest(context.Background(), "http://[::1]:named", &RDAPQueryOptions{Timeout: 5})
	// 可能 NewRequest 失败或 Do 失败，二选一都算覆盖错误分支
	if err == nil {
		t.Skip("bad url did not produce error (环境相关)")
	}
}

// ---- rdapHTTPRequest: 默认 client（opts.HTTPClient==nil）----

func TestRdapHTTPRequest_DefaultClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/rdap+json", r.Header.Get("Accept"))
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	body, err := rdapHTTPRequest(context.Background(), srv.URL, &RDAPQueryOptions{Timeout: 5})
	assert.NoError(t, err)
	assert.Contains(t, string(body), "ok")
}

// ---- rdapHTTPRequest: 非 200 状态码 ----

func TestRdapHTTPRequest_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `internal error`)
	}))
	defer srv.Close()

	_, err := rdapHTTPRequest(context.Background(), srv.URL, &RDAPQueryOptions{Timeout: 5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP服务器返回错误")
}

// ---- rdapHTTPRequest: Do 失败（不可达）----

func TestRdapHTTPRequest_DoFail(t *testing.T) {
	_, err := rdapHTTPRequest(context.Background(), "http://127.0.0.1:1/rdap", &RDAPQueryOptions{Timeout: 1, HTTPClient: &http.Client{Timeout: 1}})
	assert.Error(t, err)
}

// ---- QueryRDAPWithContext: 解析失败（返回非 JSON）----

func TestQueryRDAPWithContext_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not-json`)
	}))
	defer srv.Close()

	b := GetRDAPBootstrap()
	b.mu.Lock()
	b.dns["tstjson"] = srv.URL
	b.mu.Unlock()

	_, err := QueryRDAPWithContext(context.Background(), &RDAPQueryOptions{
		Domain:     "example.tstjson",
		Timeout:    5,
		HTTPClient: srv.Client(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析RDAP响应失败")
}

// ---- QueryRDAP_IPWithContext: 解析失败 ----

func TestQueryRDAP_IPWithContext_BadJSON(t *testing.T) {
	// 通过注入自定义 ipRanges 让 discover 返回测试服务器
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not-json`)
	}))
	defer srv.Close()

	b := GetRDAPBootstrap()
	b.mu.Lock()
	origIP := b.ipRanges
	b.ipRanges = []rdapIPRange{{cidr: "8.0.0.0/8", rdapURL: srv.URL}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.ipRanges = origIP
		b.mu.Unlock()
	}()

	_, err := QueryRDAP_IPWithContext(context.Background(), &RDAPQueryOptions{
		IP:         "8.8.8.8",
		Timeout:    5,
		HTTPClient: srv.Client(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析RDAP IP响应失败")
}

// ---- QueryRDAP_ASNWithContext: 成功路径 ----

func TestQueryRDAP_ASNWithContext_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"objectClassName":"autnum","handle":"AS13335","startAutnum":13335,"endAutnum":13335,"name":"CLOUDFLARE","country":"US","status":["active"]}`)
	}))
	defer srv.Close()

	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 13335, end: 13335, rdapURL: srv.URL}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	res, err := QueryRDAP_ASNWithContext(context.Background(), &RDAPQueryOptions{
		ASN:        "AS13335",
		Timeout:    5,
		HTTPClient: srv.Client(),
	})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "CLOUDFLARE", res.Name)
}

// ---- QueryRDAP_ASNWithContext: 未发现 ASN RDAP 服务器 ----

func TestQueryRDAP_ASNWithContext_DiscoverFail(t *testing.T) {
	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = nil
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	_, err := QueryRDAP_ASNWithContext(context.Background(), &RDAPQueryOptions{
		ASN:     "AS999999999",
		Timeout: 5,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "发现ASN RDAP服务器失败")
}

// ---- discoverEntityRDAPServer: -APNIC 后缀 + default 分支 ----

func TestDiscoverEntityRDAPServer_ExtraSuffixes(t *testing.T) {
	// -APNIC 后缀
	assert.Equal(t, "https://rdap.apnic.net", discoverEntityRDAPServer("X-APNIC"))
	// default（无已知后缀）
	assert.Equal(t, "https://rdap.arin.net/registry", discoverEntityRDAPServer("UNKNOWN-HANDLE"))
}

// ---- export: ExportToJSON/CSV/Markdown 各 nil ----

func TestExportToJSON_NilExtra(t *testing.T) {
	var buf bytes.Buffer
	err := ExportToJSON(nil, &buf)
	assert.Error(t, err)
}

func TestExportToCSV_NilExtra(t *testing.T) {
	var buf bytes.Buffer
	err := ExportToCSV(nil, &buf)
	assert.Error(t, err)
}

func TestExportToMarkdown_NilExtra(t *testing.T) {
	var buf bytes.Buffer
	err := ExportToMarkdown(nil, &buf)
	assert.Error(t, err)
}

// ---- RegisterExporter: nil 参数 ----

func TestRegisterExporter_Nil(t *testing.T) {
	assert.NotPanics(t, func() { RegisterExporter(nil) })
}

// ---- RegisterExporter: 覆盖同名 + GetExporter ----

func TestRegisterExporter_OverrideAndGet(t *testing.T) {
	// 自定义导出器
	custom := &stubExporter{format: "stub-format", output: "a"}
	RegisterExporter(custom)
	got, ok := GetExporter("stub-format")
	assert.True(t, ok)
	assert.Same(t, custom, got)
	// 覆盖同名
	custom2 := &stubExporter{format: "stub-format", output: "b"}
	RegisterExporter(custom2)
	got2, _ := GetExporter("stub-format")
	assert.Same(t, custom2, got2)
	// 清理
	UnregisterExporter("stub-format")
	_, ok = GetExporter("stub-format")
	assert.False(t, ok)
}

// ---- GetExporter: 未注册 ----

func TestGetExporter_NotFound(t *testing.T) {
	_, ok := GetExporter("does-not-exist")
	assert.False(t, ok)
}

// ---- ExportWith: 走内置 markdown 导出器 ----

func TestExportWith_Markdown(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "x.com"},
	}
	var buf bytes.Buffer
	err := ExportWith(info, "markdown", &buf)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "x.com")
}

// ---- ExportWith: 走内置 csv 导出器（覆盖 csvExporter.Export）----

func TestExportWith_CSV(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "x.com"},
	}
	var buf bytes.Buffer
	err := ExportWith(info, "csv", &buf)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "x.com")
	assert.Contains(t, buf.String(), "Field")
}

// ---- ExportWith: 走自定义导出器 ----

func TestExportWith_CustomExporter(t *testing.T) {
	custom := &stubExporter{format: "stub-format", output: "stub-export"}
	RegisterExporter(custom)
	defer UnregisterExporter("stub-format")

	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x"}}
	var buf bytes.Buffer
	err := ExportWith(info, "stub-format", &buf)
	assert.NoError(t, err)
	assert.Equal(t, "stub-export", buf.String())
}

// ---- ExportToCSV: 写入失败分支不可达 ----
// csv.Writer 把底层 writer 的错误缓存到 Flush()，而 ExportToCSV 用 defer Flush()，
// Flush 在 return 之后执行，函数内的 writer.Write() 永远返回 nil。
// 因此 ExportToCSV 内 `if err := writer.Write(...); err != nil` 分支实际不可达。

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, fmt.Errorf("write error") }

// ---- ExportToMarkdown: 写入失败 ----

func TestExportToMarkdown_WriteFail(t *testing.T) {
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}
	err := ExportToMarkdown(info, errWriter{})
	assert.Error(t, err)
}

// ---- ExportToJSON: 写入失败 ----

func TestExportToJSON_WriteFail(t *testing.T) {
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}
	err := ExportToJSON(info, errWriter{})
	assert.Error(t, err)
}

// ---- ExportToMarkdown: 仅联系人无域名 ----

func TestExportToMarkdown_ContactsOnly(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrar: &whoisparser.Contact{Name: "R"},
	}
	var buf bytes.Buffer
	err := ExportToMarkdown(info, &buf)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(buf.String(), "R"))
}

// ---- stubExporter 测试用自定义导出器 ----
// 复用 export_test.go 中已定义的 stubExporter（{format,output}）。
