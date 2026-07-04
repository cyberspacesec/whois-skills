package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd 显示版本信息。
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印 whois-hacker 版本信息",
		Long:  `打印 whois-hacker 的版本号、Git commit 与构建时间。`,
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "whois-hacker %s\n", Version)
			fmt.Fprintf(out, "  commit: %s\n", GitCommit)
			fmt.Fprintf(out, "  built:  %s\n", BuildTime)
		},
	}
}
