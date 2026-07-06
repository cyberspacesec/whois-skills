package whois

import (
	"strings"
	"testing"
)

// TestDefaultWhoisTextCleanerStripComments 验证去注释行。
func TestDefaultWhoisTextCleanerStripComments(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
	raw := `# This is a comment
% Another comment
Domain Name: example.com
;; yet another
Registrant: Alice`
	cleaned := c.Clean(raw)
	if strings.Contains(cleaned, "# This is a comment") {
		t.Error("未去除 # 注释行")
	}
	if strings.Contains(cleaned, "% Another comment") {
		t.Error("未去除 % 注释行")
	}
	if strings.Contains(cleaned, ";; yet another") {
		t.Error("未去除 ;; 注释行")
	}
	if !strings.Contains(cleaned, "domain name: example.com") {
		t.Error("误删了有效行")
	}
}

// TestDefaultWhoisTextCleanerFoldBlankLines 验证折叠空行。
func TestDefaultWhoisTextCleanerFoldBlankLines(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
	raw := "line1\n\n\n\nline2\n\n\nline3"
	cleaned := c.Clean(raw)
	// 不应出现连续两个空行
	if strings.Contains(cleaned, "\n\n\n") {
		t.Errorf("未折叠连续空行: %q", cleaned)
	}
}

// TestDefaultWhoisTextCleanerFoldInlineWhitespace 验证行内空白折叠。
func TestDefaultWhoisTextCleanerFoldInlineWhitespace(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
	raw := "Domain Name:    example.com"
	cleaned := c.Clean(raw)
	if strings.Contains(cleaned, "    ") {
		t.Errorf("未折叠行内多空白: %q", cleaned)
	}
}

// TestDefaultWhoisTextCleanerNormalizeFieldNames 验证字段名归一化。
func TestDefaultWhoisTextCleanerNormalizeFieldNames(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
	raw := "Domain Name: example.com"
	cleaned := c.Clean(raw)
	// 字段名应小写
	if !strings.HasPrefix(cleaned, "domain name:") {
		t.Errorf("字段名未归一化为小写: %q", cleaned)
	}
}

// TestDefaultWhoisTextCleanerTrimTrailingBlanks 验证去尾部空行。
func TestDefaultWhoisTextCleanerTrimTrailingBlanks(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
	raw := "line1\n\n\n"
	cleaned := c.Clean(raw)
	if strings.HasSuffix(cleaned, "\n") {
		t.Errorf("未去除尾部空行: %q", cleaned)
	}
}

// TestDefaultWhoisTextCleanerEmpty 验证空输入。
func TestDefaultWhoisTextCleanerEmpty(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig())
	if c.Clean("") != "" {
		t.Error("空输入应返回空")
	}
}

// TestDefaultWhoisTextCleanerCustomConfig 验证自定义配置（关闭去注释）。
func TestDefaultWhoisTextCleanerCustomConfig(t *testing.T) {
	c := NewDefaultWhoisTextCleaner(WhoisCleanConfig{
		StripComments:        false,
		FoldBlankLines:       true,
		TrimLines:            true,
		FoldInlineWhitespace: true,
		NormalizeFieldNames:  false,
		TrimTrailingBlanks:   true,
	})
	raw := "# keep me\nDomain Name: EXAMPLE.COM"
	cleaned := c.Clean(raw)
	if !strings.Contains(cleaned, "# keep me") {
		t.Error("关闭去注释后应保留注释行")
	}
	// 字段名不归一化，应保留原大小写
	if !strings.Contains(cleaned, "Domain Name:") {
		t.Error("关闭归一化后应保留原字段名大小写")
	}
}

// TestWhoisTextCleanerGlobalInjection 验证全局注入与懒加载。
func TestWhoisTextCleanerGlobalInjection(t *testing.T) {
	// 保存原值
	original := globalWhoisTextCleaner
	defer func() { globalWhoisTextCleaner = original }()
	// 注意：sync.Once 已触发，无法重置；仅验证注入生效
	custom := NewDefaultWhoisTextCleaner(WhoisCleanConfig{})
	SetWhoisTextCleaner(custom)
	if GetWhoisTextCleaner() != custom {
		t.Error("注入后应返回注入实例")
	}
}

// TestCleanWhoisText 验证便捷函数。
func TestCleanWhoisText(t *testing.T) {
	original := globalWhoisTextCleaner
	defer func() { globalWhoisTextCleaner = original }()
	SetWhoisTextCleaner(NewDefaultWhoisTextCleaner(DefaultWhoisCleanConfig()))

	raw := "# comment\nDomain Name:    example.com"
	cleaned := CleanWhoisText(raw)
	if strings.Contains(cleaned, "# comment") {
		t.Error("便捷函数应清洗注释")
	}
}