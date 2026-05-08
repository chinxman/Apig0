# NIST CSF Readiness Mapping

## Disclaimer

This is a readiness and alignment mapping to the NIST Cybersecurity Framework. It is not a formal assessment outcome and does not claim complete implementation of NIST CSF.

## Evidence References

- Evidence index: `security/evidence/evidence-index.md`
- Risk register: `security/reports/risk-register.md`
- Remediation plan: `security/reports/remediation-plan.md`
- Policies: `security/policies/`

## Mapping

| NIST CSF Function | Apig0-Relevant Controls | Evidence Location | Gaps | Future Work |
| --- | --- | --- | --- | --- |
| Identify | Repository layout, route map, security-sensitive component list, risk-register template | `README.md`, `security/reports/risk-register.md` | Data-flow diagram pending | Add gateway trust-boundary and data-flow documentation |
| Protect | TOTP, token hashing, access scoping, rate limiting, CORS controls, secure headers middleware | `auth/`, `config/`, `middleware/` | Full configuration hardening checklist pending | Add deployment hardening guide |
| Detect | Audit records, monitor middleware, metrics endpoint, security scan outputs | `config/audit.go`, `middleware/monitor.go`, `security/scans/` | Alerting and detection tests pending | Add detection use cases and log-review procedure |
| Respond | Incident-response policy and remediation plan template | `security/policies/incident-response-policy.md`, `security/reports/remediation-plan.md` | Incident exercise evidence pending | Add incident tabletop and post-incident review template |
| Recover | Runtime reset and bootstrap admin flows, policy guidance | `config/setup_runtime.go`, `auth/setup.go`, `security/policies/incident-response-policy.md` | Backup and restore evidence pending | Document recovery objectives and restore validation |
| Govern | Root agent instructions, security policies, evidence workflow | `AGENTS.md`, `security/AGENT.md`, `security/policies/` | Ownership review pending | Assign owners and review cadence |

## Findings References

No validated NIST CSF-aligned findings are recorded yet. Add references after validation.
