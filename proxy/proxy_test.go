package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apig0/config"

	"github.com/gin-gonic/gin"
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

func TestRetryTransportSupportsOptInTLSSkipVerify(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	withoutSkip := newRetryTransport(config.ServiceConfig{})
	if resp, err := withoutSkip.RoundTrip(req); err == nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		t.Fatal("expected TLS verification failure without skip flag")
	}

	withSkip := newRetryTransport(config.ServiceConfig{TLSSkipVerify: true})
	resp, err := withSkip.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected opt-in TLS skip verify to succeed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestNewReverseProxyStripsGatewayAuthHeadersBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type capturedRequest struct {
		authorization string
		apiKey        string
		path          string
	}

	seen := make(chan capturedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- capturedRequest{
			authorization: r.Header.Get("Authorization"),
			apiKey:        r.Header.Get("X-API-Key"),
			path:          r.URL.Path,
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	router := gin.New()
	router.Any("/orders/*path", NewReverseProxy(config.ServiceConfig{
		Name:     "orders",
		BaseURL:  upstream.URL,
		AuthType: config.ServiceAuthNone,
	}))
	router.Any("/orders", NewReverseProxy(config.ServiceConfig{
		Name:     "orders",
		BaseURL:  upstream.URL,
		AuthType: config.ServiceAuthNone,
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer gateway-token")
	req.Header.Set("X-API-Key", "gateway-key")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	got := <-seen
	if got.authorization != "" {
		t.Fatalf("expected Authorization header to be stripped, got %q", got.authorization)
	}
	if got.apiKey != "" {
		t.Fatalf("expected X-API-Key header to be stripped, got %q", got.apiKey)
	}
	if got.path != "/" {
		t.Fatalf("unexpected upstream path: %q", got.path)
	}
}
