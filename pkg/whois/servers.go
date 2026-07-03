package whois

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	domain_util "github.com/cyberspacesec/go-domain-util"
	"github.com/sirupsen/logrus"
)

// WhoisServerManager 管理WHOIS服务器列表
type WhoisServerManager struct {
	// 互斥锁保护并发访问
	mu sync.RWMutex

	// WHOIS服务器映射表 (TLD -> 服务器地址)
	servers map[string]string

	// 服务器健康状态
	serverHealth map[string]*ServerHealth

	// 默认WHOIS服务器（用于未知TLD）
	defaultServer string

	// 最后一次更新时间
	lastUpdated time.Time

	// 配置文件路径
	configPath string

	// 健康检查配置
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	maxFailures         int
}

// ServerHealth 服务器健康状态
type ServerHealth struct {
	// 上次检查时间
	LastCheck time.Time

	// 是否可用
	IsHealthy bool

	// 连续失败次数
	FailureCount int

	// 平均响应时间（毫秒）
	AvgResponseTime int64

	// 最近响应时间记录
	recentResponseTimes []int64

	// 最大响应时间记录数
	maxResponseRecords int
}

// serverManager 是全局的WHOIS服务器管理器实例
var serverManager *WhoisServerManager
var managerOnce sync.Once

// GetServerManager 返回单例的WhoisServerManager实例
func GetServerManager() *WhoisServerManager {
	managerOnce.Do(func() {
		serverManager = &WhoisServerManager{
			servers:             make(map[string]string),
			serverHealth:        make(map[string]*ServerHealth),
			defaultServer:       "whois.iana.org",
			lastUpdated:         time.Now(),
			healthCheckInterval: 5 * time.Minute,
			healthCheckTimeout:  10 * time.Second,
			maxFailures:         3,
		}
		// 初始化默认服务器列表
		serverManager.loadDefaultServers()
		// 启动健康检查
		go serverManager.startHealthCheck()
	})
	return serverManager
}

// GetWhoisServer 根据TLD获取对应的WHOIS服务器地址
func (m *WhoisServerManager) GetWhoisServer(domain string) (string, error) {
	// 从域名提取TLD
	tld := extractTLD(domain)
	if tld == "" {
		return "", fmt.Errorf("无效的域名或无法提取TLD: %s", domain)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 尝试获取健康的服务器
	if server, ok := m.getHealthyServer(tld); ok {
		return server, nil
	}

	// 如果没有健康的服务器，返回默认服务器
	return m.defaultServer, nil
}

// getHealthyServer 获取健康的WHOIS服务器
func (m *WhoisServerManager) getHealthyServer(tld string) (string, bool) {
	server, ok := m.servers[tld]
	if !ok {
		return "", false
	}

	// 检查服务器健康状态
	health, ok := m.serverHealth[server]
	if !ok || !health.IsHealthy {
		return "", false
	}

	return server, true
}

// checkServerHealth 检查服务器健康状态
func (m *WhoisServerManager) checkServerHealth(server string) {
	conn, err := net.DialTimeout("tcp", server+":43", m.healthCheckTimeout)
	startTime := time.Now()

	health := m.getOrCreateServerHealth(server)

	if err != nil {
		m.mu.Lock()
		health.FailureCount++
		health.IsHealthy = health.FailureCount < m.maxFailures
		health.LastCheck = time.Now()
		m.mu.Unlock()
		return
	}
	defer conn.Close()

	// 发送测试查询
	_, err = conn.Write([]byte("example.com\r\n"))
	if err != nil {
		m.mu.Lock()
		health.FailureCount++
		health.IsHealthy = health.FailureCount < m.maxFailures
		health.LastCheck = time.Now()
		m.mu.Unlock()
		return
	}

	// 更新健康状态
	responseTime := time.Since(startTime).Milliseconds()
	m.mu.Lock()
	health.FailureCount = 0
	health.IsHealthy = true
	health.LastCheck = time.Now()
	m.updateResponseTime(health, responseTime)
	m.mu.Unlock()
}

// getOrCreateServerHealth 获取或创建服务器健康状态记录
func (m *WhoisServerManager) getOrCreateServerHealth(server string) *ServerHealth {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.serverHealth[server]
	if !exists {
		health = &ServerHealth{
			IsHealthy:           true,
			maxResponseRecords:  100,
			recentResponseTimes: make([]int64, 0, 100),
		}
		m.serverHealth[server] = health
	}
	return health
}

// updateResponseTime 更新服务器响应时间统计
func (m *WhoisServerManager) updateResponseTime(health *ServerHealth, responseTime int64) {
	// 添加新的响应时间记录
	if len(health.recentResponseTimes) >= health.maxResponseRecords {
		health.recentResponseTimes = health.recentResponseTimes[1:]
	}
	health.recentResponseTimes = append(health.recentResponseTimes, responseTime)

	// 计算平均响应时间
	var total int64
	for _, rt := range health.recentResponseTimes {
		total += rt
	}
	health.AvgResponseTime = total / int64(len(health.recentResponseTimes))
}

// startHealthCheck 启动定期健康检查
func (m *WhoisServerManager) startHealthCheck() {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		servers := make([]string, 0, len(m.servers))
		for _, server := range m.servers {
			servers = append(servers, server)
		}
		m.mu.RUnlock()

		// 并发检查所有服务器
		var wg sync.WaitGroup
		for _, server := range servers {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				m.checkServerHealth(s)
			}(server)
		}
		wg.Wait()

		// 记录健康检查结果
		m.logHealthStatus()
	}
}

// logHealthStatus 记录服务器健康状态
func (m *WhoisServerManager) logHealthStatus() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for server, health := range m.serverHealth {
		logrus.WithFields(logrus.Fields{
			"server":          server,
			"healthy":         health.IsHealthy,
			"failure_count":   health.FailureCount,
			"avg_response_ms": health.AvgResponseTime,
			"last_check":      health.LastCheck,
		}).Info("WHOIS服务器健康状态")
	}
}

// GetServerStats 获取服务器统计信息
func (m *WhoisServerManager) GetServerStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})
	serverStats := make(map[string]interface{})

	for server, health := range m.serverHealth {
		serverStats[server] = map[string]interface{}{
			"healthy":               health.IsHealthy,
			"failure_count":         health.FailureCount,
			"avg_response_ms":       health.AvgResponseTime,
			"last_check":            health.LastCheck,
			"recent_response_times": health.recentResponseTimes,
		}
	}

	stats["servers"] = serverStats
	stats["total_servers"] = len(m.servers)
	stats["healthy_servers"] = m.countHealthyServers()
	stats["last_updated"] = m.lastUpdated

	return stats
}

// countHealthyServers 统计健康的服务器数量
func (m *WhoisServerManager) countHealthyServers() int {
	count := 0
	for _, health := range m.serverHealth {
		if health.IsHealthy {
			count++
		}
	}
	return count
}

// extractTLD 从域名中提取顶级域名
func extractTLD(domain string) string {
	// 移除可能的协议前缀和路径
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if idx := strings.Index(domain, "/"); idx > 0 {
		domain = domain[:idx]
	}
	domain = strings.Trim(domain, ".")

	// 使用go-domain-util进行准确的TLD提取（使用Public Suffix List）
	fldDomain, err := domain_util.FldDomain(domain)
	if err == nil && fldDomain != "" {
		// 从first-level domain中提取TLD
		parts := strings.Split(fldDomain, ".")
		if len(parts) >= 2 {
			return strings.ToLower(strings.Join(parts[1:], "."))
		}
		return strings.ToLower(parts[0])
	}

	// 回退到简单的最后一部分提取
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToLower(parts[len(parts)-1])
}

// ExtractTLD 从域名中提取顶级域名（公开API）
func ExtractTLD(domain string) (string, error) {
	tld := extractTLD(domain)
	if tld == "" {
		return "", fmt.Errorf("无效的域名或无法提取TLD: %s", domain)
	}
	return tld, nil
}

// ExtractEffectiveTLD 使用Public Suffix List提取有效的顶级域名
func ExtractEffectiveTLD(domain string) (string, error) {
	hostEntry, err := domain_util.NewHostEntry(domain)
	if err != nil {
		return "", fmt.Errorf("解析域名失败: %w", err)
	}
	return hostEntry.Tld, nil
}

// DomainInfo 域名解析信息
type DomainInfo struct {
	// 完整域名
	FullDomain string `json:"full_domain"`

	// 顶级域名
	TLD string `json:"tld"`

	// 域名（SLD + TLD）
	Domain string `json:"domain"`

	// 子域名
	SubDomain string `json:"sub_domain"`

	// 通配符基础域名
	WildcardBase string `json:"wildcard_base"`
}

// ParseDomain 解析域名为结构化信息
func ParseDomain(domain string) (*DomainInfo, error) {
	hostEntry, err := domain_util.NewHostEntry(domain)
	if err != nil {
		return nil, fmt.Errorf("解析域名失败: %w", err)
	}
	return &DomainInfo{
		FullDomain:   hostEntry.Host,
		TLD:          hostEntry.Tld,
		Domain:       hostEntry.Domain,
		SubDomain:    hostEntry.SubName,
		WildcardBase: hostEntry.WildcardBase,
	}, nil
}

// UpdateServer 更新或添加单个WHOIS服务器映射
func (m *WhoisServerManager) UpdateServer(tld, server string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.servers[strings.ToLower(tld)] = server
	m.lastUpdated = time.Now()
}

// UpdateServers 批量更新WHOIS服务器映射
func (m *WhoisServerManager) UpdateServers(servers map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for tld, server := range servers {
		m.servers[strings.ToLower(tld)] = server
	}
	m.lastUpdated = time.Now()
}

// SetDefaultServer 设置默认WHOIS服务器
func (m *WhoisServerManager) SetDefaultServer(server string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultServer = server
}

// LoadFromFile 从配置文件加载WHOIS服务器列表
func (m *WhoisServerManager) LoadFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("无法读取WHOIS服务器配置文件: %w", err)
	}

	var servers map[string]string
	if err := json.Unmarshal(data, &servers); err != nil {
		return fmt.Errorf("解析WHOIS服务器配置文件失败: %w", err)
	}

	m.UpdateServers(servers)
	m.configPath = filePath

	return nil
}

// SaveToFile 将当前WHOIS服务器列表保存到配置文件
func (m *WhoisServerManager) SaveToFile(filePath string) error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.servers, "", "  ")
	m.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("序列化WHOIS服务器配置失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入WHOIS服务器配置文件失败: %w", err)
	}

	m.configPath = filePath
	return nil
}

// GetAllServers 获取所有WHOIS服务器映射
func (m *WhoisServerManager) GetAllServers() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建副本以避免并发问题
	result := make(map[string]string, len(m.servers))
	for k, v := range m.servers {
		result[k] = v
	}

	return result
}

// GetLastUpdated 获取最后更新时间
func (m *WhoisServerManager) GetLastUpdated() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.lastUpdated
}

// loadDefaultServers 加载默认WHOIS服务器列表
func (m *WhoisServerManager) loadDefaultServers() {
	// 使用一个更全面的内置服务器列表
	defaultServers := map[string]string{
		// IANA预留顶级域名
		"com": "whois.verisign-grs.com",
		"net": "whois.verisign-grs.com",
		"org": "whois.pir.org",
		"int": "whois.iana.org",
		"edu": "whois.educause.edu",
		"gov": "whois.dotgov.gov",
		"mil": "whois.nic.mil",

		// 通用顶级域名
		"info":   "whois.afilias.net",
		"biz":    "whois.nic.biz",
		"name":   "whois.nic.name",
		"pro":    "whois.afilias.net",
		"museum": "whois.museum",
		"aero":   "whois.aero",
		"coop":   "whois.nic.coop",
		"travel": "whois.nic.travel",
		"mobi":   "whois.dotmobiregistry.net",
		"cat":    "whois.nic.cat",
		"jobs":   "whois.nic.jobs",
		"tel":    "whois.nic.tel",
		"asia":   "whois.nic.asia",

		// 新通用顶级域名
		"xyz":    "whois.nic.xyz",
		"top":    "whois.nic.top",
		"club":   "whois.nic.club",
		"site":   "whois.nic.site",
		"online": "whois.nic.online",
		"vip":    "whois.nic.vip",
		"shop":   "whois.nic.shop",
		"app":    "whois.nic.google",
		"dev":    "whois.nic.google",
		"store":  "whois.nic.store",
		"blog":   "whois.nic.blog",
		"art":    "whois.nic.art",
		"design": "whois.nic.design",
		"io":     "whois.nic.io",
		"co":     "whois.nic.co",
		"ai":     "whois.nic.ai",

		// 国家和地区顶级域名
		"cn": "whois.cnnic.cn",
		"hk": "whois.hkirc.hk",
		"tw": "whois.twnic.net.tw",
		"jp": "whois.jprs.jp",
		"kr": "whois.kr",
		"in": "whois.registry.in",
		"uk": "whois.nic.uk",
		"ru": "whois.tcinet.ru",
		"de": "whois.denic.de",
		"fr": "whois.nic.fr",
		"nl": "whois.domain-registry.nl",
		"it": "whois.nic.it",
		"es": "whois.nic.es",
		"au": "whois.auda.org.au",
		"nz": "whois.irs.net.nz",
		"br": "whois.registro.br",
		"mx": "whois.mx",
		"ca": "whois.cira.ca",
		"us": "whois.nic.us",
		"eu": "whois.eu",
		"me": "whois.nic.me",
		"cc": "ccwhois.verisign-grs.com",

		// 更多国家代码顶级域名
		"ac": "whois.nic.ac",
		"ae": "whois.aeda.net.ae",
		"af": "whois.nic.af",
		"ag": "whois.nic.ag",
		"am": "whois.amnic.net",
		"as": "whois.nic.as",
		"at": "whois.nic.at",
		"be": "whois.dns.be",
		"bz": "whois.afilias-grs.info",
		"ch": "whois.nic.ch",
		"cl": "whois.nic.cl",
		"cr": "whois.nic.cr",
		"cx": "whois.nic.cx",
		"cz": "whois.nic.cz",
		"dk": "whois.dk-hostmaster.dk",
		"fo": "whois.nic.fo",
		"gg": "whois.gg",
		"gi": "whois2.afilias-grs.net",
		"gs": "whois.nic.gs",
		"ht": "whois.nic.ht",
		"im": "whois.nic.im",
		"is": "whois.isnic.is",
		"je": "whois.je",
		"kz": "whois.nic.kz",
		"li": "whois.nic.li",
		"lt": "whois.domreg.lt",
		"lu": "whois.dns.lu",
		"lv": "whois.nic.lv",
		"md": "whois.nic.md",
		"ms": "whois.nic.ms",
		"mu": "whois.nic.mu",
		"my": "whois.mynic.my",
		"no": "whois.norid.no",
		"nu": "whois.nic.nu",
		"pe": "kero.yachay.pe",
		"pl": "whois.dns.pl",
		"pm": "whois.nic.pm",
		"pt": "whois.dns.pt",
		"re": "whois.nic.re",
		"ro": "whois.rotld.ro",
		"rs": "whois.rnids.rs",
		"sb": "whois.nic.sb",
		"se": "whois.iis.se",
		"sg": "whois.sgnic.sg",
		"sh": "whois.nic.sh",
		"si": "whois.arnes.si",
		"sk": "whois.sk-nic.sk",
		"sm": "whois.nic.sm",
		"so": "whois.nic.so",
		"st": "whois.nic.st",
		"su": "whois.tcinet.ru",
		"tf": "whois.nic.tf",
		"th": "whois.thnic.co.th",
		"tj": "whois.nic.tj",
		"tk": "whois.dot.tk",
		"tl": "whois.nic.tl",
		"tm": "whois.nic.tm",
		"to": "whois.tonic.to",
		"tr": "whois.nic.tr",
		"ua": "whois.ua",
		"ug": "whois.co.ug",
		"uy": "whois.nic.org.uy",
		"uz": "whois.cctld.uz",
		"vc": "whois2.afilias-grs.net",
		"ve": "whois.nic.ve",
		"vg": "whois.nic.vg",
		"wf": "whois.nic.wf",
		"ws": "whois.website.ws",
		"yt": "whois.nic.yt",
		"za": "whois.registry.net.za",
	}

	m.UpdateServers(defaultServers)
	logrus.Infof("已加载 %d 个默认WHOIS服务器配置", len(defaultServers))
}

// InitWhoisServerManager 初始化WHOIS服务器管理器并从配置文件加载（如果提供）
func InitWhoisServerManager(configPath string) error {
	manager := GetServerManager()

	if configPath != "" {
		// 尝试从配置文件加载，但不强制要求文件存在
		if err := manager.LoadFromFile(configPath); err != nil {
			if !os.IsNotExist(err) {
				// 只有在文件存在但加载失败时返回错误
				return err
			}
			// 文件不存在，尝试创建包含默认配置的文件
			logrus.Infof("WHOIS服务器配置文件不存在，将创建默认配置: %s", configPath)
			if err := manager.SaveToFile(configPath); err != nil {
				logrus.Warnf("创建默认配置文件失败: %v", err)
			}
		} else {
			logrus.Infof("已从 %s 加载 WHOIS 服务器配置", configPath)
		}
	}

	return nil
}

// DiscoverWhoisServer 从IANA发现TLD的WHOIS服务器
func (m *WhoisServerManager) DiscoverWhoisServer(tld string) (string, error) {
	conn, err := net.DialTimeout("tcp", "whois.iana.org:43", m.healthCheckTimeout)
	if err != nil {
		return "", fmt.Errorf("连接IANA服务器失败: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(tld + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("发送IANA查询失败: %w", err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, conn)
	if err != nil {
		return "", fmt.Errorf("读取IANA响应失败: %w", err)
	}

	server := extractReferralServer(buf.String())
	if server == "" {
		return "", fmt.Errorf("未找到%s的WHOIS服务器", tld)
	}

	// 缓存发现的服务器
	m.UpdateServer(tld, server)
	return server, nil
}

// RefreshServerList 自动刷新WHOIS服务器列表
func (m *WhoisServerManager) RefreshServerList() error {
	m.mu.RLock()
	tlds := make([]string, 0, len(m.servers))
	for tld := range m.servers {
		tlds = append(tlds, tld)
	}
	m.mu.RUnlock()

	var lastErr error
	refreshed := 0
	for _, tld := range tlds {
		if _, err := m.DiscoverWhoisServer(tld); err != nil {
			lastErr = err
			continue
		}
		refreshed++
	}

	logrus.Infof("WHOIS服务器列表刷新完成: %d/%d 成功", refreshed, len(tlds))

	if lastErr != nil && refreshed == 0 {
		return lastErr
	}
	return nil
}
