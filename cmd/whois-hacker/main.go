// Package main 是 whois-hacker 命令行工具的入口。
//
// whois-hacker 是一个基于 cobra 的 CLI，既能启动 HTTP 服务（serve），
// 也能直接调用 SDK 的全部能力（whois/ip/asn/rdap/availability/diff/
// quality/correlation/batch/idn/format/export/servers 等）。
package main

import "os"

func main() {
	rootCmd = newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		// cobra 已自行打印错误信息，此处仅以非零码退出
		os.Exit(1)
	}
}
