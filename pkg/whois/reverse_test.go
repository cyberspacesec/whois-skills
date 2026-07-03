package whois

import (
	"context"
	"testing"
)

// mockReverseWhoisProvider 模拟反向WHOIS查询提供者
type mockReverseWhoisProvider struct {
	name    string
	results []*ReverseWhoisResult
	err     error
}

func (m *mockReverseWhoisProvider) SearchByRegistrant(ctx context.Context, query string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return m.results, m.err
}

func (m *mockReverseWhoisProvider) SearchByEmail(ctx context.Context, email string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return m.results, m.err
}

func (m *mockReverseWhoisProvider) SearchByOrganization(ctx context.Context, org string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return m.results, m.err
}

func (m *mockReverseWhoisProvider) Name() string {
	return m.name
}

func TestNewReverseWhoisClient(t *testing.T) {
	provider := &mockReverseWhoisProvider{name: "test"}
	client := NewReverseWhoisClient(provider)
	if client == nil {
		t.Fatal("NewReverseWhoisClient() returned nil")
	}
}

func TestReverseWhoisClient_ProviderName(t *testing.T) {
	provider := &mockReverseWhoisProvider{name: "test-provider"}
	client := NewReverseWhoisClient(provider)
	if client.ProviderName() != "test-provider" {
		t.Errorf("ProviderName() = %s, want test-provider", client.ProviderName())
	}
}

func TestReverseWhoisClient_ProviderName_NilProvider(t *testing.T) {
	client := NewReverseWhoisClient(nil)
	if client.ProviderName() != "none" {
		t.Errorf("ProviderName() = %s, want none", client.ProviderName())
	}
}

func TestReverseWhoisClient_SearchByRegistrant(t *testing.T) {
	results := []*ReverseWhoisResult{
		{Domain: "example.com", Registrant: "John Doe"},
		{Domain: "example.org", Registrant: "John Doe"},
	}
	provider := &mockReverseWhoisProvider{name: "test", results: results}
	client := NewReverseWhoisClient(provider)

	got, err := client.SearchByRegistrant(context.Background(), "John Doe", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Results count = %d, want 2", len(got))
	}
}

func TestReverseWhoisClient_SearchByEmail(t *testing.T) {
	results := []*ReverseWhoisResult{
		{Domain: "example.com", Email: "john@example.com"},
	}
	provider := &mockReverseWhoisProvider{name: "test", results: results}
	client := NewReverseWhoisClient(provider)

	got, err := client.SearchByEmail(context.Background(), "john@example.com", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Results count = %d, want 1", len(got))
	}
	if got[0].Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", got[0].Domain)
	}
}

func TestReverseWhoisClient_SearchByOrganization(t *testing.T) {
	results := []*ReverseWhoisResult{
		{Domain: "example.com", Organization: "Example Corp"},
	}
	provider := &mockReverseWhoisProvider{name: "test", results: results}
	client := NewReverseWhoisClient(provider)

	got, err := client.SearchByOrganization(context.Background(), "Example Corp", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Results count = %d, want 1", len(got))
	}
}

func TestReverseWhoisResult_Fields(t *testing.T) {
	result := &ReverseWhoisResult{
		Domain:         "example.com",
		Registrant:     "John Doe",
		Email:          "john@example.com",
		Organization:   "Example Corp",
		CreationDate:   "2020-01-01",
		ExpirationDate: "2025-01-01",
		Registrar:      "Example Registrar",
	}

	if result.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", result.Domain)
	}
	if result.Registrant != "John Doe" {
		t.Errorf("Registrant = %s, want John Doe", result.Registrant)
	}
	if result.Email != "john@example.com" {
		t.Errorf("Email = %s, want john@example.com", result.Email)
	}
	if result.Organization != "Example Corp" {
		t.Errorf("Organization = %s, want Example Corp", result.Organization)
	}
	if result.Registrar != "Example Registrar" {
		t.Errorf("Registrar = %s, want Example Registrar", result.Registrar)
	}
}

func TestReverseWhoisOptions_Fields(t *testing.T) {
	opts := &ReverseWhoisOptions{
		Limit:         50,
		IncludeExpired: true,
	}

	if opts.Limit != 50 {
		t.Errorf("Limit = %d, want 50", opts.Limit)
	}
	if !opts.IncludeExpired {
		t.Error("IncludeExpired should be true")
	}
}

func TestReverseWhoisProvider_Interface(t *testing.T) {
	// Verify mock implements the interface
	var _ ReverseWhoisProvider = &mockReverseWhoisProvider{}
}

func TestReverseWhoisClient_SearchByRegistrant_EmptyResults(t *testing.T) {
	provider := &mockReverseWhoisProvider{name: "test", results: []*ReverseWhoisResult{}}
	client := NewReverseWhoisClient(provider)

	got, err := client.SearchByRegistrant(context.Background(), "nobody", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Results count = %d, want 0", len(got))
	}
}
