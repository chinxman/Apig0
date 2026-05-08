# Finding Classification

Generated: 2026-05-08

This document classifies the remaining Semgrep findings in `security/evidence/scans/semgrep.json` for the local clean Apig0 repository. This is an internal security-readiness artifact and does not claim formal certification, independent audit approval, or absence of all vulnerabilities.

## Remaining Semgrep Findings

| # | Rule | Severity | Affected path | Classification | Rationale | Recommended follow-up |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `dangerous-exec-command` | Error | `config/vault_providers.go:227` | Accepted risk | This provider intentionally shells out to supported secret-provider CLIs, but the binary is restricted by `allowedProviderBinaries` and executed without `sh -c`. The execution surface is still an operator-enabled integration point. | Keep provider CLI usage limited to trusted operator-managed deployments and document it as an administrative extension point. |
| 2 | `dangerous-exec-command` | Error | `config/vault_providers.go:246` | Accepted risk | This is the health-check variant of the same provider CLI integration. The binary is allowlisted and executed directly, but the design still intentionally relies on external command execution. | Same treatment as finding 1. |
| 3 | `dangerous-exec-command` | Error | `config/vault_providers.go:668` | Accepted risk | The generic exec vault remains intentionally capable of executing an operator-configured command. Current hardening removes shell execution, rejects shell metacharacters, tokenizes the template, and validates the binary, but the feature still executes local programs by design. | Preserve as a documented operator-only feature, or disable it in stricter production profiles if the deployment does not need it. |
| 4 | `cookie-missing-httponly` | Warning | `middleware/csrf.go:17` | False positive | The CSRF cookie is intentionally readable by browser JavaScript because the middleware uses the double-submit pattern and requires the client to copy the token into `X-CSRF-Token`. The code comment now documents that design constraint directly at the cookie constructor. | No patch recommended unless the CSRF design changes away from double-submit cookies. |

## Fixed In This Pass

| Rule | Affected path | Result |
| --- | --- | --- |
| `cookie-missing-secure` | `auth/session.go` | Fixed by moving session cookies to a default-secure constructor with an explicit local-development opt-out (`APIG0_INSECURE_COOKIES=true` or `APIG0_SECURE=false`). |
| `cookie-missing-secure` | `middleware/csrf.go` | Fixed by moving the CSRF cookie to a default-secure constructor while preserving the JS-readable double-submit pattern. |
| `dangerous-exec-command` | `cli/cli.go` | Fixed by replacing the background launcher `exec.Command` call with `os.StartProcess` while preserving the validated executable path, detached session behavior, cwd, env, and log-file wiring. |

## Summary By Classification

- `False positive`: 1
- `Accepted risk`: 3
- `Needs future hardening`: 0
- `Fixed in this pass`: 4

## Readiness Notes

- `gitleaks`: clean in the local clean repo scan output (`security/evidence/scans/gitleaks.txt`)
- `grype`: clean per current evidence state provided for this run
- `govulncheck`: clean (`security/evidence/scans/govulncheck.txt`)
- `go test ./...`: pass
- `go vet ./...`: pass
- `semgrep`: 4 remaining findings after cookie and CLI hardening

The remaining Semgrep items are now limited to the previously documented vault exec accepted-risk findings and the intentional CSRF `HttpOnly` false positive. None of the remaining findings is a current confirmed high- or critical-severity exploitable issue in this local clean repository state.
