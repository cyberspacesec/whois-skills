package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newMetricsCmd 可观测性查看与导出子命令组。
func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "WHOIS 指标查看与导出",
		Long: `查看与导出 whois 库的全局指标（GetGlobalMetrics）。

  metrics stats    显示全局内置指标（BuiltInStats）
  metrics export   导出为 Prometheus 文本格式

注意：全局指标是进程内的，每次 CLI 调用都是新进程，
因此 stats/export 看到的是本次进程内（含懒加载初始化）的状态。
要观察跨查询的指标，请在 serve 常驻模式下通过 HTTP 端点查看。`,
	}

	cmd.AddCommand(newMetricsStatsCmd())
	cmd.AddCommand(newMetricsExportCmd())

	return cmd
}

// newMetricsStatsCmd 显示全局内置指标。
func newMetricsStatsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "stats",
		Short: "显示全局内置指标（BuiltInStats）",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			m := whois.GetGlobalMetrics()
			stats := m.GetBuiltInStats()

			if asJSON {
				b, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}

			fmt.Fprintln(out, "WHOIS 全局指标（GetGlobalMetrics）:")
			fmt.Fprintf(out, "  总查询数: %d\n", stats.TotalQueries)
			fmt.Fprintf(out, "  成功查询: %d\n", stats.SuccessfulQueries)
			fmt.Fprintf(out, "  失败查询: %d\n", stats.FailedQueries)
			fmt.Fprintf(out, "  缓存命中: %d\n", stats.CacheHits)
			fmt.Fprintf(out, "  缓存未命中: %d\n", stats.CacheMisses)
			fmt.Fprintf(out, "  API 请求数: %d\n", stats.APIRequests)
			fmt.Fprintf(out, "  限流事件: %d\n", stats.RateLimitEvents)
			fmt.Fprintf(out, "  总查询耗时: %dms\n", stats.TotalQueryTimeMs)
			if stats.TotalQueries > 0 {
				avg := float64(stats.TotalQueryTimeMs) / float64(stats.TotalQueries)
				fmt.Fprintf(out, "  平均查询耗时: %.2fms\n", avg)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newMetricsExportCmd 导出为 Prometheus 文本格式。
// 基于 GetGlobalMetrics().GetBuiltInStats() 构造 exposition 文本。
func newMetricsExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "导出为 Prometheus 文本格式",
		Long:  `基于全局内置指标（BuiltInStats）导出 Prometheus exposition 文本格式，可直接被 Prometheus 抓取。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			m := whois.GetGlobalMetrics()
			s := m.GetBuiltInStats()

			// 构造 Prometheus exposition 文本
			type metric struct {
				name, help, mtype string
				value             int64
			}
			metrics := []metric{
				{"whois_queries_total", "Total WHOIS queries", "counter", s.TotalQueries},
				{"whois_queries_successful_total", "Successful WHOIS queries", "counter", s.SuccessfulQueries},
				{"whois_queries_failed_total", "Failed WHOIS queries", "counter", s.FailedQueries},
				{"whois_cache_hits_total", "Cache hits", "counter", s.CacheHits},
				{"whois_cache_misses_total", "Cache misses", "counter", s.CacheMisses},
				{"whois_api_requests_total", "API requests", "counter", s.APIRequests},
				{"whois_rate_limit_events_total", "Rate limit events", "counter", s.RateLimitEvents},
				{"whois_query_time_ms_total", "Total query time in ms", "counter", s.TotalQueryTimeMs},
			}
			for _, mt := range metrics {
				fmt.Fprintf(out, "# HELP %s %s\n", mt.name, mt.help)
				fmt.Fprintf(out, "# TYPE %s %s\n", mt.name, mt.mtype)
				fmt.Fprintf(out, "%s %d\n\n", mt.name, mt.value)
			}
			return nil
		},
	}
}
