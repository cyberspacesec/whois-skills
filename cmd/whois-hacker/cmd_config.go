package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// newConfigCmd 库配置（WhoisLibraryConfig）管理子命令组。
// 与全局 --config（加载 AppConfig YAML）正交：这里管理的是库级运行时配置。
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "库配置（WhoisLibraryConfig）管理",
		Long: `管理 whois 库级运行时配置（WhoisLibraryConfig），覆盖查询/缓存/代理/限速/
批量/监控/调度/可观测/日志九大子系统。

与全局 --config flag（加载 AppConfig 应用配置）正交：本子命令操作的是库配置，
可加载/保存/校验/合并/应用，并以可读摘要或 JSON 形式查看。

配置文件格式为 JSON（WhoisLibraryConfig 的序列化形式）。`,
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigValidateCmd())
	cmd.AddCommand(newConfigSaveCmd())
	cmd.AddCommand(newConfigMergeCmd())
	cmd.AddCommand(newConfigApplyCmd())

	return cmd
}

// newConfigShowCmd 显示库配置：默认值、当前全局值或指定文件。
func newConfigShowCmd() *cobra.Command {
	var (
		fromFile string
		defaul   bool
		summary  bool
		asJSON   bool
	)
	c := &cobra.Command{
		Use:   "show",
		Short: "显示库配置（默认值 / 当前全局 / 指定文件）",
		Long: `显示 WhoisLibraryConfig。

  --default         显示默认库配置
  --file <path>     显示指定文件中的库配置
  （都不给）        显示当前全局库配置（GetWhoisLibraryConfig）

默认输出可读摘要，--json 输出结构化 JSON。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			var cfg *whois.WhoisLibraryConfig
			switch {
			case defaul:
				d := whois.DefaultWhoisLibraryConfig()
				cfg = &d
			case fromFile != "":
				cfg = whois.LoadWhoisLibraryConfigFromFile(fromFile)
				if cfg == nil {
					return fmt.Errorf("无法从 %s 加载库配置（文件不存在或解析失败）", fromFile)
				}
			default:
				cfg = whois.GetWhoisLibraryConfig()
			}

			if summary || !asJSON {
				fmt.Fprintln(out, whois.WhoisLibraryConfigSummary(cfg))
				if asJSON {
					// 同时输出摘要与 JSON
				}
			}
			if asJSON {
				b, err := json.MarshalIndent(cfg, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			}
			return nil
		},
	}
	c.Flags().StringVar(&fromFile, "file", "", "从指定文件加载库配置")
	c.Flags().BoolVar(&defaul, "default", false, "显示默认库配置")
	c.Flags().BoolVar(&summary, "summary", false, "输出可读摘要（默认行为，与 --json 同用时两者都输出）")
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON")
	return c
}

// newConfigValidateCmd 校验库配置文件。
func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file>",
		Short: "校验库配置文件",
		Long:  `校验 WhoisLibraryConfig JSON 文件是否合法。退出码 0 表示合法，非 0 表示有错误。`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg := whois.LoadWhoisLibraryConfigFromFile(args[0])
			if cfg == nil {
				return fmt.Errorf("无法从 %s 加载库配置", args[0])
			}
			if err := whois.ValidateWhoisLibraryConfig(cfg); err != nil {
				fmt.Fprintf(out, "❌ 配置无效: %v\n", err)
				return err // 非 0 退出码
			}
			fmt.Fprintf(out, "✅ 配置合法: %s\n", args[0])
			return nil
		},
	}
}

// newConfigSaveCmd 把默认或当前全局库配置保存到文件。
func newConfigSaveCmd() *cobra.Command {
	var defaul bool
	c := &cobra.Command{
		Use:   "save <file>",
		Short: "保存库配置到文件",
		Long: `把库配置保存为 JSON 文件。

  --default  保存默认库配置
  （不给）   保存当前全局库配置`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			var cfg *whois.WhoisLibraryConfig
			if defaul {
				d := whois.DefaultWhoisLibraryConfig()
				cfg = &d
			} else {
				cfg = whois.GetWhoisLibraryConfig()
			}
			if err := whois.SaveWhoisLibraryConfigToFile(cfg, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(out, "✅ 已保存库配置到 %s\n", args[0])
			return nil
		},
	}
	c.Flags().BoolVar(&defaul, "default", false, "保存默认库配置")
	return c
}

// newConfigMergeCmd 合并多份库配置（base + overrides）。
func newConfigMergeCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "merge <base> <override>...",
		Short: "合并多份库配置文件",
		Long: `以第一份为 base，依次合并后续文件作为 override，输出合并结果。

合并策略：override 中非零值覆盖 base 对应字段。`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			base := whois.LoadWhoisLibraryConfigFromFile(args[0])
			if base == nil {
				return fmt.Errorf("无法加载 base 配置: %s", args[0])
			}
			overrides := make([]*whois.WhoisLibraryConfig, 0, len(args)-1)
			for _, f := range args[1:] {
				o := whois.LoadWhoisLibraryConfigFromFile(f)
				if o == nil {
					return fmt.Errorf("无法加载 override 配置: %s", f)
				}
				overrides = append(overrides, o)
			}
			merged := whois.MergeWhoisLibraryConfigs(base, overrides...)
			if asJSON {
				b, err := json.MarshalIndent(merged, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else {
				fmt.Fprintln(out, whois.WhoisLibraryConfigSummary(merged))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "输出 JSON（默认输出可读摘要）")
	return c
}

// newConfigApplyCmd 加载并应用库配置到全局（影响后续查询行为）。
func newConfigApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply <file>",
		Short: "加载并应用库配置到全局",
		Long: `加载 WhoisLibraryConfig 文件并应用到全局，影响后续查询的默认行为
（缓存/代理/限速/重试等）。

适用于在脚本中先 apply 再查询的场景。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg := whois.LoadWhoisLibraryConfigFromFile(args[0])
			if cfg == nil {
				return fmt.Errorf("无法从 %s 加载库配置", args[0])
			}
			if err := whois.ValidateWhoisLibraryConfig(cfg); err != nil {
				return fmt.Errorf("配置校验失败: %w", err)
			}
			if err := whois.ApplyWhoisLibraryConfig(cfg); err != nil {
				return fmt.Errorf("应用配置失败: %w", err)
			}
			fmt.Fprintf(out, "✅ 已应用库配置: %s\n", args[0])
			return nil
		},
	}
}
