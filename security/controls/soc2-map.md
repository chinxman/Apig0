# SOC 2 Readiness Mapping

## Disclaimer

This document is a security-readiness mapping for Apig0. It is not a SOC 2 certification, not a SOC 2 report, and not an audit opinion. It is intended to organize evidence and gaps for future review.

## Evidence References

- Evidence index: `security/evidence/evidence-index.md`
- Command history: `security/evidence/commands-run.md`
- Findings: `security/reports/findings.md`
- Remediation plan: `security/reports/remediation-plan.md`
- Local scan outputs: `security/scans/`

## Mapping

| SOC 2 Area | Apig0-Relevant Controls | Evidence Location | Gaps | Future Work |
| --- | --- | --- | --- | --- |
| Security - Logical Access | Password and TOTP browser login, gateway token hashing, user/service scoping, admin-only management routes | `auth/`, `config/apitokens.go`, `security/evidence/` | Formal access review evidence is pending | Add periodic access review records and admin account review checklist |
| Security - Authentication | TOTP setup and verification, session handlers, lockout behavior | `auth/totp.go`, `auth/session.go`, `auth/lockout.go` | Full authentication test evidence is pending | Add auth-focused test evidence and retest notes |
| Security - Change Management | Go tests, security-readiness workflow, remediation tracking templates | `.github/workflows/security-readiness.yml`, `security/reports/remediation-plan.md` | Pull request security review evidence is pending | Add PR checklist and finding-to-fix traceability |
| Security - Monitoring | Prometheus-style metrics, audit records, monitor middleware | `middleware/monitor.go`, `middleware/prometheus.go`, `config/audit.go` | Alerting evidence is pending | Document monitoring thresholds and incident triggers |
| Security - Vulnerability Management | Local scripts for tests, vulnerability checks, static analysis, secret scanning when tools are installed | `security/scripts/run-local-security-checks.sh`, `security/scans/` | External validation pending | Import validated outputs from `apig0-security-lab` |
| Availability | Health endpoint, timeout and retry support, rate limiting | `main.go`, `proxy/`, `middleware/ratelimit.go` | Load and resilience evidence is pending | Add controlled performance and failure-mode tests |
| Confidentiality | Service secret storage modes, encrypted persistent mode option, token one-time delivery model | `config/service_secrets.go`, `config/vault.go`, `auth/token_delivery.go` | Secret-rotation evidence is pending | Add secret-handling validation and rotation runbook |

## Findings References

No validated SOC 2-related findings are recorded yet. Add finding IDs from `security/reports/findings.md` as evidence becomes available.
