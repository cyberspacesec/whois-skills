package whois

import (
	"context"
	"strings"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

// mockReferralProvider 模拟 registry 返回 referral server。
type mockReferralProvider struct {
	registryRaw   string
	registrarRaw  string
	registryInfo  whoisparser.WhoisInfo
	registrarInfo whoisparser.WhoisInfo
	queryCount    int
}

func (m *mockReferralProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	m.queryCount++
	// 第一次查询返回 registry 结果，第二次返回 registrar
	if m.queryCount == 1 {
		return m.registryRaw, nil
	}
	return m.registrarRaw, nil
}

func (m *mockReferralProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	if strings.Contains(raw, "registry") {
		return m.registryInfo, nil
	}
	return m.registrarInfo, nil
}

// TestFollowReferralEnabled 验证启用 referral 时向 registrar 二次查询。
func TestFollowReferralEnabled(t *testing.T) {
	original := globalWhoisQueryProvider
	defer func() { globalWhoisQueryProvider = original }()

	mock := &mockReferralProvider{
		registryRaw: "registry response",
		registryInfo: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:      "example.com",
				WhoisServer: "whois.registrar.com", // referral server
			},
		},
		registrarRaw: "registrar response",
		registrarInfo: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:  "example.com",
				Status:  []string{"clientTransferProhibited"},
			},
			Registrant: &whoisparser.Contact{Name: "Test Owner"},
		},
	}
	SetWhoisQueryProvider(mock)

	result, err := ExecuteQueryWithResultContext(context.Background(), &QueryOptions{
		Domain:         "example.com",
		FollowReferral: true,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	// 应触发两次查询
	if mock.queryCount != 2 {
		t.Errorf("应触发 2 次查询（registry + registrar），得到 %d", mock.queryCount)
	}
	// registrar 结果应被合并
	if result.Info == nil || result.Info.Domain == nil || len(result.Info.Domain.Status) == 0 {
		t.Errorf("应合并 registrar 的 status，得到 %+v", result.Info)
	}
	if result.Info.Registrant == nil || result.Info.Registrant.Name != "Test Owner" {
		t.Errorf("应合并 registrar 的 registrant，得到 %+v", result.Info.Registrant)
	}
	// 原始响应应包含两部分
	if !strings.Contains(result.RawResponse, "registry") || !strings.Contains(result.RawResponse, "registrar") {
		t.Error("原始响应应包含 registry 和 registrar 两部分")
	}
}

// TestFollowReferralDisabled 验证禁用 referral 时仅查询 registry。
func TestFollowReferralDisabled(t *testing.T) {
	original := globalWhoisQueryProvider
	defer func() { globalWhoisQueryProvider = original }()

	mock := &mockReferralProvider{
		registryRaw: "registry response",
		registryInfo: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:      "example.com",
				WhoisServer: "whois.registrar.com",
			},
		},
	}
	SetWhoisQueryProvider(mock)

	_, err := ExecuteQueryWithResultContext(context.Background(), &QueryOptions{
		Domain:         "example.com",
		FollowReferral: false,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if mock.queryCount != 1 {
		t.Errorf("禁用 referral 应仅查询 1 次，得到 %d", mock.queryCount)
	}
}

// TestFollowReferralNoReferralServer 验证 registry 未返回 referral server 时跳过二次查询。
func TestFollowReferralNoReferralServer(t *testing.T) {
	original := globalWhoisQueryProvider
	defer func() { globalWhoisQueryProvider = original }()

	mock := &mockReferralProvider{
		registryRaw: "registry response",
		registryInfo: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:      "example.com",
				WhoisServer: "", // 无 referral
			},
		},
	}
	SetWhoisQueryProvider(mock)

	_, err := ExecuteQueryWithResultContext(context.Background(), &QueryOptions{
		Domain:         "example.com",
		FollowReferral: true,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if mock.queryCount != 1 {
		t.Errorf("无 referral server 时应仅查询 1 次，得到 %d", mock.queryCount)
	}
}

// TestMergeWhoisInfo 验证合并逻辑。
func TestMergeWhoisInfo(t *testing.T) {
	base := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:        "example.com",
			CreatedDate:   "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
	}
	override := &QueryResult{
		Info: &whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:      "example.com",
				Status:      []string{"clientTransferProhibited"},
				UpdatedDate: "2023-06-01",
			},
			Registrant: &whoisparser.Contact{Name: "Owner"},
		},
	}

	mergeWhoisInfo(base, override)

	if len(base.Domain.Status) != 1 {
		t.Errorf("应合并 status，得到 %v", base.Domain.Status)
	}
	if base.Domain.UpdatedDate != "2023-06-01" {
		t.Errorf("应合并 updatedDate，得到 %s", base.Domain.UpdatedDate)
	}
	if base.Registrant == nil || base.Registrant.Name != "Owner" {
		t.Errorf("应合并 registrant，得到 %+v", base.Registrant)
	}
	// 原有字段应保留
	if base.Domain.CreatedDate != "2020-01-01" {
		t.Errorf("原有 createdDate 应保留，得到 %s", base.Domain.CreatedDate)
	}
}