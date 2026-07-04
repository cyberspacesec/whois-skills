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

// newServersCmd 列出 WHOIS 服务器映射。
func newServersCmd() *cobra.Command {
	var tld string
	cmd := &cobra.Command{
		Use:   "servers",
		Short: "列出 WHOIS 服务器映射",
		Long:  `列出内置的 TLD → WHOIS 服务器映射，可选按 TLD 过滤。`,
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
	cmd.Flags().StringVar(&tld, "tld", "", "只查看指定 TLD 的服务器")
	return cmd
}

// 确保 whoisparser 引用不被移除（queryInfo 在 cmd_analyze.go 用到）。
var _ *whoisparser.WhoisInfo
