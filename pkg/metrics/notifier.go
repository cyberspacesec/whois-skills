package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"

	"github.com/sirupsen/logrus"
)

// EmailNotifier 邮件通知处理器
type EmailNotifier struct {
	// SMTP服务器配置
	Host     string
	Port     int
	Username string
	Password string

	// 发件人
	From string

	// 收件人列表
	To []string

	// 抄送列表
	CC []string
}

// SlackNotifier Slack通知处理器
type SlackNotifier struct {
	// Webhook URL
	WebhookURL string

	// 频道
	Channel string

	// 用户名
	Username string

	// 图标
	IconEmoji string
}

// WebhookNotifier Webhook通知处理器
type WebhookNotifier struct {
	// Webhook URL
	URL string

	// HTTP方法
	Method string

	// 请求头
	Headers map[string]string

	// 自定义消息格式化函数
	FormatMessage func(*AlertEvent) (interface{}, error)
}

// String 返回告警级别的字符串表示
func (l AlertLevel) String() string {
	switch l {
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case CriticalLevel:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Notify 发送邮件通知
func (n *EmailNotifier) Notify(event *AlertEvent) error {
	// 构建邮件内容
	subject := fmt.Sprintf("[%v] %s", event.Level, event.RuleName)
	body := fmt.Sprintf(`
告警详情：
- 规则：%s
- 级别：%v
- 消息：%s
- 当前值：%.2f
- 阈值：%.2f
- 持续时间：%v
- 时间：%v
`,
		event.RuleName,
		event.Level,
		event.Message,
		event.CurrentValue,
		event.Threshold,
		event.Duration,
		event.Timestamp.Format("2006-01-02 15:04:05"),
	)

	// 构建邮件头
	headers := make(map[string]string)
	headers["From"] = n.From
	headers["To"] = strings.Join(n.To, ",")
	if len(n.CC) > 0 {
		headers["Cc"] = strings.Join(n.CC, ",")
	}
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	// 构建完整邮件
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// 发送邮件
	auth := smtp.PlainAuth("", n.Username, n.Password, n.Host)
	addr := fmt.Sprintf("%s:%d", n.Host, n.Port)

	recipients := append(n.To, n.CC...)
	err := smtp.SendMail(addr, auth, n.From, recipients, []byte(message))
	if err != nil {
		return fmt.Errorf("发送邮件失败: %v", err)
	}

	return nil
}

// Notify 发送Slack通知
func (n *SlackNotifier) Notify(event *AlertEvent) error {
	// 构建消息
	message := map[string]interface{}{
		"channel":    n.Channel,
		"username":   n.Username,
		"icon_emoji": n.IconEmoji,
		"attachments": []map[string]interface{}{
			{
				"color": getSlackColor(event.Level),
				"title": fmt.Sprintf("[%v] %s", event.Level, event.RuleName),
				"text":  event.Message,
				"fields": []map[string]interface{}{
					{
						"title": "当前值",
						"value": fmt.Sprintf("%.2f", event.CurrentValue),
						"short": true,
					},
					{
						"title": "阈值",
						"value": fmt.Sprintf("%.2f", event.Threshold),
						"short": true,
					},
					{
						"title": "持续时间",
						"value": event.Duration.String(),
						"short": true,
					},
					{
						"title": "时间",
						"value": event.Timestamp.Format("2006-01-02 15:04:05"),
						"short": true,
					},
				},
			},
		},
	}

	// 发送请求
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("发送Slack消息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack响应错误: %s", resp.Status)
	}

	return nil
}

// Notify 发送Webhook通知
func (n *WebhookNotifier) Notify(event *AlertEvent) error {
	var payload interface{}
	var err error

	// 使用自定义格式化函数或默认格式
	if n.FormatMessage != nil {
		payload, err = n.FormatMessage(event)
		if err != nil {
			return fmt.Errorf("格式化消息失败: %v", err)
		}
	} else {
		payload = map[string]interface{}{
			"rule_name":     event.RuleName,
			"level":         event.Level,
			"message":       event.Message,
			"current_value": event.CurrentValue,
			"threshold":     event.Threshold,
			"duration":      event.Duration.String(),
			"timestamp":     event.Timestamp,
		}
	}

	// 序列化消息
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	// 创建请求
	method := n.Method
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, n.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for k, v := range n.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送Webhook请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook响应错误: %s", resp.Status)
	}

	return nil
}

// getSlackColor 根据告警级别获取Slack消息颜色
func getSlackColor(level AlertLevel) string {
	switch level {
	case InfoLevel:
		return "#36a64f" // 绿色
	case WarnLevel:
		return "#ffcc00" // 黄色
	case ErrorLevel:
		return "#ff9900" // 橙色
	case CriticalLevel:
		return "#ff0000" // 红色
	default:
		return "#cccccc" // 灰色
	}
}

// RegisterDefaultNotifiers 注册默认通知处理器
func (am *AlertManager) RegisterDefaultNotifiers() {
	// 注册邮件通知处理器
	am.RegisterNotifier("email", &EmailNotifier{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "alert@example.com",
		Password: "your-password",
		From:     "alert@example.com",
		To:       []string{"admin@example.com"},
		CC:       []string{"ops@example.com"},
	})

	// 注册Slack通知处理器
	am.RegisterNotifier("slack", &SlackNotifier{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		Channel:    "#alerts",
		Username:   "WhoisHacker",
		IconEmoji:  ":warning:",
	})

	// 注册Webhook通知处理器
	am.RegisterNotifier("webhook", &WebhookNotifier{
		URL:    "https://api.example.com/alerts",
		Method: http.MethodPost,
		Headers: map[string]string{
			"Authorization": "Bearer your-token",
		},
	})

	logrus.Info("默认通知处理器已注册")
}
