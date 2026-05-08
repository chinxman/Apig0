# Security Readiness Report

## Metadata

| Field | Value |
| --- | --- |
| Project | Apig0 |
| Report type | Internal security-readiness report |
| Date/time | 2026-05-08T21:45:00Z |
| Branch | security-readiness-evidence-pass |
| Commit hash | 27b82f0c88ef1271148755223bfb883a19d87328 |
| Operator | Codex |
| Environment | Local workspace `/Users/chinxman/Code/apig0-clean` |

## Scope

Included:

- Repository source code.
- Go tests and vetting.
- Current scan evidence under `security/evidence/`.
- Security-readiness documentation and CI workflow.

Excluded:

- Public or third-party targets.
- Production systems.
- Staging systems not explicitly authorized in `security/scope.md`.
- Destructive pentesting.
- External certification or legal compliance validation.

## Tools And Evidence

| Tool | Purpose | Status | Evidence |
| --- | --- | --- | --- |
| Go test | Native tests | Passed | `security/evidence/scans/go-test.txt` |
| Go vet | Native static checks | Passed | `security/evidence/scans/go-vet.txt` |
| govulncheck | Go vulnerability checks | Passed | `security/evidence/scans/govulncheck.txt` |
| gitleaks | Secret scanning | Passed; no leaks found | `security/evidence/scans/gitleaks.txt`, `security/evidence/scans/gitleaks.sarif` |
| semgrep | SAST | Completed; 8 findings remain | `security/evidence/scans/semgrep.json`, `security/reports/finding-classification.md` |
| trivy | Filesystem/dependency scan | Completed | `security/evidence/scans/trivy-fs.json` |
| syft | SBOM generation | Completed | `security/evidence/sbom/syft-spdx.json` |
| grype | Vulnerability matching | Passed; `0` matches | `security/evidence/scans/grype.json` |
| curl | Authorized HTTP and HTTPS checks | Completed | `security/evidence/scans/live-http-*20260507T213454Z.*`, `security/evidence/scans/live-https-*20260507T213454Z.*` |
| nmap | Authorized service/version check on port `8989` | Completed | `security/evidence/scans/nmap-192-168-12-192-8989-20260507T213454Z.txt` |
| nuclei | Authorized low-rate baseline subset | Completed | `security/evidence/scans/nuclei-baseline-192-168-12-192-8989-20260507T213454Z.jsonl`, `security/evidence/scans/nuclei-https-baseline-192-168-12-192-8989-20260507T213454Z.jsonl` |
| OWASP ZAP baseline | Local/scoped web/API baseline | Skipped | ZAP baseline runtime unavailable locally |

## Findings Summary

| Severity | Count | Notes |
| --- | ---: | --- |
| Critical | 0 | None validated |
| High | 0 | No unresolved high-severity blocker remains in the local clean repo state |
| Medium | 0 | No unresolved medium-severity blocker remains after current classification and patch status review |
| Low | 0 | None validated as open |
| Informational | 1 | Readiness architecture and live-validation context remain documented |

## Current Readiness Assessment

Current status: READY FOR INTERNAL SECURITY REVIEW

Rationale:

- Go tests, Go vet, govulncheck, Gitleaks, and Grype support the current clean state.
- Semgrep remains at 8 findings, but each finding is classified and documented.
- The remaining Semgrep items do not create an unresolved high- or critical-severity blocker in this local clean repository state.
- Current evidence and report language are aligned to the clean repo state.

## Boundary Statement

This report is internal security-readiness and audit-preparation evidence. It does not represent formal certification, legal compliance, or third-party approval.
