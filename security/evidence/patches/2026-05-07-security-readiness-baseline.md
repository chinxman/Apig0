# 2026-05-07 Security-Readiness Baseline Patch Evidence

## Metadata

| Field | Value |
| --- | --- |
| Date/time | 2026-05-07T20:54:34Z |
| Branch | master |
| Commit hash | 357987669e6e082c955a15208b31297fa5a1d786 |
| Operator | Codex |
| Environment | Local workspace `/Users/chinxman/Documents/apig0` |

## Patches

| Finding | Files changed | Security impact | Verification |
| --- | --- | --- | --- |
| APIG0-SEC-001 | `auth/handlers.go`, `auth/handlers_test.go` | Invalid TOTP attempts now call `RecordFailure`, locked users are rejected during verification, and failed-attempt state clears only after full MFA success | `go test ./...`, `go vet ./...` |
| APIG0-SEC-002 | `config/services.go`, `auth/services_admin.go`, `config/services_test.go` | Admin-defined upstream base URLs must be absolute `http` or `https` URLs with a host, no embedded credentials, and no fragments | `go test ./...`, `go vet ./...` |
| APIG0-SEC-004 | `go.mod`, `.github/workflows/security-readiness.yml` | Repository and CI now pin Go 1.26.3 to avoid called Go standard-library vulnerabilities reported by govulncheck under go1.26.2 | `govulncheck ./...` retest passed |
| APIG0-INFO-001 | `AGENTS.md`, `.codex/agents/*.toml`, `security/`, `.github/workflows/security-readiness.yml`, `.gitignore` | Project-scoped readiness architecture, evidence structure, workflow, and tracked Codex agents were added | File presence and `git status` review |

## Unpatched Or Manual Items

| Finding | Reason | Required owner action |
| --- | --- | --- |
| APIG0-SEC-003 | Gitleaks found historical redacted secret matches in Git history. History rewrite, credential rotation, and revocation decisions require owner approval. | Verify whether the historical values were real, rotate/revoke if needed, and decide whether to purge or formally accept the historical Git risk. |
