package security

import (
	"net/http"
	"sync"
	"time"
)

// RequestLog 请求日志记录
type RequestLog struct {
	// 请求时间
	Timestamp time.Time `json:"timestamp"`

	// 请求方法
	Method string `json:"method"`

	// 请求路径
	Path string `json:"path"`

	// 客户端IP
	ClientIP string `json:"client_ip"`

	// API密钥ID（如果有）
	APIKeyID string `json:"api_key_id,omitempty"`

	// 响应状态码
	StatusCode int `json:"status_code"`

	// 处理时间（毫秒）
	ProcessTime int64 `json:"process_time"`

	// 错误信息（如果有）
	Error string `json:"error,omitempty"`
}

// RequestLogger 请求日志记录器
type RequestLogger struct {
	mu sync.RWMutex

	// 最近的请求日志
	recentLogs []RequestLog

	// 最大日志条数
	maxLogs int
}

var (
	defaultLogger *RequestLogger
	loggerOnce    sync.Once
)

// GetRequestLogger 获取请求日志记录器实例
func GetRequestLogger() *RequestLogger {
	loggerOnce.Do(func() {
		defaultLogger = NewRequestLogger(1000) // 默认保存最近1000条日志
	})
	return defaultLogger
}

// NewRequestLogger 创建新的请求日志记录器
func NewRequestLogger(maxLogs int) *RequestLogger {
	return &RequestLogger{
		recentLogs: make([]RequestLog, 0, maxLogs),
		maxLogs:    maxLogs,
	}
}

// AddLog 添加请求日志
func (l *RequestLogger) AddLog(log RequestLog) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 如果日志数量超过限制，移除最旧的日志
	if len(l.recentLogs) >= l.maxLogs {
		l.recentLogs = l.recentLogs[1:]
	}

	l.recentLogs = append(l.recentLogs, log)
}

// GetRecentLogs 获取最近的请求日志
func (l *RequestLogger) GetRecentLogs() []RequestLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 创建副本
	logs := make([]RequestLog, len(l.recentLogs))
	copy(logs, l.recentLogs)

	return logs
}

// AuthMiddleware 创建API认证中间件
func AuthMiddleware(requiredPermission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// 创建请求日志
			log := RequestLog{
				Timestamp: startTime,
				Method:    r.Method,
				Path:      r.URL.Path,
				ClientIP:  getClientIP(r),
			}

			// 检查API密钥
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				log.StatusCode = http.StatusUnauthorized
				log.Error = "Missing API key"
				log.ProcessTime = time.Since(startTime).Milliseconds()
				GetRequestLogger().AddLog(log)

				SendErrorResponse(w, http.StatusUnauthorized, "Missing API key")
				return
			}

			// 验证API密钥
			key, err := GetAPIKeyManager().ValidateKey(apiKey, requiredPermission)
			if err != nil {
				log.StatusCode = http.StatusUnauthorized
				log.Error = err.Error()
				log.ProcessTime = time.Since(startTime).Milliseconds()
				GetRequestLogger().AddLog(log)

				SendErrorResponse(w, http.StatusUnauthorized, err.Error())
				return
			}

			// 记录API密钥ID
			log.APIKeyID = key.ID

			// 检查速率限制
			if !checkRateLimit(key, r) {
				log.StatusCode = http.StatusTooManyRequests
				log.Error = "Rate limit exceeded"
				log.ProcessTime = time.Since(startTime).Milliseconds()
				GetRequestLogger().AddLog(log)

				SendErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			// 包装ResponseWriter以捕获状态码
			rw := &responseWriter{ResponseWriter: w}

			// 调用下一个处理器
			next.ServeHTTP(rw, r)

			// 完成请求日志
			log.StatusCode = rw.statusCode
			log.ProcessTime = time.Since(startTime).Milliseconds()
			GetRequestLogger().AddLog(log)
		})
	}
}

// responseWriter 包装http.ResponseWriter以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// getClientIP 获取客户端真实IP地址
func getClientIP(r *http.Request) string {
	// 尝试从X-Forwarded-For头获取
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		return forwardedFor
	}

	// 尝试从X-Real-IP头获取
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// 使用RemoteAddr
	return r.RemoteAddr
}

// respondWithError 返回错误响应
func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	SendErrorResponse(w, statusCode, message)
}

// 速率限制相关
var (
	rateLimiters = make(map[string]*rateLimiter)
	limiterMu    sync.RWMutex
)

type rateLimiter struct {
	lastReset  time.Time
	count      int
	rateLimit  int
	windowSize time.Duration
}

// checkRateLimit 检查是否超出速率限制
func checkRateLimit(key *APIKey, r *http.Request) bool {
	limiterKey := key.ID + ":" + getClientIP(r)

	limiterMu.Lock()
	defer limiterMu.Unlock()

	limiter, exists := rateLimiters[limiterKey]
	if !exists {
		limiter = &rateLimiter{
			lastReset:  time.Now(),
			rateLimit:  key.RateLimit,
			windowSize: time.Minute,
		}
		rateLimiters[limiterKey] = limiter
	}

	// 检查是否需要重置计数器
	if time.Since(limiter.lastReset) > limiter.windowSize {
		limiter.count = 0
		limiter.lastReset = time.Now()
	}

	// 检查是否超出限制
	if limiter.count >= limiter.rateLimit {
		return false
	}

	limiter.count++
	return true
}

// cleanupRateLimiters 清理过期的速率限制器
func cleanupRateLimiters() {
	limiterMu.Lock()
	defer limiterMu.Unlock()

	for key, limiter := range rateLimiters {
		if time.Since(limiter.lastReset) > time.Hour {
			delete(rateLimiters, key)
		}
	}
}

// StartRateLimitCleanup 启动速率限制器清理
func StartRateLimitCleanup() {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			cleanupRateLimiters()
		}
	}()
}
