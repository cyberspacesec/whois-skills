package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newCacheCmd 缓存运维子命令组。
// 操作的是全局 WhoisCache（GetCache），与查询路径共享同一实例。
func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "WHOIS 缓存运维（查看/查询/删除/清空）",
		Long: `管理 whois 库的全局缓存实例（GetCache）。查询类子命令（whois/ip/asn 等）
的结果会写入此缓存，cache 子命令用于运维与调试。

注意：全局缓存默认为进程内本地缓存，每次 CLI 调用都是新进程，
因此 cache stats/get 看到的是本次进程内（含懒加载初始化）的状态。
要观察跨查询的缓存命中，请在 serve 常驻模式下通过 HTTP 端点查看，
或在脚本中先 apply 一份启用 Redis 的库配置再查询。`,
	}

	cmd.AddCommand(newCacheStatsCmd())
	cmd.AddCommand(newCacheGetCmd())
	cmd.AddCommand(newCacheDeleteCmd())
	cmd.AddCommand(newCacheClearCmd())
	cmd.AddCommand(newCacheClearExpiredCmd())
	cmd.AddCommand(newCacheASNCmd())

	return cmd
}

// newCacheStatsCmd 显示缓存统计。
func newCacheStatsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "stats",
		Short: "显示缓存统计信息",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cache := whois.GetCache()
			stats := cache.GetStats()

			if asJSON {
				b, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}

			// WhoisCache.GetStats 在禁用时返回 {"enabled":false}，
			// 启用时返回 provider 的统计（含 type/entries/hits 等，无 enabled 键）。
			if v, ok := stats["enabled"]; ok && v == false {
				fmt.Fprintln(out, "缓存: 已禁用")
				return nil
			}
			fmt.Fprintf(out, "缓存: 已启用\n")
			fmt.Fprintf(out, "  类型: %v\n", stats["type"])
			fmt.Fprintf(out, "  条目数: %v\n", stats["entries"])
			fmt.Fprintf(out, "  命中: %v\n", stats["hits"])
			fmt.Fprintf(out, "  未命中: %v\n", stats["misses"])
			fmt.Fprintf(out, "  过期: %v\n", stats["expired"])
			fmt.Fprintf(out, "  总请求: %v\n", stats["requests"])
			if hr, ok := stats["hit_rate"]; ok {
				fmt.Fprintf(out, "  命中率: %.2f%%\n", hr)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newCacheGetCmd 查询指定域名的缓存条目。
func newCacheGetCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "get <domain>",
		Short: "查询指定域名的缓存条目",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cache := whois.GetCache()
			entry, ok := cache.Get(args[0])
			if !ok || entry == nil {
				fmt.Fprintf(out, "缓存中未找到: %s\n", args[0])
				return nil
			}

			if asJSON {
				b, err := json.MarshalIndent(entry, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}

			fmt.Fprintf(out, "域名: %s\n", args[0])
			fmt.Fprintf(out, "  缓存时间: %s\n", entry.CachedAt.Format(time.RFC3339))
			fmt.Fprintf(out, "  过期时间: %s\n", entry.ExpiresAt.Format(time.RFC3339))
			if entry.Info != nil {
				if entry.Info.Domain != nil {
					fmt.Fprintf(out, "  域名: %s\n", entry.Info.Domain.Domain)
				}
				if entry.Info.Registrar != nil {
					fmt.Fprintf(out, "  注册商: %s\n", entry.Info.Registrar.Name)
				}
			}
			fmt.Fprintf(out, "  原始响应长度: %d 字节\n", len(entry.RawResponse))
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON（含完整 WhoisInfo）")
	return c
}

// newCacheDeleteCmd 删除指定域名的缓存条目。
func newCacheDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <domain>",
		Short: "删除指定域名的缓存条目",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cache := whois.GetCache()
			cache.Delete(args[0])
			fmt.Fprintf(out, "已删除缓存条目: %s\n", args[0])
			return nil
		},
	}
}

// newCacheClearCmd 清空全部缓存。
func newCacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "清空全部缓存",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cache := whois.GetCache()
			cache.Clear()
			fmt.Fprintln(out, "已清空全部缓存")
			return nil
		},
	}
}

// newCacheClearExpiredCmd 清理过期缓存条目。
func newCacheClearExpiredCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-expired",
		Short: "清理过期的缓存条目",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cache := whois.GetCache()
			cache.ClearExpired()
			fmt.Fprintln(out, "已清理过期缓存条目")
			return nil
		},
	}
}

// newCacheASNCmd ASN 详情缓存子命令（独立于域名缓存）。
func newCacheASNCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asn",
		Short: "ASN 详情缓存管理",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部 ASN 详情缓存",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			m := whois.GetASNDetailCache()
			if len(m) == 0 {
				fmt.Fprintln(out, "ASN 详情缓存为空")
				return nil
			}
			b, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(out, string(b))
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "清空 ASN 详情缓存",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			whois.ClearASNDetailCache()
			fmt.Fprintln(out, "已清空 ASN 详情缓存")
			return nil
		},
	})
	return cmd
}
