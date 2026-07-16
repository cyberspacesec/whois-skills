package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"
)

// routingTransport 把所有出站 HTTP 请求改写为指向目标 httptest.Server，
// 从而在不修改 RDAP bootstrap 的前提下拦截 RDAP 命令的默认 http.Client 请求。
//
// cmd_query.go 构造的 RDAPQueryOptions 不暴露 HTTPClient，rdapHTTPRequest
// 在 HTTPClient 为 nil 时新建 &http.Client{Timeout:...}，其 Transport==nil
// 时使用 http.DefaultTransport，故替换 DefaultTransport 即可全局拦截。
type routingTransport struct{ target *httptest.Server }

func (t routingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.target.Listener.Addr().String()
	return t.target.Client().Transport.RoundTrip(req)
}

// withRDAPHTTPServer 装配一个 httptest.Server，并把 http.DefaultTransport
// 替换为指向该 server 的 routingTransport，返回 server 与恢复函数。
// 调用方应在 defer 中先 Close server 再调用恢复函数。
func withRDAPHTTPServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	origTransport := http.DefaultTransport
	http.DefaultTransport = routingTransport{target: srv}
	return srv, func() { http.DefaultTransport = origTransport }
}

// resetGlobalFlags 把与查询命令相关的全局 flag 恢复为默认值，
// 避免不同用例间相互污染。返回恢复函数。
func resetGlobalFlags(t *testing.T) func() {
	t.Helper()
	oTimeout, oUseProxy, oOutput := flagTimeout, flagUseProxy, flagOutput
	flagTimeout, flagUseProxy, flagOutput = 10, false, "json"
	return func() {
		flagTimeout, flagUseProxy, flagOutput = oTimeout, oUseProxy, oOutput
	}
}

// execRoot 构造根命令、设置参数、执行并捕获 stdout，返回输出与 RunE 错误。
func execRoot(t *testing.T, args []string) (string, error) {
	t.Helper()
	var runErr error
	out := captureStdout(t, func() {
		root := newRootCmd()
		root.SetArgs(args)
		runErr = root.Execute()
	})
	return out, runErr
}

// ---- newWhoisCmd ----

func TestNewWhoisCmd_JSONSuccess(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, newStubProvider())

	out, err := execRoot(t, []string{"whois", "example.com"})
	assert.NoError(t, err)
	assert.Contains(t, out, "example.com")
	var m map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(out), &m))
}

func TestNewWhoisCmd_RawSuccess(t *testing.T) {
	defer resetGlobalFlags(t)()
	flagOutput = "raw"
	withStubProvider(t, &stubQueryProvider{raw: "RAW-WHOIS-TEXT"})

	out, err := execRoot(t, []string{"whois", "example.com", "--raw"})
	assert.NoError(t, err)
	assert.Contains(t, out, "RAW-WHOIS-TEXT")
}

func TestNewWhoisCmd_QueryError(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, &stubQueryProvider{queryErr: errors.New("network down")})

	_, err := execRoot(t, []string{"whois", "example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询失败")
	assert.Contains(t, err.Error(), "network down")
}

func TestNewWhoisCmd_RawQueryError(t *testing.T) {
	defer resetGlobalFlags(t)()
	flagOutput = "raw"
	withStubProvider(t, &stubQueryProvider{queryErr: errors.New("boom")})

	_, err := execRoot(t, []string{"whois", "example.com", "--raw"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询失败")
}

func TestNewWhoisCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, newStubProvider())

	_, err := execRoot(t, []string{"whois"}) // 缺参数
	assert.Error(t, err)
}

// ---- newIPCmd ----

func TestNewIPCmd_InvalidIP(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"ip", "999.999.999.999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IP 查询失败")
	assert.Contains(t, err.Error(), "无效的IP地址")
}

func TestNewIPCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"ip"}) // 缺参数
	assert.Error(t, err)
}

// ---- newASNCmd ----

func TestNewASNCmd_ParseError(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"asn", "AS-INVALID"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ASN 解析失败")
}

func TestNewASNCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"asn"})
	assert.Error(t, err)
}

func TestNewASNCmd_UnsupportedSource(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"asn", "13335", "--source", "weird"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ASN 查询失败")
}

// TestNewASNCmd_RDAPSuccess 通过 source=rdap 覆盖 ASN 查询成功分支。
// queryASNFromRDAP 仅走 RDAP HTTP，被 routingTransport 路由到本地 server；
// IncludeBGP=false 跳过 BGP、source=rdap 不触发 RADB 前缀补充，故纯本地可完成。
func TestNewASNCmd_RDAPSuccess(t *testing.T) {
	defer resetGlobalFlags(t)()
	body := `{"objectClassName":"autnum","asn":15169,"handle":"AS15169","name":"GOOGLE","country":"US","type":"active"}`
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.Write([]byte(body))
	})
	defer srv.Close()
	defer restore()

	out, err := execRoot(t, []string{"asn", "AS15169", "--source", "rdap", "--include-bgp=false"})
	assert.NoError(t, err)
	assert.Contains(t, out, "GOOGLE")
	assert.Contains(t, out, "15169")
}

// ---- newAvailabilityCmd ----

func TestNewAvailabilityCmd_Registered(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, newStubProvider())

	out, err := execRoot(t, []string{"availability", "example.com"})
	assert.NoError(t, err)
	assert.Contains(t, out, "registered")
	assert.Contains(t, out, "example.com")
}

func TestNewAvailabilityCmd_Available(t *testing.T) {
	defer resetGlobalFlags(t)()
	// Query 返回 parser ErrNotFoundDomain → ExecuteQuery 原样返回 → available
	withStubProvider(t, &stubQueryProvider{queryErr: whoisparser.ErrNotFoundDomain})

	out, err := execRoot(t, []string{"availability", "free-domain.test"})
	assert.NoError(t, err)
	assert.Contains(t, out, "available")
}

func TestNewAvailabilityCmd_Reserved(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, &stubQueryProvider{queryErr: whoisparser.ErrReservedDomain})

	out, err := execRoot(t, []string{"availability", "reserved.test"})
	assert.NoError(t, err)
	assert.Contains(t, out, "reserved")
}

func TestNewAvailabilityCmd_Premium(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, &stubQueryProvider{queryErr: whoisparser.ErrPremiumDomain})

	out, err := execRoot(t, []string{"availability", "premium.test"})
	assert.NoError(t, err)
	assert.Contains(t, out, "premium")
}

func TestNewAvailabilityCmd_Blocked(t *testing.T) {
	defer resetGlobalFlags(t)()
	withStubProvider(t, &stubQueryProvider{queryErr: whoisparser.ErrBlockedDomain})

	out, err := execRoot(t, []string{"availability", "blocked.test"})
	assert.NoError(t, err)
	assert.Contains(t, out, "blocked")
}

func TestNewAvailabilityCmd_RateLimited(t *testing.T) {
	defer resetGlobalFlags(t)()
	// ErrDomainLimitExceed → status=rate_limited，并返回 err（可用性命令走错误分支）
	withStubProvider(t, &stubQueryProvider{queryErr: whoisparser.ErrDomainLimitExceed})

	_, err := execRoot(t, []string{"availability", "limited.test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "可用性检测失败")
}

func TestNewAvailabilityCmd_GenericError(t *testing.T) {
	defer resetGlobalFlags(t)()
	// 非 parser 已知错误 → CheckError 包装成未知 WhoisError → 原样返回 → 命令错误分支
	withStubProvider(t, &stubQueryProvider{queryErr: errors.New("dns failure")})

	_, err := execRoot(t, []string{"availability", "some.test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "可用性检测失败")
	assert.Contains(t, err.Error(), "dns failure")
}

func TestNewAvailabilityCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"availability"})
	assert.Error(t, err)
}

// ---- newRDAPBootstrapCmd ----

func TestNewRDAPBootstrapCmd_TLD(t *testing.T) {
	defer resetGlobalFlags(t)()
	out, err := execRoot(t, []string{"rdap", "bootstrap", "--tld", "com"})
	assert.NoError(t, err)
	assert.Contains(t, out, "com")
	assert.Contains(t, out, "rdap.verisign.com")
}

func TestNewRDAPBootstrapCmd_ASN(t *testing.T) {
	defer resetGlobalFlags(t)()
	out, err := execRoot(t, []string{"rdap", "bootstrap", "--asn", "13335"})
	assert.NoError(t, err)
	assert.Contains(t, out, "13335")
}

func TestNewRDAPBootstrapCmd_TLDAndASN(t *testing.T) {
	defer resetGlobalFlags(t)()
	out, err := execRoot(t, []string{"rdap", "bootstrap", "--tld", "com", "--asn", "13335"})
	assert.NoError(t, err)
	assert.Contains(t, out, "com")
	assert.Contains(t, out, "13335")
}

func TestNewRDAPBootstrapCmd_EmptyArgs(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"rdap", "bootstrap"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--tld")
}

func TestNewRDAPBootstrapCmd_InvalidASN(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"rdap", "bootstrap", "--asn", "AS-XYZ"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效 ASN")
}

func TestNewRDAPBootstrapCmd_UnknownTLD(t *testing.T) {
	defer resetGlobalFlags(t)()
	// 未知 TLD：GetDNSServer 返回空串，但命令仍正常输出（server 为空）
	out, err := execRoot(t, []string{"rdap", "bootstrap", "--tld", "nonexistenttld"})
	assert.NoError(t, err)
	assert.Contains(t, out, "nonexistenttld")
}

// ---- newRDAPDomainCmd ----

func TestNewRDAPDomainCmd_Success(t *testing.T) {
	defer resetGlobalFlags(t)()
	body := `{"objectClassName":"domain","ldhName":"example.com","status":["active"],"events":[{"eventAction":"registration"}]}`
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.Write([]byte(body))
	})
	defer srv.Close()
	defer restore()

	out, err := execRoot(t, []string{"rdap", "domain", "example.com"})
	assert.NoError(t, err)
	assert.Contains(t, out, "example.com")
}

func TestNewRDAPDomainCmd_HTTPError(t *testing.T) {
	defer resetGlobalFlags(t)()
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	defer srv.Close()
	defer restore()

	_, err := execRoot(t, []string{"rdap", "domain", "example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP 域名查询失败")
}

func TestNewRDAPDomainCmd_BadJSON(t *testing.T) {
	defer resetGlobalFlags(t)()
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	})
	defer srv.Close()
	defer restore()

	_, err := execRoot(t, []string{"rdap", "domain", "example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP 域名查询失败")
}

func TestNewRDAPDomainCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"rdap", "domain"})
	assert.Error(t, err)
}

// ---- newRDAPIPCmd ----

func TestNewRDAPIPCmd_Success(t *testing.T) {
	defer resetGlobalFlags(t)()
	body := `{"objectClassName":"ip network","startAddress":"8.8.8.0","endAddress":"8.8.8.255","country":"US"}`
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.Write([]byte(body))
	})
	defer srv.Close()
	defer restore()

	out, err := execRoot(t, []string{"rdap", "ip", "8.8.8.8"})
	assert.NoError(t, err)
	assert.Contains(t, out, "8.8.8.0")
	assert.Contains(t, out, "US")
}

func TestNewRDAPIPCmd_HTTPError(t *testing.T) {
	defer resetGlobalFlags(t)()
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})
	defer srv.Close()
	defer restore()

	_, err := execRoot(t, []string{"rdap", "ip", "8.8.8.8"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP IP 查询失败")
}

func TestNewRDAPIPCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"rdap", "ip"})
	assert.Error(t, err)
}

// ---- newRDAPASNCmd ----

func TestNewRDAPASNCmd_Success(t *testing.T) {
	defer resetGlobalFlags(t)()
	body := `{"objectClassName":"autnum","asn":13335,"handle":"AS13335","name":"CLOUDFLARENET","country":"US"}`
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.Write([]byte(body))
	})
	defer srv.Close()
	defer restore()

	out, err := execRoot(t, []string{"rdap", "asn", "AS13335"})
	assert.NoError(t, err)
	assert.Contains(t, out, "13335")
	assert.Contains(t, out, "CLOUDFLARENET")
}

func TestNewRDAPASNCmd_InvalidASN(t *testing.T) {
	defer resetGlobalFlags(t)()
	// extractASNNumber 返回 0 → 纯本地错误，无需 HTTP
	_, err := execRoot(t, []string{"rdap", "asn", "not-a-number"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP ASN 查询失败")
	assert.Contains(t, err.Error(), "无效的ASN")
}

func TestNewRDAPASNCmd_HTTPError(t *testing.T) {
	defer resetGlobalFlags(t)()
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	})
	defer srv.Close()
	defer restore()

	_, err := execRoot(t, []string{"rdap", "asn", "AS13335"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP ASN 查询失败")
}

func TestNewRDAPASNCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"rdap", "asn"})
	assert.Error(t, err)
}

// ---- newRDAPEntityCmd ----

func TestNewRDAPEntityCmd_Success(t *testing.T) {
	defer resetGlobalFlags(t)()
	body := `{"objectClassName":"entity","handle":"TEST-ARIN","roles":["registrar"]}`
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.Write([]byte(body))
	})
	defer srv.Close()
	defer restore()

	out, err := execRoot(t, []string{"rdap", "entity", "TEST-ARIN"})
	assert.NoError(t, err)
	assert.Contains(t, out, "TEST-ARIN")
}

func TestNewRDAPEntityCmd_HTTPError(t *testing.T) {
	defer resetGlobalFlags(t)()
	srv, restore := withRDAPHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	defer srv.Close()
	defer restore()

	_, err := execRoot(t, []string{"rdap", "entity", "TEST-ARIN"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RDAP 实体查询失败")
}

func TestNewRDAPEntityCmd_ArgsValidation(t *testing.T) {
	defer resetGlobalFlags(t)()
	_, err := execRoot(t, []string{"rdap", "entity"})
	assert.Error(t, err)
}

// ---- newRDAPCmd 父命令结构 ----

func TestNewRDAPCmd_Subcommands(t *testing.T) {
	root := newRootCmd()
	for _, sub := range []string{"domain", "ip", "asn", "entity", "bootstrap"} {
		_, _, err := root.Find([]string{"rdap", sub})
		assert.NoErrorf(t, err, "rdap 应包含子命令 %s", sub)
	}
}

// ---- newIPCmd 真实网络成功路径 ----
// IP WHOIS 走 whois.iana.org:43 → RIR referral，无法用 provider stub。
// 仅当测试环境可访问 whois.iana.org:43 时覆盖 raw=true / raw=false 成功分支。
// 标记网络依赖，失败时跳过以免 flaky。

func TestNewIPCmd_JSONSuccess_Network(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	defer resetGlobalFlags(t)()
	out, err := execRoot(t, []string{"ip", "1.1.1.1"})
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	assert.Contains(t, out, "1.1.1.1")
}

func TestNewIPCmd_RawSuccess_Network(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	defer resetGlobalFlags(t)()
	flagOutput = "raw"
	out, err := execRoot(t, []string{"ip", "1.1.1.1", "--raw"})
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	// 原始文本至少包含 IP 或 refer 关键字
	assert.NotEmpty(t, out)
}
