package proxy

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"apig0/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRetryTransportReturnsReadableFinalRetryResponse(t *testing.T) {
	calls := 0
	rt := &retryTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			body := "retry"
			if calls == 2 {
				body = "final"
			}
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
		service: config.ServiceConfig{RetryCount: 1},
	}

	req, err := http.NewRequest(http.MethodGet, "https://upstream.example/resource", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()

	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read final response body: %v", err)
	}
	if string(raw) != "final" {
		t.Fatalf("unexpected final response body: %q", raw)
	}
}
