package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RDAPResult RDAP查询结果 (RFC 9083)
type RDAPResult struct {
	// 原始JSON响应
	RawJSON []byte `json:"-"`

	// 对象类名
	ObjectClassName string `json:"objectClassName"`

	// LDH名称
	LDHName string `json:"ldhName"`

	// Unicode名称
	UnicodeName string `json:"unicodeName"`

	// 状态
	Status []string `json:"status"`

	// 名称服务器
	Nameservers []RDAPNameserver `json:"nameservers,omitempty"`

	// 事件
	Events []RDAPEvent `json:"events,omitempty"`

	// 实体（联系人）
	Entities []RDAPEntity `json:"entities,omitempty"`

	// 链接
	Links []RDAPLink `json:"links,omitempty"`

	// 备注
	Remarks []RDAPRemark `json:"remarks,omitempty"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`

	// 查询服务器
	Server string `json:"server"`
}

// RDAPEvent RDAP事件
type RDAPEvent struct {
	EventAction string `json:"eventAction"`
	EventDate   string `json:"eventDate"`
	EventActor  string `json:"eventActor,omitempty"`
}

// RDAPEntity RDAP实体
type RDAPEntity struct {
	Roles      []string       `json:"roles"`
	VCardArray []interface{}  `json:"vcardArray,omitempty"`
	PublicIDs  []RDAPPublicID `json:"publicIds,omitempty"`
}

// RDAPNameserver RDAP名称服务器
type RDAPNameserver struct {
	LDHName     string       `json:"ldhName"`
	IPAddresses *RDAPIPAddrs `json:"ipAddresses,omitempty"`
}

// RDAPLink RDAP链接
type RDAPLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
	Type string `json:"type,omitempty"`
}

// RDAPRemark RDAP备注
type RDAPRemark struct {
	Title       string   `json:"title,omitempty"`
	Description []string `json:"description"`
}

// RDAPPublicID RDAP公共ID
type RDAPPublicID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// RDAPIPAddrs RDAP IP地址
type RDAPIPAddrs struct {
	V4 []string `json:"v4,omitempty"`
	V6 []string `json:"v6,omitempty"`
}

// RDAPQueryOptions RDAP查询选项
type RDAPQueryOptions struct {
	// 域名
	Domain string `json:"domain"`

	// IP地址 (用于IP查询)
	IP string `json:"ip,omitempty"`

	// AS号 (用于ASN查询, 如 13335)
	ASN string `json:"asn,omitempty"`

	// 实体Handle (用于Entity查询)
	EntityHandle string `json:"entity_handle,omitempty"`

	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`

	// 是否使用自定义HTTP客户端
	HTTPClient *http.Client `json:"-"`
}

// ==============================
// RDAP IP查询结果
// ==============================

// RDAPIPResult RDAP IP查询结果
type RDAPIPResult struct {
	// 原始JSON响应
	RawJSON []byte `json:"-"`

	// 对象类名
	ObjectClassName string `json:"objectClassName"`

	// 起始地址
	StartAddress string `json:"startAddress"`

	// 结束地址
	EndAddress string `json:"endAddress"`

	// CIDR表示
	CIDR []string `json:"cidr"`

	// IP版本 (v4/v6)
	IPVersion string `json:"ipVersion"`

	// 类型
	Type string `json:"type"`

	// 名称
	Name string `json:"name"`

	// 国家
	Country string `json:"country"`

	// 父网络
	ParentHandle string `json:"parentHandle,omitempty"`

	// 事件
	Events []RDAPEvent `json:"events,omitempty"`

	// 实体
	Entities []RDAPEntity `json:"entities,omitempty"`

	// 链接
	Links []RDAPLink `json:"links,omitempty"`

	// 状态
	Status []string `json:"status"`

	// 备注
	Remarks []RDAPRemark `json:"remarks,omitempty"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`

	// 查询服务器
	Server string `json:"server"`
}

// ==============================
// RDAP ASN查询结果
// ==============================

// RDAPASNResult RDAP ASN查询结果
type RDAPASNResult struct {
	// 原始JSON响应
	RawJSON []byte `json:"-"`

	// 对象类名
	ObjectClassName string `json:"objectClassName"`

	// AS号
	ASN int `json:"asn"`

	// Handle
	Handle string `json:"handle"`

	// 名称
	Name string `json:"name"`

	// 国家
	Country string `json:"country"`

	// 类型
	Type string `json:"type"`

	// 起始AS号
	StartAutnum int `json:"startAutnum,omitempty"`

	// 结束AS号
	EndAutnum int `json:"endAutnum,omitempty"`

	// 事件
	Events []RDAPEvent `json:"events,omitempty"`

	// 实体
	Entities []RDAPEntity `json:"entities,omitempty"`

	// 链接
	Links []RDAPLink `json:"links,omitempty"`

	// 状态
	Status []string `json:"status"`

	// 备注
	Remarks []RDAPRemark `json:"remarks,omitempty"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`

	// 查询服务器
	Server string `json:"server"`
}

// ==============================
// RDAP Entity查询结果
// ==============================

// RDAPEntityResult RDAP Entity查询结果
type RDAPEntityResult struct {
	// 原始JSON响应
	RawJSON []byte `json:"-"`

	// 对象类名
	ObjectClassName string `json:"objectClassName"`

	// Handle
	Handle string `json:"handle"`

	// vCard数组
	VCardArray []interface{} `json:"vcardArray,omitempty"`

	// 角色
	Roles []string `json:"roles"`

	// 公共ID
	PublicIDs []RDAPPublicID `json:"publicIds,omitempty"`

	// 事件
	Events []RDAPEvent `json:"events,omitempty"`

	// 链接
	Links []RDAPLink `json:"links,omitempty"`

	// 状态
	Status []string `json:"status"`

	// 备注
	Remarks []RDAPRemark `json:"remarks,omitempty"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`

	// 查询服务器
	Server string `json:"server"`
}

// ==============================
// Bootstrap缓存
// ==============================

// RDAPBootstrap RDAP bootstrap数据缓存
type RDAPBootstrap struct {
	mu sync.RWMutex

	// DNS bootstrap数据
	dns map[string]string

	// IP bootstrap数据 (CIDR -> RDAP URL)
	ipRanges []rdapIPRange

	// ASN bootstrap数据 (ASN range -> RDAP URL)
	asnRanges []rdapASNRange

	// 最后更新时间
	lastUpdated time.Time

	// 是否已加载
	loaded bool
}

type rdapIPRange struct {
	cidr     string
	rdapURL  string
}

type rdapASNRange struct {
	start    int
	end      int
	rdapURL  string
}

var (
	defaultBootstrap *RDAPBootstrap
	bootstrapOnce    sync.Once
)

// GetRDAPBootstrap 获取RDAP bootstrap实例
func GetRDAPBootstrap() *RDAPBootstrap {
	bootstrapOnce.Do(func() {
		defaultBootstrap = &RDAPBootstrap{
			dns: make(map[string]string),
		}
		defaultBootstrap.loadDefaults()
	})
	return defaultBootstrap
}

// loadDefaults 加载默认的RDAP服务器映射
func (b *RDAPBootstrap) loadDefaults() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// DNS (TLD -> RDAP URL)
	b.dns = map[string]string{
		"com":     "https://rdap.verisign.com/com/v1",
		"net":     "https://rdap.verisign.com/net/v1",
		"org":     "https://rdap.publicinterestregistry.org/rdap",
		"info":    "https://rdap.identitydigital.com/rdap",
		"biz":     "https://rdap.nic.biz",
		"mobi":    "https://rdap.identitydigital.com/rdap",
		"pro":     "https://rdap.identitydigital.com/rdap",
		"name":    "https://rdap.verisign.com/name/v1",
		"edu":     "https://rdap.educause.edu",
		"gov":     "https://rdap.dotgov.gov",
		"io":      "https://rdap.nic.io",
		"co":      "https://rdap.nic.co",
		"ai":      "https://rdap.nic.ai",
		"app":     "https://rdap.nic.google",
		"dev":     "https://rdap.nic.google",
		"xyz":     "https://rdap.centralnic.com/xyz",
		"site":    "https://rdap.centralnic.com/site",
		"online":  "https://rdap.centralnic.com/online",
		"shop":    "https://rdap.centralnic.com/shop",
		"store":   "https://rdap.centralnic.com/store",
		"blog":    "https://rdap.centralnic.com/blog",
		"top":     "https://rdap.centralnic.com/top",
		"club":    "https://rdap.nic.club",
		"vip":     "https://rdap.nic.vip",
		"cn":      "https://rdap.cnnic.cn",
		"uk":      "https://rdap.nic.uk",
		"de":      "https://rdap.denic.de",
		"fr":      "https://rdap.nic.fr",
		"eu":      "https://rdap.eu.org",
		"au":      "https://rdap.auda.org.au",
		"jp":      "https://rdap.jprs.jp",
		"br":      "https://rdap.registro.br",
		"ru":      "https://rdap.tcinet.ru",
		"cc":      "https://rdap.verisign.com/cc/v1",
		"tv":      "https://rdap.verisign.com/tv/v1",
	}

	// IP RIR CIDR blocks -> RDAP URL
	// 基于IANA分配的IPv4地址空间主要RIR块
	b.ipRanges = []rdapIPRange{
		// ARIN - 北美地区
		{cidr: "3.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "4.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "6.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "7.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "8.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "9.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "11.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "12.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "13.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "15.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "16.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "17.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "18.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "19.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "20.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "21.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "22.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "23.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "24.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "25.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "26.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "32.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "33.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "34.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "35.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "38.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "40.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "44.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "45.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "47.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "48.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "50.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "52.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "54.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "55.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "56.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "57.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "63.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "64.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "65.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "66.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "67.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "68.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "69.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "70.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "71.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "72.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "73.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "74.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "75.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "76.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "96.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "97.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "98.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "99.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "100.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "104.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "107.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "108.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "128.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "129.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "130.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "131.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "132.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "133.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "134.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "135.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "136.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "137.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "138.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "139.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "140.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "141.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "142.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "143.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "144.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "145.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "146.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "147.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "148.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "149.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "150.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "151.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "152.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "153.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "155.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "156.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "157.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "158.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "159.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "160.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "161.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "162.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "163.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "164.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "165.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "166.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "167.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "168.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "169.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "170.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "171.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "172.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "173.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "174.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "192.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "198.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "199.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "204.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "205.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "206.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "207.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "208.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "209.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},
		{cidr: "216.0.0.0/8", rdapURL: "https://rdap.arin.net/registry"},

		// RIPE NCC - 欧洲/中东/中亚
		{cidr: "2.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "5.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "31.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "37.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "46.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "62.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "77.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "78.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "79.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "80.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "81.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "82.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "83.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "84.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "85.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "86.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "87.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "88.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "89.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "90.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "91.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "92.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "93.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "94.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "95.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "109.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "176.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "178.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "185.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "188.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "212.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "213.0.0.0/8", rdapURL: "https://rdap.ripe.net"},
		{cidr: "217.0.0.0/8", rdapURL: "https://rdap.ripe.net"},

		// APNIC - 亚太地区
		{cidr: "1.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "14.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "27.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "36.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "39.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "42.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "49.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "58.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "59.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "60.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "61.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "101.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "103.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "106.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "110.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "111.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "112.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "113.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "114.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "115.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "116.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "117.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "118.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "119.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "120.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "121.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "122.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "123.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "124.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "125.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "126.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "175.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "180.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "182.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "183.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "202.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "203.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "210.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "211.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "218.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "219.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "220.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "221.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "222.0.0.0/8", rdapURL: "https://rdap.apnic.net"},
		{cidr: "223.0.0.0/8", rdapURL: "https://rdap.apnic.net"},

		// LACNIC - 拉丁美洲和加勒比地区
		{cidr: "177.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "179.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "181.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "186.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "187.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "189.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "190.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "191.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "200.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},
		{cidr: "201.0.0.0/8", rdapURL: "https://rdap.lacnic.net/rdap"},

		// AFRINIC - 非洲地区
		{cidr: "41.0.0.0/8", rdapURL: "https://rdap.afrinic.net/rdap"},
		{cidr: "102.0.0.0/8", rdapURL: "https://rdap.afrinic.net/rdap"},
		{cidr: "105.0.0.0/8", rdapURL: "https://rdap.afrinic.net/rdap"},
		{cidr: "154.0.0.0/8", rdapURL: "https://rdap.afrinic.net/rdap"},
		{cidr: "196.0.0.0/8", rdapURL: "https://rdap.afrinic.net/rdap"},
		{cidr: "197.0.0.0/8", rdapURL: "https://rdap.afrinic.net/rdap"},
	}

	// ASN Ranges
	b.asnRanges = []rdapASNRange{
		{start: 1, end: 23455, rdapURL: "https://rdap.arin.net/registry"},
		{start: 23456, end: 25123, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 25124, end: 42949, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 42950, end: 43251, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 43252, end: 63487, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 63488, end: 65535, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 65536, end: 131071, rdapURL: "https://rdap.apnic.net"},
		{start: 131072, end: 196607, rdapURL: "https://rdap.ripe.net"},
		{start: 196608, end: 206495, rdapURL: "https://rdap.arin.net/registry"},
		{start: 206496, end: 210047, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 210048, end: 212991, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 212992, end: 262143, rdapURL: "https://rdap.ripe.net"},
		{start: 262144, end: 327679, rdapURL: "https://rdap.arin.net/registry"},
		{start: 327680, end: 328191, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 328192, end: 393215, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 393216, end: 393983, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 393984, end: 524287, rdapURL: "https://rdap.ripe.net"},
		{start: 524288, end: 524543, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 524544, end: 525311, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 525312, end: 559007, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 559008, end: 559231, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 559232, end: 655343, rdapURL: "https://rdap.arin.net/registry"},
		{start: 655344, end: 655359, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 655360, end: 656127, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 656128, end: 786431, rdapURL: "https://rdap.apnic.net"},
		{start: 786432, end: 787967, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 787968, end: 1048575, rdapURL: "https://rdap.ripe.net"},
		{start: 1048576, end: 1114111, rdapURL: "https://rdap.arin.net/registry"},
		{start: 1114112, end: 1114623, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 1114624, end: 2097151, rdapURL: "https://rdap.apnic.net"},
		{start: 2097152, end: 2147745583, rdapURL: "https://rdap.ripe.net"},
		{start: 2147745584, end: 2147745855, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2147745856, end: 2147760383, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2147760384, end: 2148270079, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2148270080, end: 2148270207, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2148270208, end: 2148270303, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2148270304, end: 2151677951, rdapURL: "https://rdap.arin.net/registry"},
		{start: 2151677952, end: 2151678015, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2151678016, end: 2151682047, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2151682048, end: 2152103167, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2152103168, end: 2152113919, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2152113920, end: 2153775103, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2153775104, end: 2153783295, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2153783296, end: 2155804671, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2155804672, end: 2155812351, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2155812352, end: 2156218367, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156218368, end: 2156226047, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156226048, end: 2156249087, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156249088, end: 2156250367, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156250368, end: 2156266495, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156266496, end: 2156266751, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156266752, end: 2156274687, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156274688, end: 2156282879, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156282880, end: 2156315647, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156315648, end: 2156335103, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156335104, end: 2156335359, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156335360, end: 2156445695, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156445696, end: 2156453887, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156453888, end: 2156519423, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156519424, end: 2156527615, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156527616, end: 2156543999, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156544000, end: 2156548095, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156548096, end: 2156570623, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156570624, end: 2156574719, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156574720, end: 2156580863, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156580864, end: 2156581119, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156581120, end: 2156625919, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156625920, end: 2156630783, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156630784, end: 2156644351, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156644352, end: 2156645375, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156645376, end: 2156732415, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156732416, end: 2156737535, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156737536, end: 2156797951, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2156797952, end: 2156803071, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2156803072, end: 2164260863, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2164260864, end: 2164261119, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2164261120, end: 2173347839, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2173347840, end: 2173348863, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2173348864, end: 2179194879, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2179194880, end: 2179248127, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2179248128, end: 2181033759, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2181033760, end: 2181041151, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2181041152, end: 2193571839, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2193571840, end: 2193575935, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2193575936, end: 2197833727, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2197833728, end: 2197835775, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2197835776, end: 2201760767, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2201760768, end: 2201778175, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2201778176, end: 2205536255, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2205536256, end: 2205544447, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2205544448, end: 2206750719, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2206750720, end: 2206758911, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2206758912, end: 2208744447, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2208744448, end: 2208746495, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2208746496, end: 2209976319, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2209976320, end: 2209978367, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2209978368, end: 2210058239, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2210058240, end: 2210072063, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2210072064, end: 2212381695, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2212381696, end: 2212384767, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2212384768, end: 2212671487, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2212671488, end: 2212675583, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2212675584, end: 2213015551, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2213015552, end: 2213019647, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2213019648, end: 2213020671, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2213020672, end: 2226580479, rdapURL: "https://rdap.apnic.net"},
		{start: 2226580480, end: 2226604031, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2226604032, end: 2238969855, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2238969856, end: 2238972159, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2238972160, end: 2239178751, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2239178752, end: 2239180799, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2239180800, end: 2239455231, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2239455232, end: 2239456255, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2239456256, end: 2239621119, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2239621120, end: 2239622143, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2239622144, end: 2239723519, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2239723520, end: 2239725567, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2239725568, end: 2239755775, rdapURL: "https://rdap.afrinic.net/rdap"},
		{start: 2239755776, end: 2239756287, rdapURL: "https://rdap.lacnic.net/rdap"},
		{start: 2239756288, end: 2240000000, rdapURL: "https://rdap.afrinic.net/rdap"},
	}

	b.lastUpdated = time.Now()
	b.loaded = true
}

// GetDNSServer 获取TLD对应的RDAP服务器
func (b *RDAPBootstrap) GetDNSServer(tld string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dns[strings.ToLower(tld)]
}

// GetASN_RDAPServer 获取ASN对应的RDAP服务器
func (b *RDAPBootstrap) GetASN_RDAPServer(asn int) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, r := range b.asnRanges {
		if asn >= r.start && asn <= r.end {
			return r.rdapURL
		}
	}
	return ""
}

// ==============================
// 查询函数
// ==============================

// QueryRDAP 查询域名的RDAP信息
func QueryRDAP(domain string) (*RDAPResult, error) {
	return QueryRDAPWithContext(context.Background(), &RDAPQueryOptions{Domain: domain})
}

// QueryRDAPWithContext 使用上下文查询RDAP信息
func QueryRDAPWithContext(ctx context.Context, opts *RDAPQueryOptions) (*RDAPResult, error) {
	if opts == nil || opts.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10
	}

	// Step 1: 发现RDAP服务器
	rdapBaseURL, err := discoverRDAPServer(ctx, opts.Domain)
	if err != nil {
		return nil, fmt.Errorf("发现RDAP服务器失败: %w", err)
	}

	// Step 2: 查询RDAP
	rdapURL := fmt.Sprintf("%s/domain/%s", strings.TrimRight(rdapBaseURL, "/"), opts.Domain)

	body, err := rdapHTTPRequest(ctx, rdapURL, opts)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &RDAPResult{
		RawJSON:   body,
		QueryTime: startTime,
		Server:    rdapBaseURL,
	}

	if err := json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("解析RDAP响应失败: %w", err)
	}

	return result, nil
}

// QueryRDAP_IP 查询IP地址的RDAP信息
func QueryRDAP_IP(ip string) (*RDAPIPResult, error) {
	return QueryRDAP_IPWithContext(context.Background(), &RDAPQueryOptions{IP: ip})
}

// QueryRDAP_IPWithContext 使用上下文查询IP地址的RDAP信息
func QueryRDAP_IPWithContext(ctx context.Context, opts *RDAPQueryOptions) (*RDAPIPResult, error) {
	if opts == nil || opts.IP == "" {
		return nil, fmt.Errorf("IP地址不能为空")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10
	}

	// Step 1: 发现RDAP服务器（通过IANA bootstrap）
	rdapBaseURL, err := discoverIP_RDAPServer(opts.IP)
	if err != nil {
		return nil, fmt.Errorf("发现IP RDAP服务器失败: %w", err)
	}

	// Step 2: 查询RDAP
	rdapURL := fmt.Sprintf("%s/ip/%s", strings.TrimRight(rdapBaseURL, "/"), opts.IP)

	body, err := rdapHTTPRequest(ctx, rdapURL, opts)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &RDAPIPResult{
		RawJSON:   body,
		QueryTime: startTime,
		Server:    rdapBaseURL,
	}

	if err := json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("解析RDAP IP响应失败: %w", err)
	}

	return result, nil
}

// QueryRDAP_ASN 查询ASN的RDAP信息
func QueryRDAP_ASN(asn string) (*RDAPASNResult, error) {
	return QueryRDAP_ASNWithContext(context.Background(), &RDAPQueryOptions{ASN: asn})
}

// QueryRDAP_ASNWithContext 使用上下文查询ASN的RDAP信息
func QueryRDAP_ASNWithContext(ctx context.Context, opts *RDAPQueryOptions) (*RDAPASNResult, error) {
	if opts == nil || opts.ASN == "" {
		return nil, fmt.Errorf("ASN不能为空")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10
	}

	// 清理ASN格式
	asnNum := extractASNNumber(opts.ASN)
	if asnNum == 0 {
		return nil, fmt.Errorf("无效的ASN: %s", opts.ASN)
	}

	// Step 1: 发现RDAP服务器
	rdapBaseURL, err := discoverASN_RDAPServer(asnNum)
	if err != nil {
		return nil, fmt.Errorf("发现ASN RDAP服务器失败: %w", err)
	}

	// Step 2: 查询RDAP
	rdapURL := fmt.Sprintf("%s/autnum/%d", strings.TrimRight(rdapBaseURL, "/"), asnNum)

	body, err := rdapHTTPRequest(ctx, rdapURL, opts)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &RDAPASNResult{
		RawJSON:   body,
		QueryTime: startTime,
		Server:    rdapBaseURL,
	}

	if err := json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("解析RDAP ASN响应失败: %w", err)
	}

	return result, nil
}

// QueryRDAP_Entity 查询Entity的RDAP信息
func QueryRDAP_Entity(handle string) (*RDAPEntityResult, error) {
	return QueryRDAP_EntityWithContext(context.Background(), &RDAPQueryOptions{EntityHandle: handle})
}

// QueryRDAP_EntityWithContext 使用上下文查询Entity的RDAP信息
func QueryRDAP_EntityWithContext(ctx context.Context, opts *RDAPQueryOptions) (*RDAPEntityResult, error) {
	if opts == nil || opts.EntityHandle == "" {
		return nil, fmt.Errorf("Entity Handle不能为空")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10
	}

	// Entity需要先知道在哪个RIR，可以通过Handle后缀判断
	rdapBaseURL := discoverEntityRDAPServer(opts.EntityHandle)

	// Step 2: 查询RDAP
	rdapURL := fmt.Sprintf("%s/entity/%s", strings.TrimRight(rdapBaseURL, "/"), opts.EntityHandle)

	body, err := rdapHTTPRequest(ctx, rdapURL, opts)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &RDAPEntityResult{
		RawJSON:   body,
		QueryTime: startTime,
		Server:    rdapBaseURL,
	}

	if err := json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("解析RDAP Entity响应失败: %w", err)
	}

	return result, nil
}

// ==============================
// 内部辅助函数
// ==============================

// rdapHTTPRequest 执行RDAP HTTP请求
func rdapHTTPRequest(ctx context.Context, url string, opts *RDAPQueryOptions) ([]byte, error) {
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: time.Duration(opts.Timeout) * time.Second,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建RDAP请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RDAP查询失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RDAP服务器返回错误: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取RDAP响应失败: %w", err)
	}

	return body, nil
}

// discoverRDAPServer 通过IANA bootstrap发现域名RDAP服务器
func discoverRDAPServer(ctx context.Context, domain string) (string, error) {
	tld := extractTLD(domain)
	if tld == "" {
		return "", fmt.Errorf("无法提取TLD: %s", domain)
	}

	// 先尝试bootstrap缓存
	bootstrap := GetRDAPBootstrap()
	if server := bootstrap.GetDNSServer(tld); server != "" {
		return server, nil
	}

	// 回退到IANA RDAP
	return fmt.Sprintf("https://rdap.iana.org/%s", tld), nil
}

// discoverIP_RDAPServer 发现IP RDAP服务器
// 使用IANA分配的主要RIR CIDR块进行正确的IP范围匹配
func discoverIP_RDAPServer(ipStr string) (string, error) {
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return "", fmt.Errorf("无效的IP地址: %s", ipStr)
	}

	bootstrap := GetRDAPBootstrap()
	bootstrap.mu.RLock()
	defer bootstrap.mu.RUnlock()

	for _, r := range bootstrap.ipRanges {
		_, ipNet, err := net.ParseCIDR(r.cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(parsedIP) {
			return r.rdapURL, nil
		}
	}

	// 回退：按IP版本默认使用ARIN（v4）或APNIC（v6）
	if parsedIP.To4() != nil {
		return "https://rdap.arin.net/registry", nil
	}
	return "https://rdap.apnic.net", nil
}

// discoverASN_RDAPServer 发现ASN RDAP服务器
func discoverASN_RDAPServer(asn int) (string, error) {
	bootstrap := GetRDAPBootstrap()
	if server := bootstrap.GetASN_RDAPServer(asn); server != "" {
		return server, nil
	}
	return "", fmt.Errorf("未找到ASN %d对应的RDAP服务器", asn)
}

// discoverEntityRDAPServer 通过Entity Handle判断RDAP服务器
func discoverEntityRDAPServer(handle string) string {
	upper := strings.ToUpper(handle)
	switch {
	case strings.HasSuffix(upper, "-ARIN"):
		return "https://rdap.arin.net/registry"
	case strings.HasSuffix(upper, "-RIPE"):
		return "https://rdap.ripe.net"
	case strings.HasSuffix(upper, "-AP"):
		return "https://rdap.apnic.net"
	case strings.HasSuffix(upper, "-APNIC"):
		return "https://rdap.apnic.net"
	case strings.HasSuffix(upper, "-LACNIC"):
		return "https://rdap.lacnic.net/rdap"
	case strings.HasSuffix(upper, "-AFRINIC"):
		return "https://rdap.afrinic.net/rdap"
	default:
		// 尝试通过bootstrap查找
		return "https://rdap.arin.net/registry"
	}
}

// getKnownRDAPServer 获取已知的RDAP服务器地址（向后兼容）
func getKnownRDAPServer(tld string) string {
	bootstrap := GetRDAPBootstrap()
	return bootstrap.GetDNSServer(tld)
}
