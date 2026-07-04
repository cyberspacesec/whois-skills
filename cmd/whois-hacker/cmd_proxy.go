package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newProxyCmd 代理池运维子命令组。
func newProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "代理池运维（查看/统计/设置）",
		Long: `管理 whois 库的全局代理池（GetProxyPool）。

  proxy list    列出全部代理及其状态
  proxy stats   显示代理池汇总统计
  proxy set     设置单个全局 WHOIS 代理（SetWhoisProxy）

注意：与全局 --use-proxy flag 配合使用。--use-proxy 从 --proxy-file
加载代理列表到全局池；本子命令用于查看与手动设置。`,
	}

	cmd.AddCommand(newProxyListCmd())
	cmd.AddCommand(newProxyStatsCmd())
	cmd.AddCommand(newProxySetCmd())

	return cmd
}

// newProxyListCmd 列出全部代理及其状态。
func newProxyListCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "列出全部代理及其状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			pool := whois.GetProxyPool()
			stats := pool.GetProxyStats()

			if asJSON {
				b, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}

			proxies, _ := stats["proxies"].(map[string]interface{})
			if len(proxies) == 0 {
				fmt.Fprintln(out, "代理池为空（用 --use-proxy --proxy-file <file> 加载）")
				return nil
			}
			fmt.Fprintf(out, "代理池（共 %v 个，可用 %v 个）:\n", stats["total"], stats["available"])
			for addr, s := range proxies {
				m, _ := s.(map[string]interface{})
				avail := "❌"
				if a, _ := m["available"].(bool); a {
					avail = "✅"
				}
				fmt.Fprintf(out, "  %s %s | 失败: %v | 平均响应: %vms | 最后检查: %v\n",
					avail, addr, m["failure_count"], m["avg_response_time"], m["last_check"])
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newProxyStatsCmd 显示代理池汇总统计。
func newProxyStatsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "stats",
		Short: "显示代理池汇总统计",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			pool := whois.GetProxyPool()
			stats := pool.GetProxyStats()

			if asJSON {
				b, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}

			fmt.Fprintf(out, "代理总数: %v\n", stats["total"])
			fmt.Fprintf(out, "可用代理: %v\n", stats["available"])
			if lu, ok := stats["last_updated"]; ok && lu != nil {
				fmt.Fprintf(out, "最后更新: %v\n", lu)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newProxySetCmd 设置单个全局 WHOIS 代理（SetWhoisProxy）。
func newProxySetCmd() *cobra.Command {
	var (
		pType    string
		username string
		password string
		timeout  int
	)
	c := &cobra.Command{
		Use:   "set <address>",
		Short: "设置单个全局 WHOIS 代理（替换默认客户端）",
		Long: `设置全局 WHOIS 代理（SetWhoisProxy），替换默认客户端 defaultClient，
后续查询走此单个代理。

注意：这与 --use-proxy 的代理池（ProxyPool，多代理轮询）是两套机制：
  - proxy set     → 设置单个代理到 defaultClient（不进 ProxyPool，proxy list 看不到）
  - --use-proxy   → 加载代理列表到 ProxyPool，查询时轮询

address 格式为 host:port，--type 指定 socks5 或 http。

示例：
  whois-hacker proxy set 127.0.0.1:1080 --type socks5
  whois-hacker proxy set proxy.example.com:8080 --type http --user alice --pass secret`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg := &whois.ProxyConfig{
				Address:  args[0],
				Type:     pType,
				Username: username,
				Password: password,
				Timeout:  timeout,
				Enabled:  true,
			}
			if err := whois.SetWhoisProxy(cfg); err != nil {
				return err
			}
			fmt.Fprintf(out, "✅ 已设置全局代理: %s://%s\n", pType, args[0])
			return nil
		},
	}
	c.Flags().StringVar(&pType, "type", "socks5", "代理类型 (socks5/http)")
	c.Flags().StringVar(&username, "user", "", "代理用户名")
	c.Flags().StringVar(&password, "pass", "", "代理密码")
	c.Flags().IntVar(&timeout, "timeout", 30, "代理超时（秒）")
	return c
}
