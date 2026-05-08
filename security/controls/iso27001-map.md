# ISO 27001 Readiness Mapping

## Disclaimer

This document maps Apig0 practices to selected ISO/IEC 27001 control themes for readiness purposes only. It is not an ISO 27001 certification, audit report, or statement of conformity.

## Evidence References

- Evidence index: `security/evidence/evidence-index.md`
- Policies: `security/policies/`
- Findings: `security/reports/findings.md`
- Risk register: `security/reports/risk-register.md`
- External evidence workflow: `security/integrations/external-security-lab.md`

## Mapping

| ISO 27001 Theme | Apig0-Relevant Controls | Evidence Location | Gaps | Future Work |
| --- | --- | --- | --- | --- |
| Organizational controls | Security policies, incident-response template, vulnerability-management process | `security/policies/` | Policy approval and review history pending | Add dated policy review records |
| Asset management | Product repository structure, security-sensitive component inventory | `README.md`, `security/reports/risk-register.md` | Formal asset register pending | Add maintained asset and data-flow inventory |
| Access control | Admin/user roles, TOTP authentication, gateway token scoping | `auth/`, `config/userstore.go`, `config/apitokens.go` | Periodic access review evidence pending | Create access review checklist and evidence storage |
| Cryptography and secrets | Hashed gateway tokens, encrypted service-secret storage option, master password support | `config/vault.go`, `config/service_secrets.go` | Key-management procedure pending | Document secret rotation and recovery process |
| Secure development | Go tests, security-readiness CI, remediation tracking | `.github/workflows/security-readiness.yml`, `security/scripts/` | Secure design review records pending | Add security design review template |
| Logging and monitoring | Audit logging, monitor middleware, metrics endpoint with auth option | `config/audit.go`, `middleware/monitor.go`, `middleware/prometheus.go` | Centralized alerting evidence pending | Add alerting and log-retention guidance |
| Supplier and tooling management | External validation environment separated from product repository | `security/integrations/` | External lab initialization pending | Maintain third-party tooling attribution and versions |
| Incident management | Lightweight incident-response policy | `security/policies/incident-response-policy.md` | Exercise evidence pending | Run tabletop review and record lessons learned |

## Findings References

No validated ISO 27001-aligned findings are recorded yet. Link future findings and remediation items here when evidence exists.
