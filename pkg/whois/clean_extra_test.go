package whois

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- normalizeFieldName 分支 ----

func TestNormalizeFieldName_NoColon(t *testing.T) {
	// idx <= 0 → 原样返回
	assert.Equal(t, "no colon here", normalizeFieldName("no colon here"))
}

func TestNormalizeFieldName_ColonFirst(t *testing.T) {
	// idx == 0 → 原样返回
	assert.Equal(t, ": value", normalizeFieldName(": value"))
}

func TestNormalizeFieldName_NotFieldLike(t *testing.T) {
	// 字段名含特殊字符（!）→ isFieldNameLike false → 原样返回
	assert.Equal(t, "Dom@in: example.com", normalizeFieldName("Dom@in: example.com"))
}

func TestNormalizeFieldName_Normalizes(t *testing.T) {
	// 正常归一化
	got := normalizeFieldName("Domain Name : example.com")
	assert.Equal(t, "domain name: example.com", got)
}

func TestNormalizeFieldName_TrimsField(t *testing.T) {
	got := normalizeFieldName("  Domain  : example.com")
	assert.Equal(t, "domain: example.com", got)
}

// ---- isFieldNameLike 分支 ----

func TestIsFieldNameLike_Empty(t *testing.T) {
	assert.False(t, isFieldNameLike(""))
	assert.False(t, isFieldNameLike("   "))
}

func TestIsFieldNameLike_SpecialChar(t *testing.T) {
	// 含 @ 等特殊字符 → false
	assert.False(t, isFieldNameLike("ab@cd"))
	assert.False(t, isFieldNameLike("a.b"))
}

func TestIsFieldNameLike_Valid(t *testing.T) {
	assert.True(t, isFieldNameLike("Domain Name"))
	assert.True(t, isFieldNameLike("Admin_ID"))
	assert.True(t, isFieldNameLike("net-name"))
	assert.True(t, isFieldNameLike("123abc"))
}

// ---- GetWhoisTextCleaner / CleanWhoisText ----

func TestCleanWhoisText_NilCleaner(t *testing.T) {
	// 先确保 once 已触发（调用一次 GetWhoisTextCleaner 触发懒加载）
	_ = GetWhoisTextCleaner()
	// once 已触发，设 globalWhoisTextCleaner=nil 后 GetWhoisTextCleaner 返回 nil
	// → CleanWhoisText 走 c==nil 分支，原样返回
	original := globalWhoisTextCleaner
	defer func() { globalWhoisTextCleaner = original }()
	globalWhoisTextCleaner = nil
	raw := "unchanged text"
	assert.Equal(t, raw, CleanWhoisText(raw))
}
