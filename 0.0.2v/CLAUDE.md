# Apig0 — Project Reference

## What it is
A session-authenticated API gateway in Go (Gin). Sits in front of backend services,
uses password + TOTP two-factor login to establish sessions, and streams live metrics
to a built-in dashboard.

## Module
```
module apig0
go 1.25.0
```

## Key files
```
main.go                     — entry point, wires everything together
apig0.yaml                  — persistent config (vault type, port, session TTL, etc.)
auth/totp.go                — ValidateTOTP() with anti-replay, QR code printing
auth/session.go             — session + challenge store, SessionMiddleware, configurable TTL
auth/handlers.go            — POST /auth/login, /auth/verify, /auth/logout
middleware/cors.go           — CORS headers
middleware/monitor.go        — request capture, SSE broadcaster, ring buffer (500 events)
proxy/proxy.go               — httputil reverse proxy wrapper
config/appconfig.go          — loads apig0.yaml, sets env var defaults
config/secrets.go            — UserSecrets, UserPasswords, InitSecrets()
config/vault.go              — VaultInterface, LoadVaultConfig, CreateVault, activeVault
config/vault_providers.go    — all provider implementations (see below)
config/vault.yaml            — full reference doc for vault configuration
dashboard.html               — live monitoring SPA with login overlay, served at /dashboard
notes/known-behaviors.md     — documented edge cases and planned changes
secret-creator/              — standalone TOTP secret + QR generator (own go.mod)
```

## URL namespace
```
GET  /healthz              — health check, no auth
GET  /dashboard            — monitoring SPA (login overlay gates access)
POST /auth/login           — step 1: username + password → challenge token
POST /auth/verify          — step 2: challenge + TOTP code → session cookie
POST /auth/logout          — clears session cookie + server-side session
GET  /api/admin/events     — SSE stream of request events, session required
GET  /api/admin/stats      — JSON metrics snapshot, session required
ANY  /{service}/*          — reverse proxy to backend, session required
```

## Auth model
- Two-factor login: password (bcrypt) → TOTP (anti-replay, 90s window)
- Session cookie `apig0_session` set on successful login
- Session TTL configurable via `APIG0_SESSION_TTL` (default: 8h)
- Proxy + admin endpoints use SessionMiddleware (cookie-based)
- No per-request TOTP headers — login once, session persists

## Vault backends (VAULT_TYPE=)
```
env          — APIG0_TOTP_SECRET_<USER> env vars (default, no server needed)
hashicorp    — HashiCorp Vault KV v2, token or AppRole auth
aws          — AWS Secrets Manager via aws CLI
gcp          — GCP Secret Manager via gcloud CLI
azure        — Azure Key Vault via az CLI
cyberark     — CyberArk CCP REST API
1password    — 1Password via op CLI
http         — any REST API (VAULT_HTTP_URL with {{path}}/{{key}} placeholders)
exec         — any shell command (VAULT_EXEC_COMMAND with {{path}}/{{key}})
```
No secrets are ever hardcoded. InitSecrets() logs a warning and refuses to
use defaults if nothing is configured.

## Adding a new vault provider
1. Implement VaultInterface in config/vault_providers.go
2. Add constructor NewXxxVault(cfg *VaultConfig)
3. Register in CreateVault() switch in config/vault.go

## Adding a new backend service
In main.go, add to the services map:
```go
services := map[string]string{
    "users":    "http://192.168.12.11:3001",
    "products": "http://192.168.12.11:3002",
    "orders":   "http://192.168.12.11:3003",
    "mynewsvc": "http://192.168.12.11:3004",  // add here
}
```
The monitor RegisterService call and proxy handler are built automatically.

## Monitor / dashboard
- Middleware order: CORS → Monitor (global), then SessionMiddleware per-route
- SSE sends a "snapshot" event on connect (recent events + current stats), then "request" events live
- Service cards appear immediately at startup (RegisterService pre-registers them)
- Monitor reads user from "session_user" context key set by SessionMiddleware

## Configuration
Settings in `apig0.yaml` persist across restarts. Env vars override YAML values.

### apig0.yaml
```yaml
vault:
  type: hashicorp
  address: http://192.168.12.10:8200
  engine: secret
gateway:
  users: devin
  port: "8080"
  session_ttl: 8h
```

### Environment variables (override YAML, planned for removal for non-secrets)
```
VAULT_TYPE                  vault backend selector (default: env)
VAULT_ADDRESS               hashicorp vault address
VAULT_ENGINE                hashicorp KV engine name
APIG0_USERS                 comma-separated user list (default: devin)
APIG0_TOTP_SECRET_<USER>    per-user TOTP secret when VAULT_TYPE=env
APIG0_TOTP_SECRET           single-user fallback when VAULT_TYPE=env
APIG0_PASSWORD_<USER>       per-user password (plaintext, bcrypt-hashed at startup)
APIG0_SESSION_TTL           session duration (Go duration string, default: 8h)
APIG0_SHOW_QR               print QR code to stdout on startup (true/false)
APIG0_PORT                  gateway port (default: 8080)
APIG0_TRUSTED_PROXIES       comma-separated trusted proxy IPs/CIDRs
APIG0_CORS_ORIGINS          comma-separated allowed origins
```

## Running for dev/test
```bash
export VAULT_TYPE=env
export APIG0_TOTP_SECRET_DEVIN=YOUR_BASE32_SECRET
export APIG0_PASSWORD_DEVIN=yourpassword
export APIG0_USERS=devin
go run .
# Dashboard: http://localhost:8080/dashboard
```
