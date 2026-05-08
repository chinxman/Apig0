# OWASP ASVS Readiness Mapping

## Disclaimer

This document provides an OWASP ASVS-aligned readiness mapping for Apig0. It is not a formal ASVS verification result and does not assert complete coverage.

## Evidence References

- Evidence index: `security/evidence/evidence-index.md`
- Findings: `security/reports/findings.md`
- Remediation plan: `security/reports/remediation-plan.md`
- Local scan outputs: `security/scans/`

## Mapping

| ASVS Area | Apig0-Relevant Controls | Evidence Location | Gaps | Future Work |
| --- | --- | --- | --- | --- |
| V1 Architecture, Design and Threat Modeling | Gateway architecture documented through route map and package layout | `README.md` | Threat model pending | Add trust boundaries and abuse cases |
| V2 Authentication | Password + TOTP login and lockout support | `auth/totp.go`, `auth/lockout.go` | Full negative test evidence pending | Add TOTP reset and replay test cases |
| V3 Session Management | Browser session handlers and logout flow | `auth/session.go`, `auth/handlers.go` | Session cookie review pending | Add cookie attribute and timeout validation |
| V4 Access Control | Admin/user route boundaries and service-scoped tokens | `auth/`, `config/access_policies.go` | Authorization matrix pending | Add role and service boundary tests |
| V5 Validation, Sanitization and Encoding | Request handling through Gin handlers and proxy routing | `auth/`, `proxy/` | Input validation evidence pending | Add request fuzzing and malformed input cases |
| V7 Error Handling and Logging | Audit records and monitor middleware | `config/audit.go`, `middleware/monitor.go` | Sensitive log review pending | Add log redaction tests |
| V8 Data Protection | Token hashing and service-secret storage modes | `config/apitokens.go`, `config/vault.go` | Secret rotation evidence pending | Add key lifecycle procedure |
| V9 Communications | TLS mode support and reverse proxy behavior | `config/tls.go`, `proxy/` | TLS configuration evidence pending | Add TLS deployment guidance and checks |
| V10 Malicious Code | Dependency and static checks supported by scripts when tooling is installed | `security/scripts/run-local-security-checks.sh` | Tool outputs pending | Add generated scan outputs |
| V11 Business Logic | Token delivery, setup bootstrap, admin service management | `auth/token_delivery.go`, `auth/setup.go`, `auth/services_admin.go` | Abuse-case validation pending | Add business-flow security tests |
| V14 Configuration | YAML/environment configuration, CORS, trusted proxy, metrics token | `apig0.yaml`, `middleware/cors.go`, `README.md` | Production hardening checklist pending | Add configuration baseline and deployment guide |

## Findings References

No validated ASVS-aligned findings are recorded yet. Link future findings and retest evidence here.
