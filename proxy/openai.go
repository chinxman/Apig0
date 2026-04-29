package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

func NewOpenAICompatibleProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenValue, ok := c.Get("api_token")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "gateway API token required"})
			return
		}
		token, ok := tokenValue.(config.APIToken)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "gateway API token required"})
			return
		}

		serviceName := config.NormalizeAllowedServiceName(token.OpenAIService)
		if serviceName == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "token is not enabled for OpenAI-compatible access"})
			return
		}

		service, ok := config.LookupService(serviceName)
		if !ok || !service.OpenAICompat {
			c.JSON(http.StatusBadGateway, gin.H{"error": "OpenAI-compatible service is unavailable"})
			return
		}
		if len(token.AllowedServices) > 0 && !contains(token.AllowedServices, serviceName) {
			c.JSON(http.StatusForbidden, gin.H{"error": "token scope denied for AI service"})
			return
		}
		if len(token.AllowedProviders) > 0 && !contains(token.AllowedProviders, config.NormalizeProviderName(service.Provider)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "token provider scope denied"})
			return
		}

		upstreamPath := strings.TrimSpace(c.Param("openaiPath"))
		if upstreamPath == "" {
			upstreamPath = "/models"
		}
		if !strings.HasPrefix(upstreamPath, "/") {
			upstreamPath = "/" + upstreamPath
		}
		if upstreamPath != "/" {
			upstreamPath = strings.TrimRight(upstreamPath, "/")
		}
		decision := config.EvaluateRouteAccess(token.User, serviceName, c.Request.Method, upstreamPath, "token")
		if !decision.Allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason})
			return
		}

		if needsModelScope(c.Request.Method, upstreamPath) {
			model, err := extractRequestModel(c.Request)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
				return
			}
			if len(token.AllowedModels) > 0 {
				if model == "" {
					c.JSON(http.StatusForbidden, gin.H{"error": "token requires an allowed model"})
					return
				}
				if !contains(token.AllowedModels, model) {
					c.JSON(http.StatusForbidden, gin.H{"error": "model is not allowed for this token"})
					return
				}
			}
		}

		u, err := url.Parse(service.BaseURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream service URL"})
			return
		}

		reverseProxy := httputil.NewSingleHostReverseProxy(u)
		originalDirector := reverseProxy.Director
		reverseProxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = joinProxyPath(u.Path, upstreamPath)
			req.URL.RawPath = ""
			req.Header.Del("Authorization")
			req.Header.Del("X-API-Key")
			applyServiceAuth(req, service)
			req.Header.Set("X-Forwarded-For", c.ClientIP())
		}
		reverseProxy.Transport = newRetryTransport(service)
		reverseProxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
		}
		reverseProxy.ModifyResponse = func(resp *http.Response) error {
			if strings.HasSuffix(resp.Request.URL.Path, "/models") && len(token.AllowedModels) > 0 {
				return filterOpenAIModelsResponse(resp, token.AllowedModels)
			}
			return nil
		}

		timeout := time.Duration(service.TimeoutMS) * time.Millisecond
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		reverseProxy.ServeHTTP(c.Writer, c.Request)
	}
}

func needsModelScope(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	switch path {
	case "/chat/completions", "/responses", "/completions", "/embeddings", "/images/generations", "/audio/transcriptions", "/audio/translations", "/audio/speech":
		return true
	default:
		return false
	}
}

func extractRequestModel(req *http.Request) (string, error) {
	if req.Body == nil {
		return "", nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	model, _ := payload["model"].(string)
	return strings.TrimSpace(model), nil
}

func filterOpenAIModelsResponse(resp *http.Response, allowedModels []string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	var payload struct {
		Object string                   `json:"object"`
		Data   []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}

	filtered := make([]map[string]interface{}, 0, len(payload.Data))
	for _, item := range payload.Data {
		modelID, _ := item["id"].(string)
		if contains(allowedModels, strings.TrimSpace(modelID)) {
			filtered = append(filtered, item)
		}
	}
	payload.Data = filtered
	if payload.Object == "" {
		payload.Object = "list"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(encoded))
	resp.ContentLength = int64(len(encoded))
	resp.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
	resp.Header.Set("Content-Type", "application/json")
	return nil
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(needle) {
			return true
		}
	}
	return false
}
