# Gateway Hardening Pass

Date: `2026-04-18`

This pass adds the first serious set of features from the Apig0 strategy review so the project moves closer to an identity-aware internal API gateway instead of staying a browser-only secure proxy.

## Scope

This implementation pass focused on:
- machine-friendly authentication
- path and method access policy
- audit logging with decision reasons
- secret lifecycle metadata
- upstream timeout and retry controls
- Prometheus-style metrics exposure
- admin UI and admin API support for the above
- runtime support so temporary mode and persistent mode both handle the new state cleanly

## Added In This Pass

### 1. Machine auth path

Added scoped API tokens for non-browser access.

Included behavior:
- token creation through admin APIs
- token revocation
- expiry support
- last-used tracking
- per-token allowed service scoping
- token-aware auth source tracking

Main files:
- `config/apitokens.go`
- `auth/session.go`
- `auth/gateway_admin.go`
- `main.go`

Practical result:
- browser users can keep using cookie sessions
- API clients and automation can authenticate without the browser challenge flow

### 2. Path and method policy engine

Added per-user route policy rules evaluated before proxying.

Included behavior:
- per-service route rules
- path-prefix matching
- method allow and deny behavior
- explicit deny reasons

Main files:
- `config/access_policies.go`
- `main.go`

Practical result:
- access is no longer limited to all-or-nothing service-level allowlists

### 3. Audit log with decision reasoning

Added durable audit logging and recent in-memory audit state.

Included behavior:
- allow and deny event recording
- reason strings for denied access
- service-not-found recording
- admin API retrieval
- monitor page visibility

Main files:
- `config/audit.go`
- `auth/gateway_admin.go`
- `main.go`
- `webui.html`

Practical result:
- operator activity and access decisions are no longer limited to the live monitor feed

### 4. Secret lifecycle metadata

Added metadata around service credentials instead of treating stored upstream auth as a blind value.

Included behavior:
- notes
- expiry timestamp
- test status
- last-tested tracking

Main files:
- `config/service_secret_metadata.go`
- `auth/services_admin.go`
- `webui.html`

Practical result:
- upstream credentials can be managed with more operational context

### 5. Upstream timeout and retry controls

Added per-service request timeout and retry settings.

Included behavior:
- configurable timeout per service
- safe retries for proxy requests
- admin service auth test path

Main files:
- `config/services.go`
- `proxy/proxy.go`
- `auth/services_admin.go`

Practical result:
- upstream failures are less brittle
- operators can validate auth wiring before relying on a service

### 6. Prometheus-style metrics

Added a `/metrics` surface and monitor snapshot exposure improvements.

Included behavior:
- Prometheus-style text output
- counters for core gateway behavior
- compatibility with external monitoring systems

Main files:
- `middleware/prometheus.go`
- `middleware/monitor.go`
- `main.go`

Practical result:
- Apig0 can feed standard monitoring workflows instead of depending only on the built-in dashboard

### 7. Admin UI and API expansion

Added admin-facing controls so these features are not setup-only or file-only.

Included behavior:
- token management
- policy editing
- audit trail viewing
- secret metadata editing
- service auth testing
- timeout and retry editing

Main files:
- `auth/gateway_admin.go`
- `auth/services_admin.go`
- `webui.html`

Practical result:
- the new gateway features are available after setup, not only during bootstrap

## Temporary Mode And Persistent Mode

This pass was intentionally wired so both runtime modes remain useful.

Temporary mode:
- still works for fast setup and testing
- clears the new feature state during reset

Persistent mode:
- loads and saves the new token, policy, secret metadata, and audit-related files
- keeps these additions available after setup completion

Supporting runtime files:
- `config/runtime_reload.go`
- `config/runtime_status.go`
- `config/setup_runtime.go`

## What This Pass Does Not Include

Not implemented in this pass:
- OIDC integration
- shared-state backend for multi-node deployments
- polished policy UI beyond the current admin editing flow
- advanced health checks or full circuit-breaker behavior

## Operational Notes

- The gateway is still best treated as a single-node appliance unless and until shared state is added for sessions, lockouts, rate limits, TOTP replay tracking, and other in-memory security state.
- The browser flow remains important, but Apig0 now has a real non-browser auth path.
- The route policy model is intentionally minimal and should be proven before becoming more abstract.

## Verification Status

No build or test commands were run as part of this pass.

That means this document records the implementation intent and repo changes, but local verification should still be done in the normal environment.
