package main

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// durationOf 把秒数转为 time.Duration。
func durationOf(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// setupLogging 按 flag 配置日志级别与格式。
func setupLogging() {
	level, err := logrus.ParseLevel(flagLogLevel)
	if err != nil {
		logrus.Warnf("解析日志级别失败: %v，使用默认级别: info", err)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	if flagLogFormat == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
}

// loadConfigFromFile 加载 YAML 配置文件。
// 命令行显式 flag 优先级高于配置文件（与原 main.go 行为一致）。
func loadConfigFromFile() {
	if flagConfig == "" {
		return
	}

	cfg, err := whois.LoadYAMLConfig(flagConfig)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("配置文件 %s 不存在，使用默认配置", flagConfig)
			return
		}
		logrus.Warnf("加载配置文件失败: %v", err)
		return
	}

	logrus.Infof("已从 %s 加载配置", flagConfig)

	// 仅当命令行未显式设置时，才用配置文件的值覆盖。
	// log 字段在这里覆盖；server/cache/proxy/metrics/alerts 由 serve 子命令的
	// applyServeConfigFromYAML 覆盖（查询类子命令只用到 log 与 proxy）。
	// 注意：这些 flag 均注册在 root 的 PersistentFlags 上，必须用 PersistentFlags().Changed
	// 检查；root.Flags() 的懒合并 PFlagSet 不会反映 PersistentFlags 的 Changed 状态。
	if !rootCmd.PersistentFlags().Changed("log-level") && cfg.Log.Level != "" {
		flagLogLevel = cfg.Log.Level
		setupLogging()
	}
	if !rootCmd.PersistentFlags().Changed("log-format") && cfg.Log.Format != "" {
		flagLogFormat = cfg.Log.Format
		setupLogging()
	}
	if !rootCmd.PersistentFlags().Changed("use-proxy") {
		flagUseProxy = cfg.Proxy.Enabled
	}
	if !rootCmd.PersistentFlags().Changed("proxy-file") && cfg.Proxy.File != "" {
		flagProxyFile = cfg.Proxy.File
	}
}

// rootCmd 在 init 阶段由 newRootCmd() 赋值，供 loadConfigFromFile 反查 flag 是否显式设置。
// 为避免循环，在 root.go 的 init 中赋值。
var rootCmd *cobra.Command
