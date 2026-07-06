package whois

import (
	"strings"
	"sync"
)

// ============================================================================
// WHOIS 文本清洗/规范化
//
// 上层在做大规模对比、入库、反向索引前，常需要先把原始 WHOIS 文本清洗为
// 统一形式：去注释、去冗余空行、折叠空白、字段名归一化。WhoisTextCleaner
// 接口抽象这一能力，内置默认实现，上层可注入自定义清洗器。
// ============================================================================

// WhoisTextCleaner WHOIS 文本清洗器接口。
type WhoisTextCleaner interface {
	// Clean 清洗原始 WHOIS 文本，返回规范化后的文本。
	Clean(raw string) string
}

// WhoisCleanConfig 清洗配置。
type WhoisCleanConfig struct {
	// 去除注释行（# / % / ;;）
	StripComments bool
	// 折叠多余空行为单个空行
	FoldBlankLines bool
	// 去除每行首尾空白
	TrimLines bool
	// 折叠行内多空白为单空格
	FoldInlineWhitespace bool
	// 字段名归一化（小写 + 去冒号前后空白）
	NormalizeFieldNames bool
	// 去除尾部空行
	TrimTrailingBlanks bool
}

// DefaultWhoisCleanConfig 默认清洗配置（全开）。
func DefaultWhoisCleanConfig() WhoisCleanConfig {
	return WhoisCleanConfig{
		StripComments:        true,
		FoldBlankLines:       true,
		TrimLines:            true,
		FoldInlineWhitespace: true,
		NormalizeFieldNames:  true,
		TrimTrailingBlanks:   true,
	}
}

// DefaultWhoisTextCleaner 默认 WHOIS 文本清洗器。
type DefaultWhoisTextCleaner struct {
	config WhoisCleanConfig
}

// NewDefaultWhoisTextCleaner 创建默认清洗器。
func NewDefaultWhoisTextCleaner(config WhoisCleanConfig) *DefaultWhoisTextCleaner {
	return &DefaultWhoisTextCleaner{config: config}
}

// Clean 清洗原始文本。
func (c *DefaultWhoisTextCleaner) Clean(raw string) string {
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		// 去除首尾空白（含 \r）
		if c.config.TrimLines {
			line = strings.TrimRight(strings.TrimSpace(line), "\r")
			line = strings.TrimSpace(line)
		}

		// 注释行
		if c.config.StripComments {
			if isCommentLine(line) {
				continue
			}
		}

		// 空行处理
		if line == "" {
			if c.config.FoldBlankLines {
				if len(result) > 0 && result[len(result)-1] != "" {
					result = append(result, "")
				}
				continue
			}
		}

		// 行内空白折叠
		if c.config.FoldInlineWhitespace {
			line = foldInlineWhitespace(line)
		}

		// 字段名归一化
		if c.config.NormalizeFieldNames {
			line = normalizeFieldName(line)
		}

		result = append(result, line)
	}

	// 去除尾部空行
	if c.config.TrimTrailingBlanks {
		for len(result) > 0 && result[len(result)-1] == "" {
			result = result[:len(result)-1]
		}
	}

	return strings.Join(result, "\n")
}

// isCommentLine 判断是否为注释行。
func isCommentLine(line string) bool {
	if line == "" {
		return false
	}
	return strings.HasPrefix(line, "#") ||
		strings.HasPrefix(line, "%") ||
		strings.HasPrefix(line, "//") ||
		strings.HasPrefix(line, ";;")
}

// foldInlineWhitespace 折叠行内多空白为单空格。
func foldInlineWhitespace(line string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range line {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

// normalizeFieldName 归一化字段名：把 "Domain Name :" → "domain name:"
// 仅对含冒号且冒号前无空格的"字段: 值"行做小写归一化（保守，避免破坏值）。
func normalizeFieldName(line string) string {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return line
	}
	field := line[:idx]
	// 字段名不应含空格以外的特殊字符；若含数字/字母则归一化
	if !isFieldNameLike(field) {
		return line
	}
	return strings.ToLower(strings.TrimSpace(field)) + ":" + line[idx+1:]
}

// isFieldNameLike 判断字符串是否像字段名（字母/数字/空格/下划线/连字符）。
func isFieldNameLike(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != ' ' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// ---- 全局清洗器 ----

var (
	globalWhoisTextCleaner     WhoisTextCleaner
	globalWhoisTextCleanerOnce sync.Once
)

// GetWhoisTextCleaner 返回全局 WHOIS 文本清洗器（懒加载默认实现）。
func GetWhoisTextCleaner() WhoisTextCleaner {
	globalWhoisTextCleanerOnce.Do(func() {
		if globalWhoisTextCleaner == nil {
			globalWhoisTextCleaner = NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
		}
	})
	return globalWhoisTextCleaner
}

// SetWhoisTextCleaner 注入自定义 WHOIS 文本清洗器。
func SetWhoisTextCleaner(c WhoisTextCleaner) {
	globalWhoisTextCleaner = c
}

// CleanWhoisText 清洗 WHOIS 文本（走全局清洗器）。
func CleanWhoisText(raw string) string {
	c := GetWhoisTextCleaner()
	if c == nil {
		return raw
	}
	return c.Clean(raw)
}