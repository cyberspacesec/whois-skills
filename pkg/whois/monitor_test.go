package whois

import (
	"context"
	"testing"
	"time"

	whoisparser "github.com/likexian/whois-parser"
)

func TestNewDomainMonitor(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewDomainMonitor(config)

	if monitor == nil {
		t.Fatal("NewDomainMonitor() returned nil")
	}
	if monitor.config.CheckInterval != 60 {
		t.Errorf("CheckInterval = %d, want 60", monitor.config.CheckInterval)
	}
}

func TestNewDomainMonitor_Defaults(t *testing.T) {
	config := MonitorConfig{}
	monitor := NewDomainMonitor(config)

	if monitor.config.CheckInterval != 60 {
		t.Errorf("Default CheckInterval = %d, want 60", monitor.config.CheckInterval)
	}
	if monitor.config.ExpiryWarningDays != 30 {
		t.Errorf("Default ExpiryWarningDays = %d, want 30", monitor.config.ExpiryWarningDays)
	}
	if monitor.config.ExpiryCriticalDays != 7 {
		t.Errorf("Default ExpiryCriticalDays = %d, want 7", monitor.config.ExpiryCriticalDays)
	}
}

func TestDomainMonitor_AddWatch(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			ExpirationDate: "2027-01-01",
		},
	}

	monitor.AddWatch("example.com", info)

	watchList := monitor.GetWatchList()
	if len(watchList) != 1 {
		t.Errorf("WatchList count = %d, want 1", len(watchList))
	}

	state := monitor.GetWatchState("example.com")
	if state == nil {
		t.Fatal("GetWatchState() returned nil")
	}
	if state.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", state.Domain)
	}
	if state.ExpirationDate != "2027-01-01" {
		t.Errorf("ExpirationDate = %s, want 2027-01-01", state.ExpirationDate)
	}
}

func TestDomainMonitor_AddWatch_NilInfo(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	monitor.AddWatch("example.com", nil)

	state := monitor.GetWatchState("example.com")
	if state == nil {
		t.Fatal("GetWatchState() returned nil")
	}
	if state.Status != WatchStatusActive {
		t.Errorf("Status = %s, want active", state.Status)
	}
}

func TestDomainMonitor_RemoveWatch(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	monitor.AddWatch("example.com", nil)
	monitor.RemoveWatch("example.com")

	state := monitor.GetWatchState("example.com")
	if state != nil {
		t.Error("WatchState should be nil after removal")
	}
}

func TestDomainMonitor_GetWatchState_NotFound(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	state := monitor.GetWatchState("nonexistent.com")
	if state != nil {
		t.Error("Expected nil for non-existent domain")
	}
}

func TestDomainMonitor_Alerts(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	ch := monitor.Alerts()
	if ch == nil {
		t.Error("Alerts channel should not be nil")
	}
}

func TestDomainMonitor_OnAlert(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	var receivedAlert *DomainAlert
	monitor.OnAlert(func(alert *DomainAlert) {
		receivedAlert = alert
	})

	if monitor.alertCallback == nil {
		t.Error("Alert callback should be set")
	}

	// Manually trigger an alert
	testAlert := &DomainAlert{
		Domain:    "example.com",
		Type:      AlertExpiryWarning,
		Level:     AlertLevelWarning,
		Message:   "Test alert",
		Timestamp: time.Now(),
	}
	monitor.alertCallback(testAlert)

	if receivedAlert == nil {
		t.Error("Alert callback should have been called")
	}
	if receivedAlert.Domain != "example.com" {
		t.Errorf("Alert domain = %s, want example.com", receivedAlert.Domain)
	}
}

func TestDomainMonitor_CheckNow_NotInWatchlist(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	err := monitor.CheckNow(context.Background(), "nonexistent.com")
	if err == nil {
		t.Error("Expected error for non-existent domain in watchlist")
	}
}

func TestDomainMonitor_Stop(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	// Stop should not panic even without start
	monitor.Stop()
}

func TestCalculateDaysRemaining(t *testing.T) {
	tests := []struct {
		name     string
		date     string
		minDays  int  // 使用范围判断
		maxDays  int
		negative bool // 是否应该是负数
	}{
		{
			name:    "future date ISO",
			date:    time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02"),
			minDays: 29,
			maxDays: 31,
		},
		{
			name:    "far future",
			date:    "2030-01-01",
			minDays: 1000, // 肯定远大于0
			maxDays: 5000,
		},
		{
			name:     "past date",
			date:     "2020-01-01",
			negative: true,
		},
		{
			name:    "empty date",
			date:    "",
			minDays: -1,
			maxDays: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days := calculateDaysRemaining(tt.date)

			if tt.negative {
				if days >= 0 {
					t.Errorf("Expected negative days for past date, got %d", days)
				}
			} else if tt.minDays == -1 && tt.maxDays == -1 {
				if days != -1 {
					t.Errorf("Expected -1 for empty date, got %d", days)
				}
			} else {
				if days < tt.minDays || days > tt.maxDays {
					t.Errorf("Days = %d, expected between %d and %d", days, tt.minDays, tt.maxDays)
				}
			}
		})
	}
}

func TestCalculateDaysRemaining_Formats(t *testing.T) {
	// Test various date formats
	futureDate := time.Now().Add(60 * 24 * time.Hour)

	formats := []struct {
		name   string
		format string
	}{
		{"ISO date", "2006-01-02"},
		{"ISO datetime", "2006-01-02T15:04:05Z"},
	}

	for _, f := range formats {
		t.Run(f.name, func(t *testing.T) {
			dateStr := futureDate.Format(f.format)
			days := calculateDaysRemaining(dateStr)
			if days < 59 || days > 61 {
				t.Errorf("calculateDaysRemaining(%q) = %d, expected ~60", dateStr, days)
			}
		})
	}
}

func TestDetermineWatchStatus(t *testing.T) {
	config := DefaultMonitorConfig()
	monitor := NewDomainMonitor(config)

	tests := []struct {
		days int
		want WatchStatus
	}{
		{100, WatchStatusActive},
		{30, WatchStatusWarning},
		{15, WatchStatusWarning},
		{7, WatchStatusCritical},
		{3, WatchStatusCritical},
		{0, WatchStatusExpired},
		{-1, WatchStatusExpired},
		{-10, WatchStatusExpired},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.days+1000)), func(t *testing.T) {
			got := monitor.determineWatchStatus(tt.days)
			if got != tt.want {
				t.Errorf("determineWatchStatus(%d) = %s, want %s", tt.days, got, tt.want)
			}
		})
	}
}

func TestDomainAlert_Fields(t *testing.T) {
	alert := &DomainAlert{
		ID:        "test-alert-1",
		Domain:    "example.com",
		Type:      AlertExpiryWarning,
		Level:     AlertLevelWarning,
		Message:   "域名将在 15 天后到期",
		OldValue:  "active",
		NewValue:  "warning",
		Timestamp: time.Now(),
		Action:    "请关注域名到期时间",
	}

	if alert.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", alert.Domain)
	}
	if alert.Type != AlertExpiryWarning {
		t.Errorf("Type = %s, want %s", alert.Type, AlertExpiryWarning)
	}
	if alert.Level != AlertLevelWarning {
		t.Errorf("Level = %s, want %s", alert.Level, AlertLevelWarning)
	}
}

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different length", []string{"a", "b"}, []string{"a"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"both empty", []string{}, []string{}, true},
		{"nil vs empty", nil, []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringSlicesEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("stringSlicesEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatStringSlice(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{"a", "b", "c"}, "a, b, c"},
		{[]string{"single"}, "single"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatStringSlice(tt.input)
			if got != tt.want {
				t.Errorf("formatStringSlice(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatContact(t *testing.T) {
	tests := []struct {
		name  string
		contact *whoisparser.Contact
		want  string
	}{
		{
			name: "full contact",
			contact: &whoisparser.Contact{
				Name:         "John Doe",
				Organization: "Acme Corp",
				Email:        "john@acme.com",
			},
			want: "John Doe; Acme Corp; john@acme.com",
		},
		{
			name:  "nil contact",
			contact: nil,
			want:  "",
		},
		{
			name: "name only",
			contact: &whoisparser.Contact{
				Name: "John Doe",
			},
			want: "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatContact(tt.contact)
			if got != tt.want {
				t.Errorf("formatContact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAlertType_Constants(t *testing.T) {
	types := []AlertType{
		AlertExpiryWarning,
		AlertExpiryCritical,
		AlertExpiryPassed,
		AlertStatusChange,
		AlertRegistrantChange,
		AlertNSChange,
		AlertDNSChange,
		AlertQueryError,
	}

	for _, at := range types {
		if string(at) == "" {
			t.Errorf("AlertType constant should not be empty")
		}
	}
}

func TestAlertLevel_Constants(t *testing.T) {
	levels := []AlertLevel{
		AlertLevelInfo,
		AlertLevelWarning,
		AlertLevelCritical,
	}

	for _, al := range levels {
		if string(al) == "" {
			t.Errorf("AlertLevel constant should not be empty")
		}
	}
}

func TestWatchStatus_Constants(t *testing.T) {
	statuses := []WatchStatus{
		WatchStatusActive,
		WatchStatusWarning,
		WatchStatusCritical,
		WatchStatusExpired,
		WatchStatusError,
		WatchStatusChanged,
	}

	for _, ws := range statuses {
		if string(ws) == "" {
			t.Errorf("WatchStatus constant should not be empty")
		}
	}
}

func TestCollectAlerts(t *testing.T) {
	ch := make(chan *DomainAlert, 3)
	ch <- &DomainAlert{Domain: "a.com"}
	ch <- &DomainAlert{Domain: "b.com"}
	ch <- &DomainAlert{Domain: "c.com"}
	close(ch)

	alerts := CollectAlerts(ch)
	if len(alerts) != 3 {
		t.Errorf("Alerts count = %d, want 3", len(alerts))
	}
}

func TestCollectAlerts_Empty(t *testing.T) {
	ch := make(chan *DomainAlert)
	close(ch)

	alerts := CollectAlerts(ch)
	if len(alerts) != 0 {
		t.Errorf("Alerts count = %d, want 0", len(alerts))
	}
}

func TestDefaultMonitorConfig(t *testing.T) {
	config := DefaultMonitorConfig()

	if config.CheckInterval != 60 {
		t.Errorf("Default CheckInterval = %d, want 60", config.CheckInterval)
	}
	if config.ExpiryWarningDays != 30 {
		t.Errorf("Default ExpiryWarningDays = %d, want 30", config.ExpiryWarningDays)
	}
	if config.ExpiryCriticalDays != 7 {
		t.Errorf("Default ExpiryCriticalDays = %d, want 7", config.ExpiryCriticalDays)
	}
	if !config.WatchStatusChange {
		t.Error("Default WatchStatusChange should be true")
	}
	if !config.WatchRegistrantChange {
		t.Error("Default WatchRegistrantChange should be true")
	}
	if !config.WatchNSChange {
		t.Error("Default WatchNSChange should be true")
	}
}

func TestDomainMonitor_AddWatch_ExpiryStatus(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	// 域名即将到期
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "expiring.com",
			ExpirationDate: time.Now().Add(5 * 24 * time.Hour).Format("2006-01-02"),
		},
	}

	monitor.AddWatch("expiring.com", info)

	state := monitor.GetWatchState("expiring.com")
	if state == nil {
		t.Fatal("GetWatchState() returned nil")
	}
	if state.Status != WatchStatusCritical {
		t.Errorf("Status = %s, want critical (5 days remaining)", state.Status)
	}
}

func TestDomainMonitor_AddWatch_WarningStatus(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	// 域名在警告期内
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "warning.com",
			ExpirationDate: time.Now().Add(20 * 24 * time.Hour).Format("2006-01-02"),
		},
	}

	monitor.AddWatch("warning.com", info)

	state := monitor.GetWatchState("warning.com")
	if state == nil {
		t.Fatal("GetWatchState() returned nil")
	}
	if state.Status != WatchStatusWarning {
		t.Errorf("Status = %s, want warning (20 days remaining)", state.Status)
	}
}

func TestDomainMonitor_MultipleDomains(t *testing.T) {
	monitor := NewDomainMonitor(DefaultMonitorConfig())

	monitor.AddWatch("a.com", nil)
	monitor.AddWatch("b.com", nil)
	monitor.AddWatch("c.com", nil)

	watchList := monitor.GetWatchList()
	if len(watchList) != 3 {
		t.Errorf("WatchList count = %d, want 3", len(watchList))
	}

	monitor.RemoveWatch("b.com")
	watchList = monitor.GetWatchList()
	if len(watchList) != 2 {
		t.Errorf("WatchList count = %d, want 2 after removal", len(watchList))
	}
}

func BenchmarkCalculateDaysRemaining(b *testing.B) {
	date := time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateDaysRemaining(date)
	}
}

func BenchmarkStringSlicesEqual(b *testing.B) {
	a := []string{"a", "b", "c", "d", "e"}
	c := []string{"a", "b", "c", "d", "e"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stringSlicesEqual(a, c)
	}
}
