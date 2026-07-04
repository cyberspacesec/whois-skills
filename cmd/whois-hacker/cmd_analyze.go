package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	whoisparser "github.com/likexian/whois-parser"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// readDomainFile 从文本文件读取域名列表（每行一个，# 开头注释）。
func readDomainFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var domains []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, line)
	}
	return domains, scanner.Err()
}

// newDiffCmd 对比两份域名的 WHOIS 差异。
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <domain1> <domain2>",
		Short: "对比两个域名的 WHOIS 字段差异",
		Long:  `分别查询两个域名，逐字段对比 WHOIS 信息的差异（新增/删除/修改）。`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout*2))
			defer cancel()

			old, err := queryInfo(ctx, args[0])
			if err != nil {
				return fmt.Errorf("查询 %s 失败: %w", args[0], err)
			}
			newInfo, err := queryInfo(ctx, args[1])
			if err != nil {
				return fmt.Errorf("查询 %s 失败: %w", args[1], err)
			}

			changes := whois.CompareWhois(old, newInfo)
			return outputJSON(map[string]interface{}{
				"domain_old": args[0],
				"domain_new": args[1],
				"changes":    changes,
				"total":      len(changes),
			})
		},
	}
	return cmd
}

// newQualityCmd 评估域名 WHOIS 数据质量。
func newQualityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quality <domain>",
		Short: "评估域名 WHOIS 数据质量（三维评分）",
		Long:  `从完整性、时效性、可信度三个维度评估 WHOIS 数据质量，输出 0-100 评分。`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout))
			defer cancel()
			info, err := queryInfo(ctx, args[0])
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}
			score := whois.AssessQuality(info)
			return outputJSON(score)
		},
	}
	return cmd
}

// newCorrelationCmd 多域名关联分析。
func newCorrelationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "correlation <domain1> [domain2...]",
		Short: "多域名关联分析（资产画像）",
		Long: `按邮箱/注册人/组织/NS/注册商五维聚类，构建关联图与资产画像，
用于识别同主体资产群。`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := whois.NewCorrelationEngine()
			ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout*len(args)))
			defer cancel()

			for _, d := range args {
				info, err := queryInfo(ctx, d)
				if err != nil {
					fmt.Fprintf(os.Stderr, "查询 %s 失败: %v\n", d, err)
					continue
				}
				engine.AddDomain(d, info)
			}
			result := engine.Analyze()
			return outputJSON(result)
		},
	}
	return cmd
}

// newBatchCmd 批量查询。
func newBatchCmd() *cobra.Command {
	var (
		concurrency   int
		maxRetries    int
		queryDelay    int
		checkpoint    string
		checkInterval int
	)
	cmd := &cobra.Command{
		Use:   "batch <file>",
		Short: "从文件批量查询域名",
		Long: `从文本文件读取域名列表（每行一个），流式批量查询，支持并发、
限速、断点续查与进度输出。

文件格式：每行一个域名，# 开头为注释。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domains, err := readDomainFile(args[0])
			if err != nil {
				return fmt.Errorf("读取域名文件失败: %w", err)
			}
			if len(domains) == 0 {
				return fmt.Errorf("域名列表为空")
			}

			config := whois.DefaultStreamBatchConfig()
			config.Concurrency = concurrency
			config.Timeout = flagTimeout
			config.MaxRetries = maxRetries
			config.QueryDelay = queryDelay
			config.UseProxy = flagUseProxy
			if checkpoint != "" {
				config.CheckpointFile = checkpoint
				config.CheckpointInterval = checkInterval
			}

			ctx := context.Background()
			processor := whois.NewStreamBatchProcessor(config)

			// 进度回调输出到 stderr
			processor.OnProgress(func(stats whois.StreamBatchStats) {
				fmt.Fprintf(os.Stderr, "[%d/%d] 成功 %d 失败 %d 剩余 %v\n",
					stats.Completed, stats.TotalTasks,
					stats.SuccessCount, stats.FailureCount,
					stats.EstimatedRemaining)
			})

			// 收集结果
			go processor.Process(ctx, domains)

			var results []*whois.StreamBatchResult
			for r := range processor.Results() {
				results = append(results, r)
			}

			return outputJSON(map[string]interface{}{
				"total":   len(domains),
				"results": results,
			})
		},
	}
	f := cmd.Flags()
	f.IntVar(&concurrency, "concurrency", 5, "并发数")
	f.IntVar(&maxRetries, "max-retries", 3, "最大重试次数")
	f.IntVar(&queryDelay, "query-delay", 200, "域间查询延迟（毫秒）")
	f.StringVar(&checkpoint, "checkpoint", "", "断点续查文件路径")
	f.IntVar(&checkInterval, "checkpoint-interval", 10, "每完成 N 个保存一次断点")
	return cmd
}

// queryInfo 查询域名的结构化 WhoisInfo（供 diff/quality/correlation 复用）。
func queryInfo(ctx context.Context, domain string) (*whoisparser.WhoisInfo, error) {
	res, err := whois.ExecuteQueryWithResultContext(ctx, &whois.QueryOptions{
		Domain:   domain,
		Timeout:  flagTimeout,
		UseProxy: flagUseProxy,
	})
	if err != nil {
		return nil, err
	}
	if res == nil || res.Info == nil {
		return nil, fmt.Errorf("查询 %s 未返回结构化信息", domain)
	}
	return res.Info, nil
}
