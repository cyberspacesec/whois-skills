package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newToolsCmd 工具函数子命令组，暴露 SDK 的本地解析/提取工具。
func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "本地解析与提取工具（不联网）",
		Long: `暴露 whois 库的本地工具函数，纯本地计算，不发起网络请求。

  tools ip-parse <ip>     解析 IP WHOIS 原始文本为结构化信息（从 stdin 读）
  tools domain <domain>   解析域名为结构化信息（TLD/SLD/子域名）
  tools tld <domain>      提取域名的有效 TLD（含复合 TLD 如 .co.uk）
  tools normalize <type> <value>  规范化联系人字段（phone/name/email）
  tools asn-prefixes <asn>        统计 ASN 的 IPv4/IPv6 啟前缀数（需先查 ASN）
  tools asn-ip-ranges <asn>       按 ASN 取宣告的 IP 段`,
	}

	cmd.AddCommand(newToolsIPParseCmd())
	cmd.AddCommand(newToolsDomainCmd())
	cmd.AddCommand(newToolsTLDCmd())
	cmd.AddCommand(newToolsNormalizeCmd())
	cmd.AddCommand(newToolsASNPrefixesCmd())
	cmd.AddCommand(newToolsASNIPRangesCmd())

	return cmd
}

// newToolsIPParseCmd 解析 IP WHOIS 原始文本（ParseIPWhois）。
func newToolsIPParseCmd() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "ip-parse <ip>",
		Short: "解析 IP WHOIS 原始文本为结构化信息",
		Long: `从 stdin 或 --file 读取 IP WHOIS 原始文本，解析为结构化 IPWhoisInfo。
不联网，纯本地解析。

示例：
  cat raw.txt | whois-hacker tools ip-parse 8.8.8.8
  whois-hacker tools ip-parse 8.8.8.8 --file raw.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var reader io.Reader = os.Stdin
			if file != "" {
				f, err := os.Open(file)
				if err != nil {
					return err
				}
				defer f.Close()
				reader = f
			}
			raw, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			info, err := whois.ParseIPWhois(string(raw), args[0])
			if err != nil {
				return fmt.Errorf("解析失败: %w", err)
			}
			b, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "原始文本文件（默认 stdin）")
	return c
}

// newToolsDomainCmd 解析域名为结构化信息（ParseDomain）。
func newToolsDomainCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "domain <domain>",
		Short: "解析域名为结构化信息（TLD/SLD/子域名）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			info, err := whois.ParseDomain(args[0])
			if err != nil {
				return fmt.Errorf("解析失败: %w", err)
			}
			if asJSON {
				b, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintf(out, "完整域名: %s\n", info.FullDomain)
			fmt.Fprintf(out, "  顶级域: %s\n", info.TLD)
			fmt.Fprintf(out, "  域名: %s\n", info.Domain)
			fmt.Fprintf(out, "  子域名: %s\n", info.SubDomain)
			fmt.Fprintf(out, "  通配符基础: %s\n", info.WildcardBase)
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newToolsTLDCmd 提取有效 TLD（ExtractEffectiveTLD/ExtractTLD）。
func newToolsTLDCmd() *cobra.Command {
	var simple bool
	c := &cobra.Command{
		Use:   "tld <domain>",
		Short: "提取域名的有效 TLD",
		Long: `提取域名的顶级域。默认提取有效 TLD（含复合 TLD 如 .co.uk），
--simple 提取简单 TLD（最后一段）。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			var tld string
			var err error
			if simple {
				tld, err = whois.ExtractTLD(args[0])
			} else {
				tld, err = whois.ExtractEffectiveTLD(args[0])
			}
			if err != nil {
				return fmt.Errorf("提取失败: %w", err)
			}
			fmt.Fprintln(out, tld)
			return nil
		},
	}
	c.Flags().BoolVar(&simple, "simple", false, "提取简单 TLD（最后一段）")
	return c
}

// newToolsNormalizeCmd 规范化联系人字段（NormalizeContactField）。
func newToolsNormalizeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "normalize <type> <value>",
		Short: "规范化联系人字段（phone/name/email）",
		Long: `规范化联系人字段值。<type> 可为 phone/name/email。

示例：
  whois-hacker tools normalize phone "+1.234 567-8900"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			result := whois.NormalizeContactField(args[1], args[0])
			fmt.Fprintln(out, result)
			return nil
		},
	}
	return c
}

// newToolsASNPrefixesCmd 统计 ASN 嘟前缀数（ASNToPrefixCount）。
// 需要先查 ASN 详情（联网），再统计前缀。
func newToolsASNPrefixesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "asn-prefixes <asn>",
		Short: "统计 ASN 的 IPv4/IPv6 嘟前缀数",
		Long: `查询 ASN 详情并统计其宣告的 IPv4/IPv6 前缀数（ASNToPrefixCount）。
需联网查询 ASN 详情。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			asnInt, err := whois.ParseASNString(args[0])
			if err != nil {
				return fmt.Errorf("无效 ASN: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			detail, err := whois.QueryASNWithContext(ctx, &whois.ASNQueryOptions{
				ASN:             asnInt,
				Source:          whois.ASNSourceAll,
				Timeout:         flagTimeout,
				IncludePrefixes: true,
			})
			if err != nil {
				return fmt.Errorf("查询 ASN 失败: %w", err)
			}
			ipv4, ipv6 := whois.ASNToPrefixCount(detail)
			fmt.Fprintf(out, "ASN %s:\n", args[0])
			fmt.Fprintf(out, "  IPv4 前缀数: %d\n", ipv4)
			fmt.Fprintf(out, "  IPv6 前缀数: %d\n", ipv6)
			return nil
		},
	}
	return c
}

// newToolsASNIPRangesCmd 按 ASN 取 IP 段（GetIPRangesByASN）。
func newToolsASNIPRangesCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "asn-ip-ranges <asn>",
		Short: "按 ASN 取宣告的 IP 段",
		Long: `查询 ASN 宣告的 IPv4/IPv6 IP 段（GetIPRangesByASN）。
<asn> 为字符串形式（如 "13335" 或 "AS13335"）。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			v4, v6, err := whois.GetIPRangesByASN(args[0])
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}
			if asJSON {
				b, err := json.MarshalIndent(map[string]interface{}{
					"asn":    args[0],
					"ipv4":   v4,
					"ipv6":   v6,
				}, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintf(out, "ASN %s:\n", args[0])
			fmt.Fprintf(out, "  IPv4 段 (%d):\n", len(v4))
			for _, r := range v4 {
				fmt.Fprintf(out, "    %s\n", r)
			}
			fmt.Fprintf(out, "  IPv6 段 (%d):\n", len(v6))
			for _, r := range v6 {
				fmt.Fprintf(out, "    %s\n", r)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}
