package whois

import (
	"errors"
	"fmt"
	"testing"
)

func TestWhoisError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *WhoisError
		wantMsg string
	}{
		{
			name:    "with cause",
			err:     NewWhoisError(ErrConnectionReset, "连接被重置", fmt.Errorf("peer reset")),
			wantMsg: "连接被重置: peer reset",
		},
		{
			name:    "without cause",
			err:     NewWhoisError(ErrRateLimited, "被限速", nil),
			wantMsg: "被限速",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %s, want %s", got, tt.wantMsg)
			}
		})
	}
}

func TestWhoisError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := NewWhoisError(ErrConnectionReset, "连接被重置", cause)
	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestWhoisError_Unwrap_Nil(t *testing.T) {
	err := NewWhoisError(ErrRateLimited, "被限速", nil)
	if unwrapped := err.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestWhoisError_IsRetryable(t *testing.T) {
	tests := []struct {
		name         string
		errType      ErrorType
		wantRetryable bool
	}{
		{"connection reset", ErrConnectionReset, true},
		{"interval too short", ErrIntervalTooShort, true},
		{"access too fast", ErrAccessTooFast, true},
		{"server connect failed", ErrServerConnectFailed, true},
		{"rate limited", ErrRateLimited, true},
		{"query timeout", ErrQueryTimeout, true},
		{"domain empty", ErrDomainEmpty, false},
		{"server not found", ErrServerNotFound, false},
		{"parse failed", ErrParseFailed, false},
		{"validation failed", ErrValidationFailed, false},
		{"proxy failed", ErrProxyFailed, false},
		{"cache miss", ErrCacheMiss, false},
		{"referral failed", ErrReferralFailed, false},
		{"domain not registered", ErrDomainNotRegistered, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewWhoisError(tt.errType, "test", nil)
			if err.IsRetryable() != tt.wantRetryable {
				t.Errorf("IsRetryable() = %v, want %v", err.IsRetryable(), tt.wantRetryable)
			}
		})
	}
}

func TestNewWhoisError(t *testing.T) {
	err := NewWhoisError(ErrRateLimited, "被限速", nil)
	if err.Type != ErrRateLimited {
		t.Errorf("Type = %d, want %d", err.Type, ErrRateLimited)
	}
	if err.Message != "被限速" {
		t.Errorf("Message = %s, want 被限速", err.Message)
	}
	if err.Cause != nil {
		t.Error("Cause should be nil")
	}
}

func TestCheckError_Nil(t *testing.T) {
	result := CheckError(nil)
	if result != nil {
		t.Error("CheckError(nil) should return nil")
	}
}

func TestCheckError_AlreadyWhoisError(t *testing.T) {
	original := NewWhoisError(ErrRateLimited, "被限速", nil)
	result := CheckError(original)
	if result != original {
		t.Error("CheckError should return same WhoisError")
	}
}

func TestCheckError_ConnectionReset(t *testing.T) {
	result := CheckError(fmt.Errorf("connection reset by peer"))
	if result.Type != ErrConnectionReset {
		t.Errorf("Type = %d, want ErrConnectionReset", result.Type)
	}
}

func TestCheckError_IntervalTooShort(t *testing.T) {
	result := CheckError(fmt.Errorf("queried interval is too short"))
	if result.Type != ErrIntervalTooShort {
		t.Errorf("Type = %d, want ErrIntervalTooShort", result.Type)
	}
}

func TestCheckError_ServerConnectFailed(t *testing.T) {
	result := CheckError(fmt.Errorf("connect to whois server failed"))
	if result.Type != ErrServerConnectFailed {
		t.Errorf("Type = %d, want ErrServerConnectFailed", result.Type)
	}
}

func TestCheckError_AccessTooFast(t *testing.T) {
	result := CheckError(fmt.Errorf("your access is too fast"))
	if result.Type != ErrAccessTooFast {
		t.Errorf("Type = %d, want ErrAccessTooFast", result.Type)
	}
}

func TestCheckError_RateLimited(t *testing.T) {
	result := CheckError(fmt.Errorf("query limit exceeded"))
	if result.Type != ErrRateLimited {
		t.Errorf("Type = %d, want ErrRateLimited", result.Type)
	}
}

func TestCheckError_Timeout(t *testing.T) {
	result := CheckError(fmt.Errorf("context deadline exceeded"))
	if result.Type != ErrQueryTimeout {
		t.Errorf("Type = %d, want ErrQueryTimeout", result.Type)
	}
}

func TestCheckError_Unknown(t *testing.T) {
	result := CheckError(fmt.Errorf("some unknown error"))
	if result.Type != ErrorType(0) {
		t.Errorf("Type = %d, want 0 (unknown)", result.Type)
	}
}

func TestCheckError_WrappedWhoisError(t *testing.T) {
	original := NewWhoisError(ErrRateLimited, "被限速", nil)
	wrapped := fmt.Errorf("wrapper: %w", original)
	result := CheckError(wrapped)
	if result.Type != ErrRateLimited {
		t.Errorf("Type = %d, want ErrRateLimited", result.Type)
	}
}

func TestNewErrorWrapper(t *testing.T) {
	err := fmt.Errorf("test error")
	wrapper := NewErrorWrapper(err, ErrConnectionReset)
	if wrapper.Type != ErrConnectionReset {
		t.Errorf("Type = %d, want ErrConnectionReset", wrapper.Type)
	}
	if wrapper.Cause != err {
		t.Error("Cause should match original error")
	}
}

func TestErrorType_Constants(t *testing.T) {
	if ErrConnectionReset != 1 {
		t.Errorf("ErrConnectionReset = %d, want 1", ErrConnectionReset)
	}
	if ErrRateLimited != 12 {
		t.Errorf("ErrRateLimited = %d, want 11", ErrRateLimited)
	}
}

func TestWhoisError_ErrorsIs(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := NewWhoisError(ErrConnectionReset, "连接被重置", cause)
	if !errors.Is(err, cause) {
		t.Error("errors.Is should work with WhoisError chain")
	}
}

func BenchmarkCheckError(b *testing.B) {
	err := fmt.Errorf("connection reset by peer")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckError(err)
	}
}

func BenchmarkNewWhoisError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewWhoisError(ErrRateLimited, "被限速", nil)
	}
}
