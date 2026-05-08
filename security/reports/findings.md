# Security Findings

This file records validated or review-supported security-readiness findings for Apig0. Do not add unsupported certification, compliance, audit-pass, or vulnerability-free claims.

## Finding Status Values

- Open
- Patched
- Accepted Risk
- False Positive
- Needs Manual Review

## Severity Values

- Critical
- High
- Medium
- Low
- Informational

## Findings

ID: APIG0-SEC-001
Title: TOTP verification failures did not contribute to account lockout
Severity: Medium
Status: Patched
Affected file/path: `auth/handlers.go`, `auth/lockout.go`, `auth/handlers_test.go`
Evidence: Code review identified password failures recorded by `LoginHandler`, while invalid TOTP codes in `VerifyHandler` returned `401` without calling `RecordFailure`.
Impact: An attacker who obtained a valid short-lived challenge could make repeated TOTP guesses until rate limits or challenge expiry stopped attempts.
Exploitability: Requires valid username/password challenge or compromised challenge token; impact is constrained by challenge lifetime and route rate limiting.
Recommended fix: Check lockout during TOTP verification, record invalid TOTP attempts, and clear failures only after full MFA success.
Patch status: Patched in `auth/handlers.go`; regression tests added in `auth/handlers_test.go`.
Verification: `go test ./...` passed; `go vet ./...` passed.
Residual risk: Account lockout remains in-memory and single-node; distributed deployments need shared lockout state before multi-node operation.

ID: APIG0-SEC-002
Title: Admin service configuration accepted invalid or unsafe upstream URL schemes
Severity: Medium
Status: Patched
Affected file/path: `auth/services_admin.go`, `config/services.go`, `config/services_test.go`
Evidence: Code review identified service base URLs were trimmed and stored without enforcing `http` or `https` scheme and hostname before proxy or test-auth use.
Impact: Invalid schemes could cause runtime failures, and loose URL acceptance weakened SSRF and configuration safety boundaries for administrator-defined upstreams.
Exploitability: Requires admin access; impact depends on configured upstream and deployment network reachability.
Recommended fix: Validate service base URLs before saving and during normalization so only absolute `http` or `https` URLs with hosts are accepted.
Patch status: Patched in `config/services.go` and `auth/services_admin.go`; regression tests added in `config/services_test.go`.
Verification: `go test ./...` passed; `go vet ./...` passed.
Residual risk: Administrators can still configure internal HTTP(S) upstreams by design; production deployments should restrict admin access and network egress.

ID: APIG0-SEC-003
Title: Historical Git secret blocker removed from the local clean repository
Severity: High
Status: Patched
Affected file/path: Historical repository content removed from the local clean repo state
Evidence: `gitleaks detect --source . --redact` now reports no leaks found in the local clean repository, and `security/evidence/scans/gitleaks.txt` reflects the clean result.
Impact: The previously blocking historical Gitleaks issue no longer blocks the local clean repository from internal review.
Exploitability: No currently validated exploitable secret remains in this local clean repo state.
Recommended fix: Preserve the cleaned local history state and continue preventing secret material from entering source control.
Patch status: Patched in the local clean repo state before this final readiness review.
Verification: `gitleaks detect --source . --redact` passed with no findings.
Residual risk: Other external clones or mirrors not covered by this local clean repo evidence may still need separate owner review.

ID: APIG0-SEC-004
Title: Local Go 1.26.2 toolchain had called standard-library vulnerabilities
Severity: High
Status: Patched
Affected file/path: `go.mod`, `.github/workflows/security-readiness.yml`
Evidence: Earlier `govulncheck ./...` runs under local `go1.26.2` reported called vulnerabilities in Go standard-library and `golang.org/x/net` paths. After pinning Go 1.26.3, `govulncheck ./...` reported no vulnerabilities.
Impact: Reported issues included reverse proxy and HTTP/2/network standard-library vulnerabilities reachable through gateway code paths.
Exploitability: Depends on deployment Go toolchain version and exposed routes; affected deployments using vulnerable Go versions should rebuild with Go 1.26.3 or later.
Recommended fix: Pin and use Go 1.26.3 or later in local and CI builds.
Patch status: Patched with `toolchain go1.26.3` in `go.mod` and CI `go-version: "1.26.3"`.
Verification: `govulncheck ./...` passed with no vulnerabilities.
Residual risk: Operators building with older local toolchains outside module toolchain auto-download remain responsible for upgrading.

ID: APIG0-INFO-001
Title: Security-readiness architecture initialized
Severity: Informational
Status: Patched
Affected file/path: `AGENTS.md`, `.codex/agents/`, `security/`, `.github/workflows/security-readiness.yml`, `.gitignore`
Evidence: Required architecture files were created or updated in this package.
Impact: Provides repeatable internal validation and audit-preparation evidence without claiming formal certification.
Exploitability: Not applicable.
Recommended fix: Continue running scans, resolving findings, and updating the readiness artifacts.
Patch status: Patched
Verification: Required files exist; tests and scan evidence are present under `security/evidence/`.
Residual risk: Remaining Semgrep items require ongoing hardening or risk acceptance tracking.

ID: APIG0-LIVE-001
Title: Live target requires HTTPS
Severity: Informational
Status: Patched
Affected file/path: `security/scope.md`, `security/evidence/scans/live-http-root-headers-20260507T213454Z.txt`, `security/evidence/scans/live-https-root-headers-20260507T213454Z.txt`, `security/evidence/scans/nmap-192-168-12-192-8989-20260507T213454Z.txt`
Evidence: Nmap identified `8989/tcp open ssl/http Golang net/http server`. Plain HTTP requests returned `HTTP/1.0 400 Bad Request` with body `Client sent an HTTP request to an HTTPS server.` HTTPS requests returned `HTTP/2 200 OK` for `/` and `/healthz`.
Impact: The target must be tested over TLS to evaluate the actual Apig0 application surface.
Exploitability: Not a vulnerability by itself; it was a validation scope/protocol mismatch that is now documented.
Recommended fix: Use `https://192.168.12.192:8989` for application-layer validation.
Patch status: Scope and evidence updated; HTTPS validation completed.
Verification: `curl`, nmap, and a low-rate nuclei baseline subset completed against the authorized HTTPS URL/port.
Residual risk: ZAP baseline remains unavailable locally.
