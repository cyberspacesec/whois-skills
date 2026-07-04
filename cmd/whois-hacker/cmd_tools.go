package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	whoisparser "github.com/likexian/whois-parser"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newIDNCmd IDN 国际化域名转换。
func newIDNCmd() *cobra.Command {
	var action string
	cmd := &cobra.Command{
		Use:   "idn <domain>",
		Short: "国际化域名（IDN）Punycode 转换",
		Long: `在 Unicode 与 Punycode 之间转换国际化域名。

action:
  to-ascii   Unicode → Punycode（默认）
  to-unicode Punycode → Unicode
  normalize  规范化（自动判断方向）
  detect     检测是否为 IDN`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			var (
				result string
				err    error
				isIDN  bool
			)
			switch action {
			case "to-ascii":
				result, err = whois.UnicodeToPunycode(domain)
			case "to-unicode":
				result, err = whois.PunycodeToUnicode(domain)
			case "normalize":
				result, err = whois.NormalizeDomain(domain)
			case "detect":
				isIDN = whois.IsIDN(domain)
				return outputJSON(map[string]interface{}{
					"domain":  domain,
					"is_idn":  isIDN,
				})
			default:
				return fmt.Errorf("未知 action: %s", action)
			}
			if err != nil {
				return fmt.Errorf("转换失败: %w", err)
			}
			return outputJSON(map[string]string{
				"input":  domain,
				"output": result,
				"action": action,
			})
		},
	}
	cmd.Flags().StringVar(&action, "action", "to-ascii", "操作 (to-ascii/to-unicode/normalize/detect)")
	return cmd
}

// newFormatCmd WHOIS 格式检测与清洗。
func newFormatCmd() *cobra.Command {
	var detectOnly bool
	cmd := &cobra.Command{
		Use:   "format [file]",
		Short: "WHOIS 原始文本格式检测与清洗",
		Long: `检测 WHOIS 原始响应的格式（如 verisign/ripe 等），并可选地清洗为
统一格式。从文件读取，未指定文件时从 stdin 读取。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var reader io.Reader = os.Stdin
			if len(args) == 1 && args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("打开文件失败: %w", err)
				}
				defer f.Close()
				reader = f
			}

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, reader); err != nil {
				return fmt.Errorf("读取失败: %w", err)
			}
			raw := buf.String()

			format := whois.DetectWhoisFormat(raw)
			if detectOnly {
				return outputJSON(map[string]interface{}{
					"format": string(format),
				})
			}
			cleaned := whois.FormatRawResponse(raw)
			return outputJSON(map[string]string{
				"format":   string(format),
				"cleaned":  cleaned,
			})
		},
	}
	cmd.Flags().BoolVar(&detectOnly, "detect-only", false, "仅检测格式，不清洗")
	return cmd
}

// newExportCmd 导出 WHOIS 信息为 JSON/CSV/Markdown。
func newExportCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export <domain>",
		Short: "导出域名 WHOIS 信息为 JSON/CSV/Markdown",
		Long:  `查询域名 WHOIS 信息并导出为指定格式，输出到 stdout。`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			info, err := queryInfo(ctx, args[0])
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}
			var buf bytes.Buffer
			switch format {
			case "json":
				if err := whois.ExportToJSON(info, &buf); err != nil {
					return fmt.Errorf("导出 JSON 失败: %w", err)
				}
			case "csv":
				if err := whois.ExportToCSV(info, &buf); err != nil {
					return fmt.Errorf("导出 CSV 失败: %w", err)
				}
			case "markdown", "md":
				if err := whois.ExportToMarkdown(info, &buf); err != nil {
					return fmt.Errorf("导出 Markdown 失败: %w", err)
				}
			default:
				return fmt.Errorf("未知格式: %s（支持 json/csv/markdown）", format)
			}
			outputRaw(buf.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "导出格式 (json/csv/markdown)")
	return cmd
}

// newServersCmd WHOIS 服务器管理子命令组。
// 直接调用 `servers`（无子命令）兼容旧行为：列出全部映射，可按 --tld 过滤。
func newServersCmd() *cobra.Command {
	var tld string
	cmd := &cobra.Command{
		Use:   "servers",
		Short: "WHOIS 服务器映射管理（列出/统计/发现/刷新/保存）",
		Long: `管理 WHOIS 服务器映射（WhoisServerManager）。

直接调用 ` + "`servers`" + ` 列出全部 TLD → 服务器映射（可按 --tld 过滤）。
子命令：
  servers list       列出全部映射（同直接调用）
  servers stats      显示服务器健康统计
  servers discover <tld>   在线发现指定 TLD 的 WHOIS 服务器
  servers refresh    刷新服务器列表
  servers save <file>      保存当前映射到文件`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 兼容旧用法：servers / servers --tld x
			sm := whois.GetServerManager()
			servers := sm.GetAllServers()
			if tld != "" {
				if s, ok := servers[tld]; ok {
					return outputJSON(map[string]string{tld: s})
				}
				return fmt.Errorf("未找到 TLD: %s", tld)
			}
			return outputJSON(servers)
		},
	}
	cmd.Flags().StringVar(&tld, "tld", "", "只查看指定 TLD 的服务器")

	cmd.AddCommand(newServersListCmd())
	cmd.AddCommand(newServersStatsCmd())
	cmd.AddCommand(newServersDiscoverCmd())
	cmd.AddCommand(newServersRefreshCmd())
	cmd.AddCommand(newServersSaveCmd())
	return cmd
}

// newServersListCmd 列出全部 WHOIS 服务器映射。
func newServersListCmd() *cobra.Command {
	var tld string
	c := &cobra.Command{
		Use:   "list",
		Short: "列出全部 WHOIS 服务器映射",
		RunE: func(cmd *cobra.Command, args []string) error {
			sm := whois.GetServerManager()
			servers := sm.GetAllServers()
			if tld != "" {
				if s, ok := servers[tld]; ok {
					return outputJSON(map[string]string{tld: s})
				}
				return fmt.Errorf("未找到 TLD: %s", tld)
			}
			return outputJSON(servers)
		},
	}
	c.Flags().StringVar(&tld, "tld", "", "只查看指定 TLD 的服务器")
	return c
}

// newServersStatsCmd 显示服务器健康统计。
func newServersStatsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "stats",
		Short: "显示 WHOIS 服务器健康统计",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			sm := whois.GetServerManager()
			stats := sm.GetServerStats()

			if asJSON {
				return outputJSON(stats)
			}
			fmt.Fprintf(out, "WHOIS 服务器统计:\n")
			fmt.Fprintf(out, "  总数: %v\n", stats["total_servers"])
			fmt.Fprintf(out, "  健康: %v\n", stats["healthy_servers"])
			if lu, ok := stats["last_updated"]; ok {
				fmt.Fprintf(out, "  最后更新: %v\n", lu)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newServersDiscoverCmd 在线发现指定 TLD 的 WHOIS 服务器。
func newServersDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover <tld>",
		Short: "在线发现指定 TLD 的 WHOIS 服务器",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			sm := whois.GetServerManager()
			server, err := sm.DiscoverWhoisServer(args[0])
			if err != nil {
				return fmt.Errorf("发现失败: %w", err)
			}
			fmt.Fprintf(out, "%s: %s\n", args[0], server)
			return nil
		},
	}
}

// newServersRefreshCmd 刷新服务器列表。
func newServersRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "刷新 WHOIS 服务器列表",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			sm := whois.GetServerManager()
			if err := sm.RefreshServerList(); err != nil {
				return fmt.Errorf("刷新失败: %w", err)
			}
			fmt.Fprintln(out, "✅ 服务器列表已刷新")
			return nil
		},
	}
}

// newServersSaveCmd 保存当前服务器映射到文件。
func newServersSaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save <file>",
		Short: "保存当前服务器映射到文件",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			sm := whois.GetServerManager()
			if err := sm.SaveToFile(args[0]); err != nil {
				return fmt.Errorf("保存失败: %w", err)
			}
			fmt.Fprintf(out, "✅ 已保存到 %s\n", args[0])
			return nil
		},
	}
}

// 确保 whoisparser 引用不被移除（queryInfo 在 cmd_analyze.go 用到）。
var _ *whoisparser.WhoisInfo
