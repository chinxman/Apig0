package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ===================================================================
// HashiCorp Vault — KV v2 over HTTP
// ===================================================================
//
// Env vars:
//   VAULT_ADDRESS    (default: http://127.0.0.1:8200)
//   VAULT_ENGINE     (default: secret)
//   VAULT_TOKEN      — token auth
//   VAULT_ROLE_ID    — AppRole auth (with VAULT_SECRET_ID)
//   VAULT_SECRET_ID  — AppRole auth (with VAULT_ROLE_ID)
//
// Expected KV v2 layout:
//   vault kv put secret/totp/devin secret=BASE32SECRET

type HashicorpVault struct {
	address   string
	engine    string
	secretKey string
	token     string
	http      *http.Client
}

func NewHashicorpVault(cfg *VaultConfig) (*HashicorpVault, error) {
	address := envDefault("VAULT_ADDRESS", "http://127.0.0.1:8200")
	engine := envDefault("VAULT_ENGINE", "secret")

	v := &HashicorpVault{
		address:   strings.TrimRight(address, "/"),
		engine:    engine,
		secretKey: cfg.SecretKey,
		http:      &http.Client{Timeout: 10 * time.Second},
	}

	token := os.Getenv("VAULT_TOKEN")
	roleID := os.Getenv("VAULT_ROLE_ID")
	secretID := os.Getenv("VAULT_SECRET_ID")

	if token != "" {
		v.token = token
	} else if roleID != "" && secretID != "" {
		t, err := v.appRoleLogin(roleID, secretID)
		if err != nil {
			return nil, err
		}
		v.token = t
	} else {
		return nil, fmt.Errorf("set VAULT_TOKEN or VAULT_ROLE_ID + VAULT_SECRET_ID")
	}
	return v, nil
}

func (v *HashicorpVault) appRoleLogin(roleID, secretID string) (string, error) {
	body := fmt.Sprintf(`{"role_id":"%s","secret_id":"%s"}`, roleID, secretID)
	url := v.address + "/v1/auth/approle/login"

	resp, err := v.http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("approle login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("approle login HTTP %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Auth.ClientToken == "" {
		return "", fmt.Errorf("approle returned empty token")
	}
	return result.Auth.ClientToken, nil
}

func (v *HashicorpVault) LoadSecret(secretPath string, key string) (string, error) {
	// KV v2: GET /v1/{engine}/data/{secretPath}/{key}
	url := fmt.Sprintf("%s/v1/%s/data/%s/%s", v.address, v.engine, secretPath, key)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Vault-Token", v.token)

	resp, err := v.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("not found: %s/%s", secretPath, key)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vault HTTP %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	val, ok := result.Data.Data[v.secretKey]
	if !ok {
		return "", fmt.Errorf("key %q not in vault response", v.secretKey)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("vault key %q is not a string", v.secretKey)
	}
	return str, nil
}

func (v *HashicorpVault) StoreSecret(secretPath, key, value string) error {
	url := fmt.Sprintf("%s/v1/%s/data/%s/%s", v.address, v.engine, secretPath, key)
	body, _ := json.Marshal(map[string]interface{}{
		"data": map[string]string{v.secretKey: value},
	})
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("X-Vault-Token", v.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("vault store: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vault store HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (v *HashicorpVault) DeleteSecret(secretPath, key string) error {
	url := fmt.Sprintf("%s/v1/%s/data/%s/%s", v.address, v.engine, secretPath, key)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Vault-Token", v.token)
	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("vault delete: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (v *HashicorpVault) ListKeys(secretPath string) ([]string, error) {
	url := fmt.Sprintf("%s/v1/%s/metadata/%s", v.address, v.engine, secretPath)
	req, _ := http.NewRequest("LIST", url, nil)
	req.Header.Set("X-Vault-Token", v.token)
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault list HTTP %d: %s", resp.StatusCode, b)
	}
	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data.Keys, nil
}

func (v *HashicorpVault) Health() error {
	req, _ := http.NewRequest("GET", v.address+"/v1/sys/health", nil)
	req.Header.Set("X-Vault-Token", v.token)
	resp, err := v.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 429 {
		return fmt.Errorf("vault unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (v *HashicorpVault) String() string { return "hashicorp" }

// ===================================================================
// CLIVault — shared base for CLI-based providers
// ===================================================================

type CLIVault struct {
	name     string
	buildCmd func(secretPath, key string) (string, []string)
	checkCmd func() (string, []string) // optional health check
}

func (v *CLIVault) LoadSecret(secretPath, key string) (string, error) {
	bin, args := v.buildCmd(secretPath, key)
	if err := validateExecBinary(bin, allowedProviderBinaries); err != nil {
		return "", fmt.Errorf("%s: %w", v.name, err)
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", v.name, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("%s: %w", v.name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (v *CLIVault) Health() error {
	if v.checkCmd == nil {
		return nil
	}
	bin, args := v.checkCmd()
	if err := validateExecBinary(bin, allowedProviderBinaries); err != nil {
		return fmt.Errorf("%s health: %w", v.name, err)
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s health: %s", v.name, strings.TrimSpace(string(out)))
	}
	return nil
}

func (v *CLIVault) StoreSecret(secretPath, key, value string) error {
	return fmt.Errorf("%s: %w", v.name, ErrReadOnly)
}
func (v *CLIVault) DeleteSecret(secretPath, key string) error {
	return fmt.Errorf("%s: %w", v.name, ErrReadOnly)
}
func (v *CLIVault) ListKeys(secretPath string) ([]string, error) {
	return nil, fmt.Errorf("%s: %w", v.name, ErrReadOnly)
}
func (v *CLIVault) String() string { return v.name }

// requireCLI checks that a binary exists in PATH.
func requireCLI(name, installURL string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s CLI not found — install: %s", name, installURL)
	}
	return nil
}

// ===================================================================
// AWS Secrets Manager
// ===================================================================
//
// Env vars:
//   AWS_REGION or AWS_DEFAULT_REGION (default: us-east-1)
//
// Expects: aws secretsmanager get-secret-value --secret-id {path}/{user}
// The secret value should be the raw TOTP base32 string.
// AWS CLI must be installed and authenticated (aws configure / IAM role / SSO).

func NewAWSVault(cfg *VaultConfig) (*CLIVault, error) {
	if err := requireCLI("aws", "https://aws.amazon.com/cli/"); err != nil {
		return nil, err
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	return &CLIVault{
		name: "aws",
		buildCmd: func(secretPath, key string) (string, []string) {
			secretID := secretPath + "/" + key
			return "aws", []string{
				"secretsmanager", "get-secret-value",
				"--secret-id", secretID,
				"--query", "SecretString",
				"--output", "text",
				"--region", region,
			}
		},
		checkCmd: func() (string, []string) {
			return "aws", []string{"sts", "get-caller-identity"}
		},
	}, nil
}

// ===================================================================
// Google Cloud Secret Manager
// ===================================================================
//
// Env vars:
//   GCP_PROJECT (required)
//
// Expects: gcloud secrets versions access latest --secret={path}-{user}
// Secret name format: {secretPath}-{user} (e.g. totp-devin)
// gcloud CLI must be installed and authenticated (gcloud auth login).

func NewGCPVault(cfg *VaultConfig) (*CLIVault, error) {
	if err := requireCLI("gcloud", "https://cloud.google.com/sdk/docs/install"); err != nil {
		return nil, err
	}

	project := os.Getenv("GCP_PROJECT")
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT is required")
	}

	return &CLIVault{
		name: "gcp",
		buildCmd: func(secretPath, key string) (string, []string) {
			secretName := secretPath + "-" + key
			args := []string{
				"secrets", "versions", "access", "latest",
				"--secret=" + secretName,
				"--project=" + project,
			}
			return "gcloud", args
		},
		checkCmd: func() (string, []string) {
			return "gcloud", []string{"auth", "print-access-token", "--quiet"}
		},
	}, nil
}

// ===================================================================
// Azure Key Vault
// ===================================================================
//
// Env vars:
//   AZURE_VAULT_NAME (required) — the Key Vault resource name
//
// Expects: az keyvault secret show --vault-name X --name {path}-{user}
// Secret name format: {secretPath}-{user} (e.g. totp-devin)
// Azure CLI must be installed and authenticated (az login).

func NewAzureVault(cfg *VaultConfig) (*CLIVault, error) {
	if err := requireCLI("az", "https://learn.microsoft.com/en-us/cli/azure/install-azure-cli"); err != nil {
		return nil, err
	}

	vaultName := os.Getenv("AZURE_VAULT_NAME")
	if vaultName == "" {
		return nil, fmt.Errorf("AZURE_VAULT_NAME is required")
	}

	return &CLIVault{
		name: "azure",
		buildCmd: func(secretPath, key string) (string, []string) {
			secretName := secretPath + "-" + key
			return "az", []string{
				"keyvault", "secret", "show",
				"--vault-name", vaultName,
				"--name", secretName,
				"--query", "value",
				"--output", "tsv",
			}
		},
		checkCmd: func() (string, []string) {
			return "az", []string{"account", "show"}
		},
	}, nil
}

// ===================================================================
// 1Password
// ===================================================================
//
// Env vars:
//   OP_VAULT (default: "Private") — the 1Password vault name
//
// Expects: op read "op://{vault}/{path}-{user}/secret"
// Item name format: {secretPath}-{user} (e.g. totp-devin)
// Field name matches VAULT_SECRET_KEY (default: "secret")
// 1Password CLI must be installed and signed in (op signin).

func New1PasswordVault(cfg *VaultConfig) (*CLIVault, error) {
	if err := requireCLI("op", "https://1password.com/downloads/command-line/"); err != nil {
		return nil, err
	}

	opVault := envDefault("OP_VAULT", "Private")

	return &CLIVault{
		name: "1password",
		buildCmd: func(secretPath, key string) (string, []string) {
			itemName := secretPath + "-" + key
			ref := fmt.Sprintf("op://%s/%s/%s", opVault, itemName, cfg.SecretKey)
			return "op", []string{"read", ref}
		},
		checkCmd: func() (string, []string) {
			return "op", []string{"whoami"}
		},
	}, nil
}

// ===================================================================
// CyberArk CCP (Central Credential Provider) — HTTP API
// ===================================================================
//
// Env vars:
//   CYBERARK_ADDRESS (required) — CCP base URL
//   CYBERARK_APP_ID  (required) — application ID
//   CYBERARK_SAFE    (required) — safe name
//   CYBERARK_FOLDER  (default: "Root")
//
// Fetches: GET {address}/AIMWebService/api/Accounts?AppID=X&Safe=Y&Folder=Z&Object={user}
// Reads the "Content" field from the JSON response.

type CyberArkVault struct {
	address string
	appID   string
	safe    string
	folder  string
	http    *http.Client
}

func NewCyberArkVault(cfg *VaultConfig) (*CyberArkVault, error) {
	address := os.Getenv("CYBERARK_ADDRESS")
	appID := os.Getenv("CYBERARK_APP_ID")
	safe := os.Getenv("CYBERARK_SAFE")
	if address == "" || appID == "" || safe == "" {
		return nil, fmt.Errorf("CYBERARK_ADDRESS, CYBERARK_APP_ID, and CYBERARK_SAFE are required")
	}

	return &CyberArkVault{
		address: strings.TrimRight(address, "/"),
		appID:   appID,
		safe:    safe,
		folder:  envDefault("CYBERARK_FOLDER", "Root"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (v *CyberArkVault) LoadSecret(secretPath string, key string) (string, error) {
	objectName := secretPath + "-" + key
	url := fmt.Sprintf("%s/AIMWebService/api/Accounts?AppID=%s&Safe=%s&Folder=%s&Object=%s",
		v.address, v.appID, v.safe, v.folder, objectName)

	resp, err := v.http.Get(url)
	if err != nil {
		return "", fmt.Errorf("cyberark request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("cyberark HTTP %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Content string `json:"Content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Content == "" {
		return "", fmt.Errorf("cyberark returned empty Content for %s", objectName)
	}
	return result.Content, nil
}

func (v *CyberArkVault) Health() error {
	resp, err := v.http.Get(v.address + "/AIMWebService/api/verify")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (v *CyberArkVault) StoreSecret(secretPath, key, value string) error {
	return fmt.Errorf("cyberark: %w", ErrReadOnly)
}
func (v *CyberArkVault) DeleteSecret(secretPath, key string) error {
	return fmt.Errorf("cyberark: %w", ErrReadOnly)
}
func (v *CyberArkVault) ListKeys(secretPath string) ([]string, error) {
	return nil, fmt.Errorf("cyberark: %w", ErrReadOnly)
}
func (v *CyberArkVault) String() string { return "cyberark" }

// ===================================================================
// Generic HTTP Vault — connects to any REST API
// ===================================================================
//
// Env vars:
//   VAULT_HTTP_URL       (required) — URL template, use {{path}} and {{key}}
//                         e.g. https://my-api.com/secrets/{{path}}/{{key}}
//   VAULT_HTTP_METHOD    (default: GET)
//   VAULT_HTTP_HEADER    — auth header, e.g. "Authorization: Bearer mytoken"
//   VAULT_HTTP_JSON_PATH — dot-notation path to extract value from JSON response
//                         e.g. "data.value" or "payload.secret"
//   VAULT_HTTP_BODY      — request body template (for POST), use {{path}} and {{key}}
//   VAULT_HTTP_BASE64    — set "true" if the response value is base64-encoded

type HTTPVault struct {
	urlTemplate  string
	method       string
	authHeader   string
	jsonPath     string
	bodyTemplate string
	base64Decode bool
	http         *http.Client
}

func NewHTTPVault(cfg *VaultConfig) (*HTTPVault, error) {
	urlTpl := os.Getenv("VAULT_HTTP_URL")
	if urlTpl == "" {
		return nil, fmt.Errorf("VAULT_HTTP_URL is required (use {{path}} and {{key}} placeholders)")
	}

	return &HTTPVault{
		urlTemplate:  urlTpl,
		method:       envDefault("VAULT_HTTP_METHOD", "GET"),
		authHeader:   os.Getenv("VAULT_HTTP_HEADER"),
		jsonPath:     os.Getenv("VAULT_HTTP_JSON_PATH"),
		bodyTemplate: os.Getenv("VAULT_HTTP_BODY"),
		base64Decode: os.Getenv("VAULT_HTTP_BASE64") == "true",
		http:         &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (v *HTTPVault) LoadSecret(secretPath string, key string) (string, error) {
	url := templateReplace(v.urlTemplate, secretPath, key)

	var bodyReader io.Reader
	if v.bodyTemplate != "" {
		bodyReader = strings.NewReader(templateReplace(v.bodyTemplate, secretPath, key))
	}

	req, err := http.NewRequest(strings.ToUpper(v.method), url, bodyReader)
	if err != nil {
		return "", err
	}
	if v.authHeader != "" {
		parts := strings.SplitN(v.authHeader, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := v.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("http vault request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("http vault HTTP %d: %s", resp.StatusCode, b)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(raw))

	// If a JSON path is specified, parse and traverse
	if v.jsonPath != "" {
		extracted, err := extractJSONPath(raw, v.jsonPath)
		if err != nil {
			return "", err
		}
		result = extracted
	}

	// Base64 decode if configured
	if v.base64Decode {
		decoded, err := base64.StdEncoding.DecodeString(result)
		if err != nil {
			// Try URL-safe base64
			decoded, err = base64.URLEncoding.DecodeString(result)
			if err != nil {
				return "", fmt.Errorf("base64 decode failed: %w", err)
			}
		}
		result = strings.TrimSpace(string(decoded))
	}

	if result == "" {
		return "", fmt.Errorf("http vault returned empty value")
	}
	return result, nil
}

func (v *HTTPVault) StoreSecret(secretPath, key, value string) error {
	return fmt.Errorf("http vault: %w", ErrReadOnly)
}
func (v *HTTPVault) DeleteSecret(secretPath, key string) error {
	return fmt.Errorf("http vault: %w", ErrReadOnly)
}
func (v *HTTPVault) ListKeys(secretPath string) ([]string, error) {
	return nil, fmt.Errorf("http vault: %w", ErrReadOnly)
}
func (v *HTTPVault) Health() error { return nil }
func (v *HTTPVault) String() string { return "http" }

// ===================================================================
// Generic Exec Vault — run any shell command
// ===================================================================
//
// Env vars:
//   VAULT_EXEC_COMMAND (required) — command template, use {{path}} and {{key}}
//                       e.g. "my-tool get-secret {{path}}/{{key}}"
//
// The command's stdout (trimmed) is used as the secret value.

type ExecVault struct {
	commandTemplate string
	commandTokens   []string
}

func NewExecVault(cfg *VaultConfig) (*ExecVault, error) {
	cmdTpl := os.Getenv("VAULT_EXEC_COMMAND")
	if cmdTpl == "" {
		return nil, fmt.Errorf("VAULT_EXEC_COMMAND is required (use {{path}} and {{key}} placeholders)")
	}
	commandTokens, err := parseExecCommandTemplate(cmdTpl)
	if err != nil {
		return nil, err
	}
	return &ExecVault{
		commandTemplate: cmdTpl,
		commandTokens:   commandTokens,
	}, nil
}

func (v *ExecVault) LoadSecret(secretPath string, key string) (string, error) {
	cmdTokens := templateReplaceArgs(v.commandTokens, secretPath, key)
	if len(cmdTokens) == 0 {
		return "", fmt.Errorf("exec command resolved to empty command")
	}
	if err := validateExecBinary(cmdTokens[0], nil); err != nil {
		return "", err
	}
	cmd := exec.Command(cmdTokens[0], cmdTokens[1:]...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("exec: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("exec: %w", err)
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "", fmt.Errorf("exec command returned empty output")
	}
	return result, nil
}

func (v *ExecVault) StoreSecret(secretPath, key, value string) error {
	return fmt.Errorf("exec vault: %w", ErrReadOnly)
}
func (v *ExecVault) DeleteSecret(secretPath, key string) error {
	return fmt.Errorf("exec vault: %w", ErrReadOnly)
}
func (v *ExecVault) ListKeys(secretPath string) ([]string, error) {
	return nil, fmt.Errorf("exec vault: %w", ErrReadOnly)
}
func (v *ExecVault) Health() error  { return nil }
func (v *ExecVault) String() string { return "exec" }

// ===================================================================
// Shared utilities
// ===================================================================

var allowedProviderBinaries = map[string]struct{}{
	"aws":    {},
	"az":     {},
	"gcloud": {},
	"op":     {},
}

// envDefault reads an env var with a fallback.
func envDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// templateReplace substitutes {{path}} and {{key}} in a template string.
func templateReplace(tpl, secretPath, key string) string {
	s := strings.ReplaceAll(tpl, "{{path}}", secretPath)
	s = strings.ReplaceAll(s, "{{key}}", key)
	return s
}

func templateReplaceArgs(args []string, secretPath, key string) []string {
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		resolved = append(resolved, templateReplace(arg, secretPath, key))
	}
	return resolved
}

func parseExecCommandTemplate(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("VAULT_EXEC_COMMAND must not be empty")
	}

	if strings.HasPrefix(trimmed, "[") {
		var tokens []string
		if err := json.Unmarshal([]byte(trimmed), &tokens); err != nil {
			return nil, fmt.Errorf("VAULT_EXEC_COMMAND JSON array is invalid: %w", err)
		}
		if len(tokens) == 0 {
			return nil, fmt.Errorf("VAULT_EXEC_COMMAND JSON array must include a command")
		}
		return tokens, nil
	}

	if strings.ContainsAny(trimmed, "|&;<>()$`\\\n\r'\"") {
		return nil, fmt.Errorf("VAULT_EXEC_COMMAND contains shell metacharacters; use a JSON array such as [\"/usr/bin/tool\",\"get\",\"{{path}}\",\"{{key}}\"]")
	}

	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("VAULT_EXEC_COMMAND must include a command")
	}
	return tokens, nil
}

func validateExecBinary(bin string, allowlist map[string]struct{}) error {
	trimmed := strings.TrimSpace(bin)
	if trimmed == "" {
		return fmt.Errorf("command binary is empty")
	}
	if strings.ContainsAny(trimmed, "|&;<>()$`\\\n\r") {
		return fmt.Errorf("command binary contains invalid shell metacharacters")
	}
	base := filepath.Base(trimmed)
	if allowlist != nil {
		if _, ok := allowlist[base]; !ok {
			return fmt.Errorf("command binary %q is not in the allowed provider list", base)
		}
	}
	if _, err := exec.LookPath(trimmed); err != nil {
		return fmt.Errorf("command binary %q not found in PATH", trimmed)
	}
	return nil
}

// extractJSONPath traverses a JSON object using dot-notation.
// e.g. "data.value" extracts obj["data"]["value"]
func extractJSONPath(raw []byte, path string) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("json parse: %w", err)
	}

	parts := strings.Split(path, ".")
	var current any = obj
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("json path %q: expected object at %q", path, part)
		}
		current, ok = m[part]
		if !ok {
			return "", fmt.Errorf("json path %q: key %q not found", path, part)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}
