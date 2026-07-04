package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/sirupsen/logrus"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// 由 -ldflags 注入的版本信息（见 Makefile / Dockerfile）。
// 未注入时为零值，version 命令会显示 "dev"。
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// 全局 flag 值（由 cobra 绑定）
var (
	flagConfig    string
	flagLogLevel  string
	flagLogFormat string
	flagTimeout   int
	flagUseProxy  bool
	flagProxyFile string
	flagOutput    string // 输出格式：json/text/raw
)

// newRootCmd 构建根命令。
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "whois-hacker",
		Short: "Whois Hacker - 一站式 WHOIS 域名情报查询工具",
		Long: `Whois Hacker 是一个面向 AI 的 WHOIS 域名情报工具包。

它既能作为常驻服务运行（serve 子命令，暴露 HTTP/MCP API），
也能通过子命令直接调用 SDK 的全部能力：域名/IP/ASN/RDAP 查询、
反向查询、批量处理、关联分析、质量评估、可注册性检测、
差异对比、IDN 转换、格式检测、多格式导出等。

示例：
  # 直接查询域名
  whois-hacker whois example.com

  # 启动 HTTP 服务
  whois-hacker serve --host 0.0.0.0 --port 8080

  # 批量查询
  whois-hacker batch domains.txt --format json

  # 查看版本
  whois-hacker version`,
		SilenceUsage: true,
	}

	// 全局持久 flag（所有子命令继承）
	pf := root.PersistentFlags()
	pf.StringVar(&flagConfig, "config", "config/config.yaml", "配置文件路径")
	pf.StringVar(&flagLogLevel, "log-level", "info", "日志级别 (debug/info/warn/error)")
	pf.StringVar(&flagLogFormat, "log-format", "text", "日志格式 (text/json)")
	pf.IntVar(&flagTimeout, "timeout", 10, "查询超时时间（秒）")
	pf.BoolVar(&flagUseProxy, "use-proxy", false, "是否使用代理")
	pf.StringVar(&flagProxyFile, "proxy-file", "config/proxies.json", "代理列表文件")
	pf.StringVar(&flagOutput, "format", "json", "输出格式 (json/text/raw)，仅查询类子命令生效")

	// 让 viper 绑定，便于后续按 key 读取（可选）
	_ = viper.BindPFlags(pf)

	// 注册子命令
	root.AddCommand(
		newServeCmd(),
		newVersionCmd(),
		newWhoisCmd(),
		newIPCmd(),
		newASNCmd(),
		newRDAPCmd(),
		newAvailabilityCmd(),
		newDiffCmd(),
		newQualityCmd(),
		newCorrelationCmd(),
		newBatchCmd(),
		newIDNCmd(),
		newFormatCmd(),
		newExportCmd(),
		newServersCmd(),
		newConfigCmd(),
		newCacheCmd(),
		newProxyCmd(),
		newMetricsCmd(),
		newToolsCmd(),
	)

	// Cobra 自动补全子命令
	root.CompletionOptions.DisableDefaultCmd = false

	// PersistentPreRun：在所有子命令前执行，完成日志与代理初始化
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// version / completion 不需要初始化日志/代理
		if cmd.Name() == "version" || cmd.Name() == "completion" || cmd.Name() == "help" {
			return nil
		}
		setupLogging()
		// 加载配置文件（若存在），仅作为默认值，命令行 flag 优先
		loadConfigFromFile()
		// 初始化代理池（若启用）
		if flagUseProxy {
			if err := whois.LoadProxiesFromFile(flagProxyFile); err != nil {
				logrus.Warnf("加载代理配置失败: %v", err)
			} else {
				logrus.Debug("代理功能已启用")
			}
		}
		// 仅对需要网络查询的命令初始化 WHOIS 服务器管理器，
		// 避免纯本地命令（idn/servers/format/export 等）输出无关日志。
		if needsServerManager(cmd.Name()) {
			sm := whois.GetServerManager()
			if err := sm.LoadFromFile("config/servers.json"); err != nil {
				if !os.IsNotExist(err) {
					logrus.Warnf("加载 WHOIS 服务器配置失败: %v", err)
				}
			}
		}
		return nil
	}

	return root
}

// needsServerManager 判断子命令是否需要加载 WHOIS 服务器管理器。
func needsServerManager(name string) bool {
	switch name {
	case "whois", "ip", "asn", "availability", "diff", "quality", "correlation", "batch",
		"asn-prefixes", "asn-ip-ranges", "analyze", "profile", "registrars":
		return true
	default:
		return false
	}
}
