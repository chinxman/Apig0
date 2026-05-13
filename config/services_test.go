package config

import "testing"

func TestValidateServiceBaseURLAcceptsHTTPAndHTTPS(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8080/api",
		"https://api.example.com/v1",
	} {
		if err := ValidateServiceBaseURL(raw); err != nil {
			t.Fatalf("expected %q to be valid: %v", raw, err)
		}
	}
}

func TestValidateServiceBaseURLRejectsUnsafeForms(t *testing.T) {
	for _, raw := range []string{
		"",
		"localhost:8080",
		"//api.example.com/v1",
		"file:///etc/passwd",
		"https://user:pass@api.example.com",
		"https://api.example.com/v1#token",
	} {
		if err := ValidateServiceBaseURL(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestNormalizeServiceConfigPreservesTLSSkipVerify(t *testing.T) {
	svc, ok := normalizeServiceConfig(ServiceConfig{
		Name:          "orders",
		BaseURL:       "https://orders.internal",
		TLSSkipVerify: true,
		Enabled:       true,
	})
	if !ok {
		t.Fatal("expected service config to normalize")
	}
	if !svc.TLSSkipVerify {
		t.Fatal("expected tls skip verify flag to persist")
	}
}
