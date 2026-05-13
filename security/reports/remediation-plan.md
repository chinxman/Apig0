# Remediation Plan

| Finding ID | Fix | Priority | Owner | Due Date | Status | Retest Result |
| --- | --- | --- | --- | --- | --- | --- |
| APIG0-SEC-001 | Record invalid TOTP attempts in account lockout and clear failures after full MFA success | Medium | Pending | Complete | Patched | `go test ./...` passed |
| APIG0-SEC-002 | Enforce absolute `http` or `https` upstream service URLs with hosts and no embedded credentials/fragments | Medium | Pending | Complete | Patched | `go test ./...` passed |
| APIG0-SEC-003 | Preserve the cleaned local repository state and continue secret-scanning in CI and local review flows | High | Pending owner | Ongoing | Patched | `gitleaks detect --source . --redact` passed with no findings |
| APIG0-SEC-004 | Pin local module toolchain and CI to Go 1.26.3 | High | Pending | Complete | Patched | `govulncheck ./...` passed |
| APIG0-LIVE-002 | Validate live `orders` service `base_url`, upstream reachability, and TLS trust from gateway host; enable `tls_skip_verify` only if upstream intentionally uses self-signed/private CA TLS | Medium | Pending owner | Pending | Needs Manual Review | Scoped curl evidence shows gateway-generated `502 upstream unavailable` for both `/orders` and `/orders/`; repo now logs transport errors and supports opt-in upstream TLS skip verify |

## Remediation Rules

- Link each fix to a finding, risk, issue, or pull request.
- Preserve evidence for the original issue and the retest.
- Do not close a remediation item until retest evidence exists or risk acceptance is documented.
- Do not rewrite Git history or claim remediation outside the local clean repo evidence without owner approval and evidence.
