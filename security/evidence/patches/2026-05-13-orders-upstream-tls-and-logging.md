# Patch Note: Orders Upstream TLS and Proxy Diagnostics

## Metadata

| Field | Value |
| --- | --- |
| Timestamp | 2026-05-13T03:20:00Z |
| Branch | security-readiness-evidence-pass |
| Commit hash | 357987669e6e082c955a15208b31297fa5a1d786 |
| Operator | Codex |

## Summary

Added an opt-in per-service `tls_skip_verify` flag so Apig0 can proxy to internal HTTPS upstreams that use self-signed or privately issued certificates when operators explicitly allow it. Added proxy transport error logging for both standard and OpenAI-compatible proxy paths so live `502 upstream unavailable` events become diagnosable from gateway logs.

## Files Changed

- `config/services.go`
- `auth/services_admin.go`
- `proxy/proxy.go`
- `proxy/openai.go`
- `webui.html`
- `static/js/admin-services.js`
- `config/services_test.go`
- `proxy/proxy_test.go`

## Validation

- Focused tests passed:
  - `go test ./proxy ./config -count=1`
  - `go vet ./...`
- Full `go test ./... -count=1` remained blocked by current repository module-state issues reporting missing `go.sum` entries outside this patch scope.

## Residual Risk

This patch adds capability and observability, but it does not change the already-running live `orders` configuration on `192.168.12.192:8989`. The live target still requires operator action to enable `tls_skip_verify` for `orders` if the upstream certificate is not publicly trusted, or to correct another upstream reachability issue if logs show a different transport error.
