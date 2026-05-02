package proxy

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

// NewReverseProxy creates a reverse proxy to the given backend service.
func NewReverseProxy(service config.ServiceConfig) gin.HandlerFunc {
	u, err := url.Parse(service.BaseURL)
	if err != nil {
		return func(c *gin.Context) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream URL"})
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		strippedPath, strippedRawPath := rewriteProxyPath(req, service.Name)
		originalDirector(req)
		req.URL.Path = joinProxyPath(u.Path, strippedPath)
		if strippedRawPath != "" {
			req.URL.RawPath = joinProxyPath(u.EscapedPath(), strippedRawPath)
		} else {
			req.URL.RawPath = ""
		}
		applyServiceAuth(req, service)
	}
	proxy.Transport = newRetryTransport(service)
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	return func(c *gin.Context) {
		c.Request.Header.Set("X-Forwarded-For", c.ClientIP())
		timeout := time.Duration(service.TimeoutMS) * time.Millisecond
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func rewriteProxyPath(req *http.Request, serviceName string) (string, string) {
	prefix := "/" + strings.Trim(strings.ToLower(serviceName), "/")
	path := req.URL.Path
	raw := req.URL.RawPath
	if path == prefix || path == prefix+"/" {
		return "", ""
	}
	if strings.HasPrefix(path, prefix+"/") {
		next := "/" + strings.TrimLeft(strings.TrimPrefix(path, prefix), "/")
		if raw == "" {
			return next, ""
		}
		if raw == prefix || raw == prefix+"/" {
			return next, ""
		}
		if strings.HasPrefix(raw, prefix+"/") {
			return next, "/" + strings.TrimLeft(strings.TrimPrefix(raw, prefix), "/")
		}
		return next, ""
	}
	return path, raw
}

func joinProxyPath(basePath, strippedPath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		if strippedPath == "" {
			return "/"
		}
		return "/" + strings.TrimLeft(strippedPath, "/")
	}
	basePath = strings.TrimRight(basePath, "/")
	if strippedPath == "" || strippedPath == "/" {
		return basePath
	}
	return basePath + "/" + strings.TrimLeft(strippedPath, "/")
}

func TestServiceAuth(service config.ServiceConfig, requestPath string) (int, error) {
	u, err := url.Parse(service.BaseURL)
	if err != nil {
		return 0, err
	}
	requestPath = strings.TrimSpace(requestPath)
	if requestPath != "" {
		if !strings.HasPrefix(requestPath, "/") {
			requestPath = "/" + requestPath
		}
		u.Path = strings.TrimRight(u.Path, "/") + requestPath
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	applyServiceAuth(req, service)
	resp, err := newRetryTransport(service).RoundTrip(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func applyServiceAuth(req *http.Request, service config.ServiceConfig) {
	secret, hasSecret, err := config.GetServiceSecret(service.Name)
	if err != nil || !hasSecret {
		return
	}

	switch service.AuthType {
	case config.ServiceAuthBearer:
		req.Header.Set("Authorization", "Bearer "+secret)
	case config.ServiceAuthXAPIKey:
		header := service.HeaderName
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, secret)
	case config.ServiceAuthCustomHeader:
		header := service.HeaderName
		if header == "" {
			header = "Authorization"
		}
		req.Header.Set(header, secret)
	case config.ServiceAuthBasic:
		if service.BasicUsername == "" {
			return
		}
		token := base64.StdEncoding.EncodeToString([]byte(service.BasicUsername + ":" + secret))
		req.Header.Set("Authorization", "Basic "+token)
	}
}

type retryTransport struct {
	base    http.RoundTripper
	service config.ServiceConfig
}

func newRetryTransport(service config.ServiceConfig) http.RoundTripper {
	return &retryTransport{
		base:    http.DefaultTransport,
		service: service,
	}
}

func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := 1
	if isSafeRetryMethod(req.Method) {
		attempts += rt.service.RetryCount
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		cloned := cloneRequest(req)
		timeout := time.Duration(rt.service.TimeoutMS) * time.Millisecond
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(cloned.Context(), timeout)
		cloned = cloned.WithContext(ctx)
		resp, err := rt.base.RoundTrip(cloned)
		cancel()
		isLastAttempt := attempt == attempts-1
		if err == nil && (!shouldRetryStatus(req.Method, resp.StatusCode) || isLastAttempt) {
			return resp, nil
		}
		if resp != nil {
			if !shouldRetryStatus(req.Method, resp.StatusCode) {
				return resp, nil
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		lastErr = err
	}
	return nil, lastErr
}

func cloneRequest(req *http.Request) *http.Request {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	return cloned
}

func isSafeRetryMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func shouldRetryStatus(method string, status int) bool {
	if !isSafeRetryMethod(method) {
		return false
	}
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
