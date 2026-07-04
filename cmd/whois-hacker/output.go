package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
)

// outputJSON 将任意结构以 JSON 形式输出到 stdout。
func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// outputRaw 输出原始文本。
func outputRaw(s string) {
	fmt.Print(s)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		fmt.Println()
	}
}

// outputResult 按 --format 选择输出方式。
//   - json（默认）：结构化 JSON
//   - raw：原始文本（需调用方提供 rawText）
//   - text：人类可读摘要（需调用方提供 textFn）
func outputResult(v interface{}, rawText string, textFn func() string) error {
	switch flagOutput {
	case "raw":
		outputRaw(rawText)
		return nil
	case "text":
		if textFn != nil {
			fmt.Println(textFn())
		}
		return nil
	default: // json
		return outputJSON(v)
	}
}

// errExit 打印错误并退出（供子命令使用）。
func errExit(cmd *cobra.Command, msg string) {
	if flagLogFormat == "json" {
		_ = outputJSON(map[string]string{"error": msg})
	} else {
		logrus.Errorf("%s", msg)
	}
	_ = cmd
	os.Exit(1)
}
