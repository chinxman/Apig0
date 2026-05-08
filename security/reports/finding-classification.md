# Finding Classification

Generated: 2026-05-08

This document classifies the eight remaining Semgrep findings in `security/evidence/scans/semgrep.json` for the local clean Apig0 repository. This is an internal security-readiness artifact and does not claim formal certification, independent audit approval, or absence of all vulnerabilities.

## Remaining Semgrep Findings

| # | Rule | Severity | Affected path | Classification | Rationale | Recommended follow-up |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `cookie-missing-secure` | Warning | `auth/session.go:282` | Needs future hardening | The session cookie already sets `HttpOnly` and `SameSite=Strict`. `Secure` is controlled by `auth.IsSecure()` so Semgrep cannot prove it statically. The remaining risk is that a plain-HTTP deployment can still emit a non-secure session cookie. | Consider enforcing secure cookies whenever session auth is enabled, or explicitly limiting plain-HTTP mode to local development. |
| 2 | `cookie-missing-secure` | Warning | `auth/session.go:295` | Needs future hardening | The session-clear path mirrors the live session cookie behavior and inherits the same configuration-dependent `Secure` handling. | Keep this aligned with any future session-cookie hardening decision. |
| 3 | `dangerous-exec-command` | Error | `cli/cli.go:124` | False positive | The command path comes from `os.Executable()`, is validated by `validateBackgroundExecutable`, and is invoked as `exec.Command(exe, "serve")` without shell expansion or request-controlled input. | No immediate patch required. Revisit only if the background launcher starts accepting externally supplied executable paths. |
| 4 | `dangerous-exec-command` | Error | `config/vault_providers.go:227` | Accepted risk | This provider intentionally shells out to supported secret-provider CLIs, but the binary is restricted by `allowedProviderBinaries` and executed without `sh -c`. The execution surface is still an operator-enabled integration point. | Keep provider CLI usage limited to trusted operator-managed deployments and document it as an administrative extension point. |
| 5 | `dangerous-exec-command` | Error | `config/vault_providers.go:246` | Accepted risk | This is the health-check variant of the same provider CLI integration. The binary is allowlisted and executed directly, but the design still intentionally relies on external command execution. | Same treatment as finding 4. |
| 6 | `dangerous-exec-command` | Error | `config/vault_providers.go:668` | Accepted risk | The generic exec vault remains intentionally capable of executing an operator-configured command. Current hardening removes shell execution, rejects shell metacharacters, tokenizes the template, and validates the binary, but the feature still executes local programs by design. | Preserve as a documented operator-only feature, or disable it in stricter production profiles if the deployment does not need it. |
| 7 | `cookie-missing-httponly` | Warning | `middleware/csrf.go:29` | False positive | The CSRF cookie is intentionally readable by browser JavaScript because the middleware uses the double-submit pattern and requires the client to copy the token into `X-CSRF-Token`. | No patch recommended unless the CSRF design changes away from double-submit cookies. |
| 8 | `cookie-missing-secure` | Warning | `middleware/csrf.go:29` | Needs future hardening | The CSRF cookie already uses `SameSite=Strict` and toggles `Secure` through `auth.IsSecure()`. Semgrep cannot see that runtime control, but the residual risk is still real for plain-HTTP deployments. | Consider forcing secure-mode CSRF cookies outside explicit local development flows. |

## Summary By Classification

- `False positive`: 2
- `Accepted risk`: 3
- `Needs future hardening`: 3
- `Fixed`: 0

## Readiness Notes

- `gitleaks`: clean in the local clean repo scan output (`security/evidence/scans/gitleaks.txt`)
- `grype`: clean per current evidence state provided for this run
- `govulncheck`: clean (`security/evidence/scans/govulncheck.txt`)
- `go test ./...`: pass
- `go vet ./...`: pass

The remaining Semgrep items are now classified and documented. None of the eight remaining findings is a current confirmed high- or critical-severity exploitable issue in this local clean repository state, but the cookie behavior in plain-HTTP mode and the operator-enabled exec integrations remain important hardening considerations.
