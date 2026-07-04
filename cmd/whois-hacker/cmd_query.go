package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newWhoisCmd 域名 WHOIS 查询。
func newWhoisCmd() *cobra.Command {
	var (
		maxRetries      int
		validateResult  bool
		followReferral  bool
		requiredFields  []string
		raw             bool
	)
	cmd := &cobra.Command{
		Use:   "whois <domain>",
		Short: "查询域名 WHOIS 信息",
		Long:  `查询指定域名的 WHOIS 注册信息，包括注册商、注册人、日期、名称服务器等。`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()

			opts := &whois.QueryOptions{
				Domain:         domain,
				MaxRetries:     maxRetries,
				Timeout:        flagTimeout,
				UseProxy:       flagUseProxy,
				ValidateResult: validateResult,
				RequiredFields: requiredFields,
				FollowReferral: followReferral,
			}

			if raw {
				// 原始 WHOIS 文本
				res, err := whois.ExecuteQueryWithResultContext(ctx, opts)
				if err != nil {
					return fmt.Errorf("查询失败: %w", err)
				}
				return outputResult(res, res.RawResponse, nil)
			}

			res, err := whois.ExecuteQueryWithResultContext(ctx, opts)
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}
			return outputJSON(res)
		},
	}
	f := cmd.Flags()
	f.IntVar(&maxRetries, "max-retries", 5, "最大重试次数")
	f.BoolVar(&validateResult, "validate", false, "是否验证查询结果完整性")
	f.BoolVar(&followReferral, "follow-referral", true, "是否跟随 WHOIS 引导查询")
	f.StringSliceVar(&requiredFields, "required-fields", nil, "必需字段列表（逗号分隔）")
	f.BoolVar(&raw, "raw", false, "输出原始 WHOIS 文本")
	return cmd
}

// newIPCmd IP WHOIS 查询（IANA 引导 → RIR）。
func newIPCmd() *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "ip <ip>",
		Short: "查询 IP 的 WHOIS 信息（IANA 引导 → RIR）",
		Long:  `查询 IPv4/IPv6 地址的 WHOIS 信息，遵循 IANA 引导到 RIR 的标准流程。`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ip := args[0]
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()

			opts := &whois.IPWhoisOptions{
				IP:       ip,
				Timeout:  flagTimeout,
				UseProxy: flagUseProxy,
			}
			res, err := whois.QueryIPWithContext(ctx, opts)
			if err != nil {
				return fmt.Errorf("IP 查询失败: %w", err)
			}
			if raw {
				return outputResult(res, res.RawResponse, nil)
			}
			return outputJSON(res)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "输出原始 WHOIS 文本")
	return cmd
}

// newASNCmd ASN 查询（RADB + RDAP）。
func newASNCmd() *cobra.Command {
	var (
		source       string
		includePref  bool
		includeBGP   bool
	)
	cmd := &cobra.Command{
		Use:   "asn <asn>",
		Short: "查询 ASN 详情（RADB + RDAP）",
		Long: `查询自治系统号（ASN）的详情，包括名称、组织、国家、RIR、前缀列表、
BGP 关系等。支持 RADB / RDAP / all 三种数据源。

ASN 可带或不带 AS 前缀，如 "13335" 或 "AS13335"。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asnInt, err := whois.ParseASNString(args[0])
			if err != nil {
				return fmt.Errorf("ASN 解析失败: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()

			opts := &whois.ASNQueryOptions{
				ASN:             asnInt,
				Timeout:         flagTimeout,
				Source:          whois.ASNQuerySource(source),
				IncludePrefixes: includePref,
				IncludeBGP:      includeBGP,
			}
			detail, err := whois.QueryASNWithContext(ctx, opts)
			if err != nil {
				return fmt.Errorf("ASN 查询失败: %w", err)
			}
			return outputJSON(detail)
		},
	}
	f := cmd.Flags()
	f.StringVar(&source, "source", "all", "查询来源 (radb/rdap/all)")
	f.BoolVar(&includePref, "include-prefixes", true, "是否查询前缀信息")
	f.BoolVar(&includeBGP, "include-bgp", false, "是否查询 BGP 关系")
	return cmd
}

// newAvailabilityCmd 域名可注册性检测。
func newAvailabilityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "availability <domain>",
		Short: "检测域名是否可注册",
		Long:  `检测指定域名当前是否可以注册（available/registered/reserved/premium/blocked/unknown）。`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			res, err := whois.CheckDomainAvailabilityWithContext(ctx, args[0])
			if err != nil {
				return fmt.Errorf("可用性检测失败: %w", err)
			}
			return outputJSON(res)
		},
	}
	return cmd
}

// newRDAPCmd RDAP 标准查询（RFC 9083）。
func newRDAPCmd() *cobra.Command {
	rdapCmd := &cobra.Command{
		Use:   "rdap",
		Short: "RDAP 标准查询（RFC 9083）",
		Long:  `通过 RDAP（Registration Data Access Protocol）查询域名/IP/ASN/实体信息。`,
	}
	rdapCmd.AddCommand(
		newRDAPDomainCmd(),
		newRDAPIPCmd(),
		newRDAPASNCmd(),
		newRDAPEntityCmd(),
		newRDAPBootstrapCmd(),
	)
	return rdapCmd
}

// newRDAPBootstrapCmd 查看 RDAP bootstrap 映射（TLD/ASN → RDAP 服务器）。
func newRDAPBootstrapCmd() *cobra.Command {
	var (
		tld string
		asn string
	)
	c := &cobra.Command{
		Use:   "bootstrap",
		Short: "查看 RDAP bootstrap 映射（TLD/ASN → RDAP 服务器）",
		Long: `查看 RDAP bootstrap 映射，不发起 RDAP 查询，仅返回元数据。

  --tld <tld>   查看指定 TLD 的 RDAP 服务器
  --asn <asn>   查看指定 ASN 的 RDAP 服务器（如 13335 或 AS13335）

示例：
  whois-hacker rdap bootstrap --tld com
  whois-hacker rdap bootstrap --asn 13335`,
		RunE: func(cmd *cobra.Command, args []string) error {
			b := whois.GetRDAPBootstrap()

			if tld == "" && asn == "" {
				return fmt.Errorf("请用 --tld 或 --asn 指定查询项")
			}

			result := map[string]string{}
			if tld != "" {
				result["tld"] = tld
				result["rdap_server"] = b.GetDNSServer(tld)
			}
			if asn != "" {
				asnInt, err := whois.ParseASNString(asn)
				if err != nil {
					return fmt.Errorf("无效 ASN: %w", err)
				}
				result["asn"] = asn
				result["rdap_server"] = b.GetASN_RDAPServer(asnInt)
			}
			return outputJSON(result)
		},
	}
	c.Flags().StringVar(&tld, "tld", "", "按 TLD 查看 RDAP 服务器")
	c.Flags().StringVar(&asn, "asn", "", "按 ASN 查看 RDAP 服务器")
	return c
}

func newRDAPDomainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "domain <domain>",
		Short: "RDAP 查询域名",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			res, err := whois.QueryRDAPWithContext(ctx, &whois.RDAPQueryOptions{Domain: args[0], Timeout: flagTimeout})
			if err != nil {
				return fmt.Errorf("RDAP 域名查询失败: %w", err)
			}
			return outputJSON(res)
		},
	}
}

func newRDAPIPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ip <ip>",
		Short: "RDAP 查询 IP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			res, err := whois.QueryRDAP_IPWithContext(ctx, &whois.RDAPQueryOptions{IP: args[0], Timeout: flagTimeout})
			if err != nil {
				return fmt.Errorf("RDAP IP 查询失败: %w", err)
			}
			return outputJSON(res)
		},
	}
}

func newRDAPASNCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "asn <asn>",
		Short: "RDAP 查询 ASN",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			res, err := whois.QueryRDAP_ASNWithContext(ctx, &whois.RDAPQueryOptions{ASN: args[0], Timeout: flagTimeout})
			if err != nil {
				return fmt.Errorf("RDAP ASN 查询失败: %w", err)
			}
			return outputJSON(res)
		},
	}
}

func newRDAPEntityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "entity <handle>",
		Short: "RDAP 查询实体",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			res, err := whois.QueryRDAP_EntityWithContext(ctx, &whois.RDAPQueryOptions{EntityHandle: args[0], Timeout: flagTimeout})
			if err != nil {
				return fmt.Errorf("RDAP 实体查询失败: %w", err)
			}
			return outputJSON(res)
		},
	}
}
