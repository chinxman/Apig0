# Penligent.AI Security Audit Notes - 2026-05-06

## Scope

- Target: `192.168.12.192`
- Application: Apig0 Gateway on `https://192.168.12.192:8989`
- Engagement type: external black-box web application assessment
- Tooling source: Penligent.AI free-tier assessment output provided by the project owner
- Status: remediated in the Apig0 repository after review with Codex assistance

## Important Limitations

This report is kept as project security evidence, but it is not a full formal penetration test.

- The assessment was point-in-time and run against one local network target.
- Testing was primarily unauthenticated/black-box.
- The free Penligent.AI run had tool limitations and failures, including `nmap` architecture mismatch and several JavaScript extraction/download attempts that failed before later successful extraction.
- No source-code review was performed by Penligent.AI.
- No denial-of-service, social engineering, physical security, or production data extraction testing was included.
- Some findings are scanner-pattern based and require manual validation after remediation.
- The target used a private IP address, so results may differ behind another reverse proxy, TLS terminator, or deployment network.

## Tools And Checks Observed

Penligent.AI reported use or attempted use of:

- Penligent autonomous scanner
- Nuclei exposure and misconfiguration templates
- httpx service probing
- curl-based endpoint probing
- bash `/dev/tcp` port checks
- JavaScript path extraction and keyword searches
- weak credential testing with common default credentials
- setup endpoint probing
- Prometheus metrics access checks
- HTTP security header enumeration
- username enumeration timing checks
- database dump filename probing
- attempted `nmap` scans, which failed due to binary architecture mismatch

## Positive Results

- Weak default credential tests failed with `401 Unauthorized`.
- The `admin` account locked after repeated failures with `429 Too Many Requests`, confirming lockout behavior.
- Setup interface probes returned `401 Unauthorized`.
- Database dump filename probes returned `401 Unauthorized`.
- JavaScript keyword searches did not confirm hardcoded TOTP secrets, admin tokens, shared keys, or token-generation secrets.
- Username enumeration timing was not confirmed.

## Findings And Remediation

### F-001: Wildcard CORS Policy

- Severity: Medium
- Original status: Not remediated
- Evidence: `Access-Control-Allow-Origin: *` observed on the gateway response
- Remediation:
  - Default CORS behavior now denies cross-origin browser access.
  - `APIG0_CORS_ORIGINS` must explicitly list trusted origins.
  - Wildcard CORS requires explicit `APIG0_CORS_ALLOW_WILDCARD=true` and should not be used for production admin deployments.
  - Denied cross-origin preflight requests now return `403`.

### F-002: Missing HTTP Security Headers

- Severity: Low
- Original status: Not remediated
- Evidence: missing browser hardening headers such as `Cross-Origin-Opener-Policy`
- Remediation:
  - Added security headers middleware with:
    - `Cross-Origin-Opener-Policy: same-origin`
    - `Cross-Origin-Resource-Policy: same-origin`
    - `X-Frame-Options: DENY`
    - `X-Content-Type-Options: nosniff`
    - `Referrer-Policy: strict-origin-when-cross-origin`
    - `Permissions-Policy`
    - `Content-Security-Policy`
    - `Strict-Transport-Security` when TLS/secure-cookie mode is active

### F-003: Public Prometheus Metrics

- Severity: Medium
- Original status: Not remediated
- Evidence: `/metrics` was accessible without authentication
- Remediation:
  - `/metrics` now requires authentication.
  - Access is allowed with an admin browser session.
  - Headless monitoring can use `APIG0_METRICS_TOKEN` with `Authorization: Bearer <token>` or `X-API-Key: <token>`.

## Follow-Up Verification

Recommended retest commands after deployment:

```bash
curl -sk -I https://192.168.12.192:8989/
curl -sk -I -H 'Origin: https://evil.example' https://192.168.12.192:8989/auth/login
curl -sk -i -X OPTIONS -H 'Origin: https://evil.example' https://192.168.12.192:8989/auth/login
curl -sk -i https://192.168.12.192:8989/metrics
curl -sk -H "Authorization: Bearer $APIG0_METRICS_TOKEN" https://192.168.12.192:8989/metrics
```

Expected:

- No default `Access-Control-Allow-Origin: *`.
- Security headers present on gateway responses.
- Unauthenticated `/metrics` returns `401`.
- Authenticated metrics token returns Prometheus text output when `APIG0_METRICS_TOKEN` is configured.
