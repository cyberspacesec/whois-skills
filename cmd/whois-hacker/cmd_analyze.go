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
用于识别同主体资产群。

直接调用 ` + "`correlation <domains>`" + ` 执行完整分析（同 analyze 子命令）。
子命令：
  correlation analyze <domains>     完整关联分析（图+聚类+画像）
  correlation profile <domains> --id <id> --type <t>   查指定实体资产画像
  correlation registrars <domains>  注册商维度统计`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, _, cancel := buildCorrelationEngine(args)
			defer cancel()
			result := engine.Analyze()
			return outputJSON(result)
		},
	}

	cmd.AddCommand(newCorrelationAnalyzeCmd())
	cmd.AddCommand(newCorrelationProfileCmd())
	cmd.AddCommand(newCorrelationRegistrarsCmd())
	return cmd
}

// buildCorrelationEngine 查询多域名并构建关联引擎。
func buildCorrelationEngine(domains []string) (*whois.CorrelationEngine, context.Context, context.CancelFunc) {
	engine := whois.NewCorrelationEngine()
	ctx, cancel := context.WithTimeout(context.Background(), durationOf(flagTimeout*len(domains)))
	for _, d := range domains {
		info, err := queryInfo(ctx, d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "查询 %s 失败: %v\n", d, err)
			continue
		}
		engine.AddDomain(d, info)
	}
	return engine, ctx, cancel
}

// newCorrelationAnalyzeCmd 完整关联分析。
func newCorrelationAnalyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze <domain1> [domain2...]",
		Short: "完整关联分析（图+聚类+画像）",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, _, cancel := buildCorrelationEngine(args)
			defer cancel()
			result := engine.Analyze()
			return outputJSON(result)
		},
	}
}

// newCorrelationProfileCmd 查指定实体的资产画像。
func newCorrelationProfileCmd() *cobra.Command {
	var (
		entityID   string
		entityType string
	)
	c := &cobra.Command{
		Use:   "profile <domain1> [domain2...]",
		Short: "查指定实体的资产画像（GetAssetProfile）",
		Long: `查询多域名构建关联引擎后，按实体 ID 与类型查看资产画像。

  --id <entityID>     实体 ID（邮箱/注册人/组织名）
  --type <type>       实体类型：email/registrant/organization

先跑 correlation analyze 查看聚类，拿到 entity ID 后用本子命令深入查看。`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if entityID == "" || entityType == "" {
				return fmt.Errorf("必须用 --id 和 --type 指定实体")
			}
			var ct whois.ClusterType
			switch entityType {
			case "email":
				ct = whois.ClusterByEmail
			case "registrant":
				ct = whois.ClusterByRegistrant
			case "organization", "org":
				ct = whois.ClusterByOrg
			default:
				return fmt.Errorf("无效类型 %s（可选 email/registrant/organization）", entityType)
			}
			engine, _, cancel := buildCorrelationEngine(args)
			defer cancel()
			profile := engine.GetAssetProfile(entityID, ct)
			if profile == nil {
				return fmt.Errorf("未找到实体: id=%s type=%s", entityID, entityType)
			}
			return outputJSON(profile)
		},
	}
	c.Flags().StringVar(&entityID, "id", "", "实体 ID（必填）")
	c.Flags().StringVar(&entityType, "type", "", "实体类型 email/registrant/organization（必填）")
	return c
}

// newCorrelationRegistrarsCmd 注册商维度统计。
func newCorrelationRegistrarsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "registrars <domain1> [domain2...]",
		Short: "注册商维度统计（GetRegistrarStats）",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, _, cancel := buildCorrelationEngine(args)
			defer cancel()
			stats := engine.GetRegistrarStats()
			return outputJSON(stats)
		},
	}
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

			// 收集结果（Process 启动后台 worker 后立即返回，结果通过 Results() 流式产出）
			go func() {
				if err := processor.Process(ctx, domains); err != nil {
					fmt.Fprintf(os.Stderr, "Process 错误: %v\n", err)
				}
			}()

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

	cmd.AddCommand(newBatchResumeCmd())
	return cmd
}

// newBatchResumeCmd 从断点文件恢复批量查询。
func newBatchResumeCmd() *cobra.Command {
	var (
		concurrency   int
		maxRetries    int
		queryDelay    int
		checkpoint    string
		checkInterval int
	)
	c := &cobra.Command{
		Use:   "resume --checkpoint <file>",
		Short: "从断点文件恢复批量查询",
		Long: `从断点文件（Checkpoint JSON）恢复未完成的批量查询，
只处理断点中尚未完成的域名。

断点文件由 ` + "`batch <file> --checkpoint <cp>`" + ` 在运行时产生，
当批量查询被中断（Ctrl+C / 崩溃）后，用本子命令续跑。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkpoint == "" {
				return fmt.Errorf("必须用 --checkpoint 指定断点文件路径")
			}

			// 先加载断点，取出已完成的与待处理的
			cp, err := whois.LoadCheckpointFromFile(checkpoint)
			if err != nil {
				return fmt.Errorf("加载断点失败: %w", err)
			}
			completed := len(cp.CompletedDomains)
			total := len(cp.AllDomains)
			fmt.Fprintf(os.Stderr, "断点: 总计 %d，已完成 %d，待处理 %d\n", total, completed, total-completed)
			if total-completed == 0 {
				fmt.Fprintln(os.Stderr, "全部已完成，无需恢复")
				return outputJSON(map[string]interface{}{
					"total":     total,
					"completed": completed,
					"results":   cp.Results,
					"resumed":   0,
				})
			}

			config := whois.DefaultStreamBatchConfig()
			config.Concurrency = concurrency
			config.Timeout = flagTimeout
			config.MaxRetries = maxRetries
			config.QueryDelay = queryDelay
			config.UseProxy = flagUseProxy
			config.CheckpointFile = checkpoint
			config.CheckpointInterval = checkInterval

			ctx := context.Background()
			processor, err := whois.ResumeFromCheckpoint(ctx, config)
			if err != nil {
				return fmt.Errorf("恢复失败: %w", err)
			}

			processor.OnProgress(func(stats whois.StreamBatchStats) {
				fmt.Fprintf(os.Stderr, "[%d/%d] 成功 %d 失败 %d 剩余 %v\n",
					stats.Completed, stats.TotalTasks,
					stats.SuccessCount, stats.FailureCount,
					stats.EstimatedRemaining)
			})

			var results []*whois.StreamBatchResult
			for r := range processor.Results() {
				results = append(results, r)
			}

			return outputJSON(map[string]interface{}{
				"total":     total,
				"completed": completed,
				"resumed":   len(results),
				"results":   results,
			})
		},
	}
	f := c.Flags()
	f.IntVar(&concurrency, "concurrency", 5, "并发数")
	f.IntVar(&maxRetries, "max-retries", 3, "最大重试次数")
	f.IntVar(&queryDelay, "query-delay", 200, "域间查询延迟（毫秒）")
	f.StringVar(&checkpoint, "checkpoint", "", "断点文件路径（必填）")
	f.IntVar(&checkInterval, "checkpoint-interval", 10, "每完成 N 个保存一次断点")
	return c
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
