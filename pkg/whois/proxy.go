package whois

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	// 代理地址
	Address string `json:"address"`

	// 代理类型 (socks5/http)
	Type string `json:"type"`

	// 用户名
	Username string `json:"username,omitempty"`

	// 密码
	Password string `json:"password,omitempty"`

	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`

	// 最大重试次数
	MaxRetries int `json:"max_retries,omitempty"`

	// 重试间隔（毫秒）
	RetryInterval int64 `json:"retry_interval,omitempty"`

	// 是否启用代理
	Enabled bool `json:"enabled"`

	// 内部拨号器
	dialer proxy.Dialer
}

// GetDialer 获取代理拨号器
func (c *ProxyConfig) GetDialer() (proxy.Dialer, error) {
	if c.dialer != nil {
		return c.dialer, nil
	}

	var err error
	switch c.Type {
	case "socks5":
		auth := &proxy.Auth{
			User:     c.Username,
			Password: c.Password,
		}
		c.dialer, err = proxy.SOCKS5("tcp", c.Address, auth, proxy.Direct)
	case "http":
		proxyURL := &url.URL{
			Scheme: "http",
			Host:   c.Address,
		}
		if c.Username != "" {
			proxyURL.User = url.UserPassword(c.Username, c.Password)
		}
		c.dialer = &httpProxyDialer{proxyURL: proxyURL}
	default:
		return nil, fmt.Errorf("不支持的代理类型: %s", c.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("创建代理拨号器失败: %w", err)
	}

	return c.dialer, nil
}

// httpProxyDialer HTTP代理拨号器
type httpProxyDialer struct {
	proxyURL *url.URL
}

// Dial 实现proxy.Dialer接口，通过HTTP CONNECT隧道连接目标服务器
func (d *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	// 连接到HTTP代理服务器
	proxyAddr := d.proxyURL.Host
	if !strings.Contains(proxyAddr, ":") {
		proxyAddr = proxyAddr + ":8080"
	}
	conn, err := net.Dial(network, proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("连接HTTP代理服务器失败: %w", err)
	}

	// 构建CONNECT请求
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)

	// 添加代理认证（如果配置了）
	if d.proxyURL.User != nil {
		username := d.proxyURL.User.Username()
		password, _ := d.proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
	}
	connectReq += "\r\n"

	// 发送CONNECT请求
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("发送HTTP代理CONNECT请求失败: %w", err)
	}

	// 读取代理响应
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取HTTP代理响应失败: %w", err)
	}

	// 检查是否返回200连接成功
	if !strings.Contains(line, "200") {
		conn.Close()
		return nil, fmt.Errorf("HTTP代理CONNECT失败: %s", strings.TrimSpace(line))
	}

	// 读取剩余响应头直到空行
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("读取HTTP代理响应头失败: %w", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	return conn, nil
}

// WhoisDialer 是一个自定义的WHOIS拨号函数，实现net.Dialer接口
type WhoisDialer struct {
	ProxyDialer proxy.Dialer
	Timeout     time.Duration
}

// Dial 实现net.Dialer接口
func (d *WhoisDialer) Dial(network, addr string) (net.Conn, error) {
	if d.ProxyDialer != nil {
		// 使用代理拨号
		return d.ProxyDialer.Dial(network, addr)
	}

	// 无代理时使用标准拨号
	dialer := &net.Dialer{Timeout: d.Timeout}
	return dialer.Dial(network, addr)
}

// ProxyPool 代理池
type ProxyPool struct {
	mu sync.RWMutex

	// 代理列表
	proxies []*ProxyConfig

	// 代理状态
	status map[string]*ProxyStatus

	// 当前代理索引
	currentIndex int

	// 最后更新时间
	lastUpdated time.Time
}

// ProxyStatus 代理状态
type ProxyStatus struct {
	// 是否可用
	Available bool

	// 连续失败次数
	FailureCount int

	// 平均响应时间（毫秒）
	AvgResponseTime int64

	// 最后检查时间
	LastCheck time.Time
}

var (
	defaultPool *ProxyPool
	poolOnce    sync.Once
)

// GetProxyPool 获取代理池实例
func GetProxyPool() *ProxyPool {
	poolOnce.Do(func() {
		defaultPool = &ProxyPool{
			proxies: make([]*ProxyConfig, 0),
			status:  make(map[string]*ProxyStatus),
		}
	})
	return defaultPool
}

// LoadProxiesFromFile 从文件加载代理配置
func LoadProxiesFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取代理配置文件失败: %w", err)
	}

	var configs []*ProxyConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("解析代理配置失败: %w", err)
	}

	pool := GetProxyPool()
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// 清空现有代理
	pool.proxies = make([]*ProxyConfig, 0, len(configs))
	pool.status = make(map[string]*ProxyStatus)

	// 添加新代理
	for _, config := range configs {
		// 设置默认值
		if config.Timeout <= 0 {
			config.Timeout = 10
		}
		if config.MaxRetries <= 0 {
			config.MaxRetries = 3
		}
		if config.RetryInterval <= 0 {
			config.RetryInterval = 1000
		}

		// 初始化拨号器
		if _, err := config.GetDialer(); err != nil {
			logrus.Warnf("初始化代理拨号器失败 [%s]: %v", config.Address, err)
			continue
		}

		pool.proxies = append(pool.proxies, config)
		pool.status[config.Address] = &ProxyStatus{
			Available: true,
			LastCheck: time.Now(),
		}
	}

	pool.currentIndex = 0
	pool.lastUpdated = time.Now()

	logrus.Infof("已加载 %d 个代理配置", len(configs))
	return nil
}

// GetNextProxy 获取下一个可用代理
func (p *ProxyPool) GetNextProxy() *ProxyConfig {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.proxies) == 0 {
		return nil
	}

	// 查找下一个可用代理
	startIndex := p.currentIndex
	for i := 0; i < len(p.proxies); i++ {
		index := (startIndex + i) % len(p.proxies)
		prx := p.proxies[index]
		if status := p.status[prx.Address]; status != nil && status.Available {
			p.currentIndex = (index + 1) % len(p.proxies)
			return prx
		}
	}

	// 如果没有可用代理，重置所有代理状态
	for _, status := range p.status {
		status.Available = true
		status.FailureCount = 0
	}

	// 返回第一个代理
	p.currentIndex = 1
	return p.proxies[0]
}

// MarkProxySuccess 标记代理请求成功
func (p *ProxyPool) MarkProxySuccess(prx *ProxyConfig, responseTime int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if status := p.status[prx.Address]; status != nil {
		status.Available = true
		status.FailureCount = 0
		status.LastCheck = time.Now()

		// 更新平均响应时间
		if status.AvgResponseTime == 0 {
			status.AvgResponseTime = responseTime
		} else {
			status.AvgResponseTime = (status.AvgResponseTime + responseTime) / 2
		}
	}
}

// MarkProxyFailure 标记代理请求失败
func (p *ProxyPool) MarkProxyFailure(prx *ProxyConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if status := p.status[prx.Address]; status != nil {
		status.FailureCount++
		status.LastCheck = time.Now()

		// 如果连续失败次数超过阈值，标记为不可用
		if status.FailureCount >= 3 {
			status.Available = false
		}
	}
}

// GetProxyStats 获取代理统计信息
func (p *ProxyPool) GetProxyStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})
	proxyStats := make(map[string]interface{})

	for _, prx := range p.proxies {
		status := p.status[prx.Address]
		proxyStats[prx.Address] = map[string]interface{}{
			"available":         status.Available,
			"failure_count":     status.FailureCount,
			"avg_response_time": status.AvgResponseTime,
			"last_check":        status.LastCheck,
		}
	}

	stats["proxies"] = proxyStats
	stats["total"] = len(p.proxies)
	stats["available"] = p.countAvailableProxies()
	stats["last_updated"] = p.lastUpdated

	return stats
}

// ProxyCount 获取代理总数
func (p *ProxyPool) ProxyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}

// countAvailableProxies 统计可用代理数量
func (p *ProxyPool) countAvailableProxies() int {
	count := 0
	for _, status := range p.status {
		if status.Available {
			count++
		}
	}
	return count
}

// StartProxyHealthCheck 启动代理健康检查
func (p *ProxyPool) StartProxyHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			p.checkProxyHealth()
		}
	}()

	logrus.Infof("代理健康检查已启动，间隔: %v", interval)
}

// checkProxyHealth 检查代理健康状态
func (p *ProxyPool) checkProxyHealth() {
	p.mu.RLock()
	proxies := make([]*ProxyConfig, len(p.proxies))
	copy(proxies, p.proxies)
	p.mu.RUnlock()

	for _, prx := range proxies {
		// 尝试通过代理连接到测试服务器
		startTime := time.Now()
		dialer, err := prx.GetDialer()
		if err != nil {
			p.MarkProxyFailure(prx)
			continue
		}

		conn, err := dialer.Dial("tcp", "whois.verisign-grs.com:43")
		if err != nil {
			p.MarkProxyFailure(prx)
			continue
		}
		conn.Close()

		// 记录成功
		responseTime := time.Since(startTime).Milliseconds()
		p.MarkProxySuccess(prx, responseTime)
	}
}

// extractReferralServer 从WHOIS响应中提取引导服务器
func extractReferralServer(data string) string {
	tokens := []string{
		"Registrar WHOIS Server: ",
		"whois: ",
	}
	for _, token := range tokens {
		start := strings.Index(data, token)
		if start != -1 {
			start += len(token)
			end := strings.Index(data[start:], "\n")
			if end != -1 {
				server := strings.TrimSpace(data[start : start+end])
				if server != "" && !strings.HasPrefix(strings.ToLower(server), "http") {
					return server
				}
			}
		}
	}
	return ""
}

// WhoisClient 自定义WHOIS客户端，封装whois库并支持代理
type WhoisClient struct {
	dialer        *WhoisDialer
	pool          *ProxyPool
	cache         *WhoisCache
	cacheDisabled bool
	rateLimiter   *RateLimiter
}

// NewWhoisClient 创建新的WHOIS客户端
func NewWhoisClient() *WhoisClient {
	return &WhoisClient{
		dialer: &WhoisDialer{
			Timeout: 30 * time.Second,
		},
	}
}

// SetProxyPool 设置代理池
func (c *WhoisClient) SetProxyPool(pool *ProxyPool) {
	c.pool = pool
}

// SetProxy 设置代理
func (c *WhoisClient) SetProxy(proxyDialer proxy.Dialer) {
	c.dialer.ProxyDialer = proxyDialer
}

// SetTimeout 设置超时时间
func (c *WhoisClient) SetTimeout(timeout time.Duration) {
	c.dialer.Timeout = timeout
}

// SetCache 设置缓存实例
func (c *WhoisClient) SetCache(cache *WhoisCache) {
	c.cache = cache
	c.cacheDisabled = false
}

// DisableCache 禁用缓存
func (c *WhoisClient) DisableCache() {
	c.cacheDisabled = true
}

// SetCacheTTL 设置缓存TTL（秒）
func (c *WhoisClient) SetCacheTTL(ttl int64) {
	ch := c.getCache()
	if ch != nil {
		ch.config.TTL = ttl
	}
}

// SetRateLimiter 设置速率限制器
func (c *WhoisClient) SetRateLimiter(limiter *RateLimiter) {
	c.rateLimiter = limiter
}

// getCache 获取缓存实例（支持禁用和自定义）
func (c *WhoisClient) getCache() *WhoisCache {
	if c.cacheDisabled {
		return nil
	}
	if c.cache != nil {
		return c.cache
	}
	return GetCache()
}

// Query 查询域名的WHOIS信息（使用代理池）
func (c *WhoisClient) Query(domain string) (string, error) {
	return c.QueryWithContext(context.Background(), domain)
}

// QueryWithContext 使用上下文查询域名的WHOIS信息
func (c *WhoisClient) QueryWithContext(ctx context.Context, domain string) (string, error) {
	// 检查上下文
	if ctx.Err() != nil {
		return "", NewWhoisError(ErrQueryTimeout, "查询被取消", ctx.Err())
	}

	// 尝试从缓存获取
	cache := c.getCache()
	if cache != nil {
		if entry, found := cache.Get(domain); found {
			return entry.RawResponse, nil
		}
	}

	// 如果设置了代理池，使用代理池查询
	if c.pool != nil {
		return c.queryWithProxyPoolContext(ctx, domain)
	}

	// 否则使用普通查询
	return c.queryDirectContext(ctx, domain)
}

// queryWithProxyPoolContext 使用代理池查询WHOIS信息
func (c *WhoisClient) queryWithProxyPoolContext(ctx context.Context, domain string) (string, error) {
	var lastErr error

	proxyCount := c.pool.ProxyCount()
	// 尝试使用不同代理查询
	for i := 0; i < proxyCount; i++ {
		select {
		case <-ctx.Done():
			return "", NewWhoisError(ErrQueryTimeout, "查询被取消", ctx.Err())
		default:
		}

		prx := c.pool.GetNextProxy()

		// 创建代理拨号器
		proxyDialer, err := prx.GetDialer()
		if err != nil {
			c.pool.MarkProxyFailure(prx)
			lastErr = err
			continue
		}

		// 设置当前代理
		c.dialer.ProxyDialer = proxyDialer

		// 执行查询
		response, err := c.queryDirectContext(ctx, domain)
		if err != nil {
			c.pool.MarkProxyFailure(prx)
			lastErr = err
			continue
		}

		// 查询成功
		c.pool.MarkProxySuccess(prx, 0)
		return response, nil
	}

	return "", NewWhoisError(ErrProxyFailed, fmt.Sprintf("所有代理均失败，最后错误: %v", lastErr), lastErr)
}

// queryDirectContext 直接查询WHOIS信息（不使用代理池）
func (c *WhoisClient) queryDirectContext(ctx context.Context, domain string) (string, error) {
	// 从域名中提取TLD
	tld := extractTLD(domain)
	if tld == "" {
		return "", fmt.Errorf("无效的域名格式: %s", domain)
	}

	// 获取对应TLD的WHOIS服务器地址
	server, err := c.getWhoisServer(tld)
	if err != nil {
		return "", fmt.Errorf("获取WHOIS服务器失败: %w", err)
	}

	// 速率限制检查
	if c.rateLimiter != nil && !c.rateLimiter.Allow(server) {
		return "", NewWhoisError(ErrRateLimited, "查询被限速", nil)
	}

	// 连接到WHOIS服务器并查询
	response, err := c.rawWhoisQuery(ctx, server, domain)
	if err != nil {
		return "", err
	}

	// 跟随WHOIS引导查询
	maxReferrals := 3 // 默认跟随3次引导
	for i := 0; i < maxReferrals; i++ {
		refServer := extractReferralServer(response)
		if refServer == "" || refServer == server {
			break
		}

		logrus.Debugf("跟随WHOIS引导: %s -> %s", server, refServer)
		refResponse, err := c.rawWhoisQuery(ctx, refServer, domain)
		if err != nil {
			break // 使用已有的结果
		}
		response += "\n" + refResponse
		server = refServer
	}

	// 尝试解析响应
	info, err := whoisparser.Parse(response)
	if err == nil {
		// 只有在成功解析时才缓存结果
		cache := c.getCache()
		if cache != nil {
			cache.Set(domain, &info, response)
		}
	}

	return response, nil
}

// rawWhoisQuery 发送原始WHOIS查询到指定服务器
func (c *WhoisClient) rawWhoisQuery(ctx context.Context, server, domain string) (string, error) {
	select {
	case <-ctx.Done():
		return "", NewWhoisError(ErrQueryTimeout, "查询被取消", ctx.Err())
	default:
	}

	conn, err := c.dialer.Dial("tcp", server+":43")
	if err != nil {
		return "", NewWhoisError(ErrServerConnectFailed, "连接WHOIS服务器失败", err)
	}
	defer conn.Close()

	// 设置连接超时
	if c.dialer.Timeout > 0 {
		deadline := time.Now().Add(c.dialer.Timeout)
		conn.SetDeadline(deadline)
	}

	// 发送查询请求
	_, err = conn.Write([]byte(domain + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("发送查询请求失败: %w", err)
	}

	// 读取响应
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, conn)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return buffer.String(), nil
}

// getWhoisServer 获取TLD对应的WHOIS服务器地址
func (c *WhoisClient) getWhoisServer(tld string) (string, error) {
	// 使用 WhoisServerManager 获取服务器地址
	manager := GetServerManager()
	return manager.GetWhoisServer(tld)
}

// SetWhoisProxy 设置whois查询时使用的代理
func SetWhoisProxy(cfg *ProxyConfig) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// 创建代理拨号器
	proxyDialer, err := cfg.GetDialer()
	if err != nil {
		return fmt.Errorf("创建代理拨号器失败: %w", err)
	}

	// 创建自定义WHOIS客户端
	client := NewWhoisClient()
	client.SetProxy(proxyDialer)

	// 设置超时时间
	if cfg.Timeout > 0 {
		client.SetTimeout(time.Duration(cfg.Timeout) * time.Second)
	}

	// 设置全局客户端
	defaultClient = client

	return nil
}

// 默认全局客户端
var defaultClient = NewWhoisClient()

// DirectWhois 使用自定义客户端直接进行WHOIS查询，绕过likexian/whois库，以支持代理
func DirectWhois(domain string) (string, error) {
	return DirectWhoisWithContext(context.Background(), domain)
}

// DirectWhoisWithContext 使用上下文和自定义客户端直接进行WHOIS查询
func DirectWhoisWithContext(ctx context.Context, domain string) (string, error) {
	return defaultClient.QueryWithContext(ctx, domain)
}

// isValidProxyAddress 验证代理地址格式是否有效
func isValidProxyAddress(address string) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}

	// 检查IP是否有效（如果是IP地址）
	ip := net.ParseIP(host)
	if host != "localhost" && ip == nil {
		// 尝试解析主机名
		_, err := net.LookupIP(host)
		if err != nil {
			return false
		}
	}

	// 验证端口
	portNum, err := net.LookupPort("tcp", port)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return false
	}

	return true
}
