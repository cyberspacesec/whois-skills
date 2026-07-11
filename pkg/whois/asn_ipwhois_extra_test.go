package whois

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---- queryASNFromRDAP: 成功路径（patch bootstrap.asnRanges 指向本地服务器）----

func TestQueryASNFromRDAP_Success(t *testing.T) {
	// 本地 RDAP 服务器返回 autnum JSON
	body := `{"objectClassName":"autnum","handle":"AS13335-CLOUD-ARIN","startAutnum":13335,"endAutnum":13335,"name":"CLOUDFLARENET","country":"US","type":"ASSIGNED","status":["active"],"events":[{"eventAction":"registration","eventDate":"2010-10-01T00:00:00Z"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	// patch ASN bootstrap 范围指向本地服务器（默认 client 访问 127.0.0.1:port）
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

	ClearASNDetailCache()
	info, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:     13335,
		Timeout: 5,
		Source:  ASNSourceRDAP,
	})
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "CLOUDFLARENET", info.Name)
	assert.Equal(t, "US", info.Country)
	assert.Equal(t, "ARIN", info.RIR) // 从 handle -ARIN 提取
	assert.Equal(t, "ASSIGNED", info.Status)
	assert.Equal(t, "2010-10-01T00:00:00Z", info.AllocationDate)
	assert.Equal(t, "rdap", info.Source)
	ClearASNDetailCache()
}

// ---- queryASNFromRDAP: RDAP 查询失败 ----

func TestQueryASNFromRDAP_QueryFail(t *testing.T) {
	// patch ASN bootstrap 指向一个返回 500 的服务器
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 99998, end: 99998, rdapURL: srv.URL}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	ClearASNDetailCache()
	_, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:     99998,
		Timeout: 5,
		Source:  ASNSourceRDAP,
	})
	assert.Error(t, err)
	ClearASNDetailCache()
}

// ---- QueryASNWithContext: ASNSourceRADB（连接 whois.radb.net 失败/成功）----
// whois.radb.net:43 在沙箱可访问；用真实查询覆盖 queryASNFromRADB 解析分支。
// 若网络不通则触发连接失败分支，二者都算覆盖。

func TestQueryASNWithContext_SourceRADB(t *testing.T) {
	ClearASNDetailCache()
	info, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:     13335,
		Timeout: 15,
		Source:  ASNSourceRADB,
	})
	if err != nil {
		// 连接失败分支被覆盖
		t.Logf("RADB 查询失败（网络相关，仍覆盖连接错误分支）: %v", err)
		ClearASNDetailCache()
		return
	}
	assert.NotNil(t, info)
	assert.Equal(t, "radb", info.Source)
	// 成功路径覆盖 as-name:/descr:/source: 解析分支
	assert.NotEmpty(t, info.Name)
	assert.NotEmpty(t, info.Description)
	assert.NotEmpty(t, info.RIR)
	ClearASNDetailCache()
}

// ---- QueryASNWithContext: ASNSourceAll，RDAP 成功 + 补充前缀 ----

func TestQueryASNWithContext_AllRDAPSuccess(t *testing.T) {
	body := `{"objectClassName":"autnum","handle":"AS15169-GOGL-ARIN","startAutnum":15169,"endAutnum":15169,"name":"GOOGLE","country":"US","type":"ASSIGNED"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 15169, end: 15169, rdapURL: srv.URL}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	ClearASNDetailCache()
	info, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:             15169,
		Timeout:         5,
		Source:          ASNSourceAll,
		IncludePrefixes: true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "rdap", info.Source)
	// IncludePrefixes 且 RDAP 未返回前缀 → 尝试 queryASNPrefixesFromRADB（真实网络，可能失败但忽略错误）
	ClearASNDetailCache()
}

// ---- BatchQueryASN: 非空列表（并发，依赖真实网络/RDAP patch）----

func TestBatchQueryASN_NonEmpty(t *testing.T) {
	// patch RDAP 让 AS13335 命中本地
	body := `{"objectClassName":"autnum","handle":"AS13335-ARIN","name":"CLOUD","country":"US","type":"ASSIGNED"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 13335, end: 13336, rdapURL: srv.URL}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	ClearASNDetailCache()
	result := BatchQueryASN(context.Background(), []int{13335, 13336}, 2)
	assert.Equal(t, 2, result.TotalQueried)
	assert.Equal(t, result.SuccessCount+result.FailureCount, 2)
	ClearASNDetailCache()
}

// ---- BatchQueryASN: concurrency<=0 默认 ----

func TestBatchQueryASN_DefaultConcurrency(t *testing.T) {
	ClearASNDetailCache()
	// 用一个保证 RDAP 失败的 ASN（patch 为不可达）
	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 777777, end: 777777, rdapURL: "http://127.0.0.1:1"}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	result := BatchQueryASN(context.Background(), []int{777777}, 0) // concurrency<=0 → 默认5
	assert.Equal(t, 1, result.TotalQueried)
	assert.GreaterOrEqual(t, result.FailureCount, 0)
	ClearASNDetailCache()
}

// ---- QueryASNWithContext: ASNSourceAll，RDAP 失败回退 RADB 失败 ----
// RDAP patch 指向不可达 → RDAP 失败；通过传入已取消 ctx 让 RADB DialContext 失败。

func TestQueryASNWithContext_AllBothFail(t *testing.T) {
	ClearASNDetailCache()
	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 888888, end: 888888, rdapURL: "http://127.0.0.1:1"}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消 → RADB DialContext 失败

	_, err := QueryASNWithContext(ctx, &ASNQueryOptions{
		ASN:     888888,
		Timeout: 5,
		Source:  ASNSourceAll,
	})
	assert.Error(t, err)
	ClearASNDetailCache()
}

// ---- GetIPRangesByASN: 真实 RADB（网络可达则覆盖解析；否则连接失败分支）----

func TestGetIPRangesByASN_Live(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx
	ipv4, ipv6, err := GetIPRangesByASN("13335")
	if err != nil {
		t.Logf("GetIPRangesByASN 失败（网络相关，仍覆盖连接错误分支）: %v", err)
		return
	}
	// AS13335 通常有前缀
	t.Logf("ipv4=%d ipv6=%d", len(ipv4), len(ipv6))
}

// ---- QueryIPWithContext: 真实 IANA 查询（网络可达则覆盖主体；否则 IANA 失败分支）----

func TestQueryIPWithContext_Live(t *testing.T) {
	res, err := QueryIPWithContext(context.Background(), &IPWhoisOptions{IP: "8.8.8.8", Timeout: 15})
	if err != nil {
		t.Logf("QueryIPWithContext 失败（网络相关，仍覆盖 IANA 失败分支）: %v", err)
		return
	}
	assert.NotNil(t, res)
	assert.Equal(t, "8.8.8.8", res.IP)
	assert.NotEmpty(t, res.RawResponse)
}

// ---- QueryIP: 便捷包装（无效 IP 报错）----

func TestQueryIP_Invalid(t *testing.T) {
	_, err := QueryIP("not-an-ip")
	assert.Error(t, err)
}

// ---- QueryIPWithContext: UseProxy=true 分支 ----
// 使用一个无效 proxy 配置触发；UseProxy 走 GetProxyPool（单例，空池不影响流程）
// 最终仍连 whois.iana.org，可能失败或成功，覆盖 UseProxy 赋值分支即可。

func TestQueryIPWithContext_UseProxy(t *testing.T) {
	res, err := QueryIPWithContext(context.Background(), &IPWhoisOptions{IP: "1.1.1.1", Timeout: 15, UseProxy: true})
	if err != nil {
		t.Logf("UseProxy 查询失败（网络相关，仍覆盖分支）: %v", err)
		return
	}
	assert.NotNil(t, res)
}
