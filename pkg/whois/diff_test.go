package whois

import (
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

func TestCompareWhois_BothNil(t *testing.T) {
	changes := CompareWhois(nil, nil)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes for both nil, got %d", len(changes))
	}
}

func TestCompareWhois_OldNil(t *testing.T) {
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain: "example.com",
		},
	}
	changes := CompareWhois(nil, newInfo)
	if len(changes) == 0 {
		t.Error("Expected changes when old is nil")
	}
	if changes[0].Type != ChangeAdded {
		t.Errorf("First change type = %s, want added", changes[0].Type)
	}
}

func TestCompareWhois_NewNil(t *testing.T) {
	oldInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain: "example.com",
		},
	}
	changes := CompareWhois(oldInfo, nil)
	if len(changes) == 0 {
		t.Error("Expected changes when new is nil")
	}
	if changes[0].Type != ChangeRemoved {
		t.Errorf("First change type = %s, want removed", changes[0].Type)
	}
}

func TestCompareWhois_NoChanges(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			UpdatedDate:    "2024-01-01",
			ExpirationDate: "2025-01-01",
		},
		Registrant: &whoisparser.Contact{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}
	changes := CompareWhois(info, info)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes for identical info, got %d", len(changes))
	}
}

func TestCompareWhois_DomainDateChange(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
	}
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2026-01-01",
		},
	}
	changes := CompareWhois(old, newInfo)
	if len(changes) == 0 {
		t.Error("Expected changes for different expiration dates")
	}

	found := false
	for _, c := range changes {
		if c.Field == "domain.expiration_date" && c.Type == ChangeModified {
			found = true
			if c.OldValue != "2025-01-01" {
				t.Errorf("OldValue = %v, want 2025-01-01", c.OldValue)
			}
			if c.NewValue != "2026-01-01" {
				t.Errorf("NewValue = %v, want 2026-01-01", c.NewValue)
			}
		}
	}
	if !found {
		t.Error("Expected modified change for domain.expiration_date")
	}
}

func TestCompareWhois_ContactChange(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}
	newInfo := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{
			Name:  "Jane Doe",
			Email: "jane@example.com",
		},
	}
	changes := CompareWhois(old, newInfo)
	if len(changes) == 0 {
		t.Error("Expected changes for different registrant")
	}
}

func TestCompareWhois_ContactAdded(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
	}
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
		Registrant: &whoisparser.Contact{
			Name: "John Doe",
		},
	}
	changes := CompareWhois(old, newInfo)
	found := false
	for _, c := range changes {
		if c.Type == ChangeAdded && c.Field == "registrant" {
			found = true
		}
	}
	if !found {
		t.Error("Expected added change for registrant")
	}
}

func TestCompareWhois_ContactRemoved(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
		Registrant: &whoisparser.Contact{
			Name: "John Doe",
		},
	}
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
	}
	changes := CompareWhois(old, newInfo)
	found := false
	for _, c := range changes {
		if c.Type == ChangeRemoved && c.Field == "registrant" {
			found = true
		}
	}
	if !found {
		t.Error("Expected removed change for registrant")
	}
}

func TestCompareWhois_StatusChange(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:  "example.com",
			Status:  []string{"clientTransferProhibited"},
		},
	}
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:  "example.com",
			Status:  []string{"clientTransferProhibited", "clientDeleteProhibited"},
		},
	}
	changes := CompareWhois(old, newInfo)
	found := false
	for _, c := range changes {
		if c.Field == "domain.status" && c.Type == ChangeAdded {
			found = true
		}
	}
	if !found {
		t.Error("Expected added change for new status")
	}
}

func TestCompareWhois_NameServerChange(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:       "example.com",
			NameServers:  []string{"ns1.old.com", "ns2.old.com"},
		},
	}
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:       "example.com",
			NameServers:  []string{"ns1.new.com", "ns2.new.com"},
		},
	}
	changes := CompareWhois(old, newInfo)
	if len(changes) == 0 {
		t.Error("Expected changes for different name servers")
	}
}

func TestCompareDomain_BothNil(t *testing.T) {
	changes := compareDomain(nil, nil)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(changes))
	}
}

func TestCompareDomain_OldNil(t *testing.T) {
	newDomain := &whoisparser.Domain{Domain: "example.com"}
	changes := compareDomain(nil, newDomain)
	if len(changes) == 0 {
		t.Error("Expected changes when old domain is nil")
	}
}

func TestCompareDomain_NewNil(t *testing.T) {
	oldDomain := &whoisparser.Domain{Domain: "example.com"}
	changes := compareDomain(oldDomain, nil)
	if len(changes) == 0 {
		t.Error("Expected changes when new domain is nil")
	}
}

func TestCompareStringSlices(t *testing.T) {
	tests := []struct {
		name        string
		old         []string
		new         []string
		wantAdded   int
		wantRemoved int
	}{
		{"both empty", []string{}, []string{}, 0, 0},
		{"added items", []string{"a"}, []string{"a", "b"}, 1, 0},
		{"removed items", []string{"a", "b"}, []string{"a"}, 0, 1},
		{"replaced items", []string{"a"}, []string{"b"}, 1, 1},
		{"case insensitive", []string{"A"}, []string{"a"}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := compareStringSlices("test", tt.old, tt.new)
			added := 0
			removed := 0
			for _, c := range changes {
				if c.Type == ChangeAdded {
					added++
				}
				if c.Type == ChangeRemoved {
					removed++
				}
			}
			if added != tt.wantAdded {
				t.Errorf("Added = %d, want %d", added, tt.wantAdded)
			}
			if removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", removed, tt.wantRemoved)
			}
		})
	}
}

func TestWhoisChange_Fields(t *testing.T) {
	change := &WhoisChange{
		Type:     ChangeModified,
		Field:    "domain.expiration_date",
		OldValue: "2025-01-01",
		NewValue: "2026-01-01",
		Path:     "domain.expiration_date",
	}

	if change.Type != ChangeModified {
		t.Errorf("Type = %s, want modified", change.Type)
	}
	if change.Field != "domain.expiration_date" {
		t.Errorf("Field = %s, want domain.expiration_date", change.Field)
	}
}

func TestChangeType_Constants(t *testing.T) {
	if ChangeAdded != "added" {
		t.Errorf("ChangeAdded = %s, want added", ChangeAdded)
	}
	if ChangeRemoved != "removed" {
		t.Errorf("ChangeRemoved = %s, want removed", ChangeRemoved)
	}
	if ChangeModified != "modified" {
		t.Errorf("ChangeModified = %s, want modified", ChangeModified)
	}
}

func TestCompareWhois_MultipleContacts(t *testing.T) {
	old := &whoisparser.WhoisInfo{
		Registrar: &whoisparser.Contact{Name: "RegA"},
		Registrant: &whoisparser.Contact{Name: "John"},
		Administrative: &whoisparser.Contact{Name: "Admin1"},
		Technical: &whoisparser.Contact{Name: "Tech1"},
		Billing: &whoisparser.Contact{Name: "Bill1"},
	}
	newInfo := &whoisparser.WhoisInfo{
		Registrar: &whoisparser.Contact{Name: "RegB"},
		Registrant: &whoisparser.Contact{Name: "John"},
		Administrative: &whoisparser.Contact{Name: "Admin2"},
		Technical: &whoisparser.Contact{Name: "Tech1"},
		Billing: &whoisparser.Contact{Name: "Bill1"},
	}
	changes := CompareWhois(old, newInfo)
	// Registrar and Administrative changed
	if len(changes) < 2 {
		t.Errorf("Expected at least 2 changes, got %d", len(changes))
	}
}

func BenchmarkCompareWhois(b *testing.B) {
	old := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.example.com", "ns2.example.com"},
		},
		Registrant: &whoisparser.Contact{Name: "John", Email: "john@example.com"},
	}
	newInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2026-01-01",
			Status:         []string{"clientTransferProhibited", "clientDeleteProhibited"},
			NameServers:    []string{"ns1.example.com", "ns3.example.com"},
		},
		Registrant: &whoisparser.Contact{Name: "Jane", Email: "jane@example.com"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CompareWhois(old, newInfo)
	}
}
