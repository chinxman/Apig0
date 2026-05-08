# OWASP API Top 10 Readiness Mapping

## Disclaimer

This document maps Apig0 security controls to OWASP API Top 10 themes for readiness and testing support. It is not proof that Apig0 has no OWASP API Top 10 risks.

## Evidence References

- Evidence index: `security/evidence/evidence-index.md`
- Pentest report: `security/reports/pentest-report.md`
- Findings: `security/reports/findings.md`
- Local scans: `security/scans/`
- External lab integration: `security/integrations/external-security-lab.md`

## Mapping

| OWASP API Top 10 2023 | Apig0-Relevant Controls | Evidence Location | Gaps | Future Work |
| --- | --- | --- | --- | --- |
| API1: Broken Object Level Authorization | User/service scoping and access policy enforcement for proxied routes | `config/access_policies.go`, `auth/`, `proxy/` | Route authorization test evidence pending | Add endpoint-level authorization test matrix |
| API2: Broken Authentication | Password + TOTP login, sessions, lockout, gateway token hashing | `auth/`, `config/apitokens.go` | Negative auth testing pending | Add brute-force, replay, and token lifecycle tests |
| API3: Broken Object Property Level Authorization | Admin service/user/token handlers separate management data from portal data | `auth/admin.go`, `auth/services_admin.go` | Property-level abuse tests pending | Add request body and response field review |
| API4: Unrestricted Resource Consumption | Rate limiting, timeout, retry controls | `middleware/ratelimit.go`, `proxy/proxy.go` | Load and abuse test evidence pending | Add resource-consumption scenarios in external lab |
| API5: Broken Function Level Authorization | Admin route grouping and role checks | `auth/gateway_admin.go`, `auth/handlers.go` | Function authorization matrix pending | Add tests for admin/user role boundaries |
| API6: Unrestricted Access to Sensitive Business Flows | Token creation, one-time key claim, setup/bootstrap flows | `auth/token_delivery.go`, `auth/setup.go` | Abuse-case testing pending | Add flow-specific controls and retest evidence |
| API7: Server-Side Request Forgery | Upstream service configuration and reverse proxy behavior | `config/services.go`, `proxy/proxy.go` | SSRF validation pending | Add upstream URL allowlist guidance and tests |
| API8: Security Misconfiguration | CORS defaults, trusted proxy configuration, metrics auth option, secure headers middleware | `middleware/cors.go`, `middleware/security_headers.go`, `README.md` | Deployment hardening evidence pending | Add production configuration checklist |
| API9: Improper Inventory Management | Route map and package layout documented in README | `README.md` | API inventory automation pending | Generate route inventory from code or tests |
| API10: Unsafe Consumption of APIs | Upstream auth injection, timeout/retry handling, AI backend scoping | `proxy/`, `config/services.go` | Upstream failure-mode evidence pending | Add tests for upstream errors and secret leakage |

## Findings References

No validated OWASP API Top 10 findings are recorded yet. Add finding IDs after security validation.
