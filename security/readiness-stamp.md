# Security Readiness Stamp

## Current Readiness Status

READY FOR INTERNAL SECURITY REVIEW

## Run Metadata

| Field | Value |
| --- | --- |
| Timestamp | 2026-05-08T21:45:00Z |
| Branch | security-readiness-evidence-pass |
| Commit hash | 27b82f0c88ef1271148755223bfb883a19d87328 |
| Latest workflow run | Pending |
| Operator | Codex |

## Check Summary

| Check | Status | Evidence |
| --- | --- | --- |
| AGENTS.md present | Passed | `AGENTS.md` |
| Codex subagents present | Passed | `.codex/agents/*.toml` |
| Scope and rules of engagement present | Passed | `security/scope.md`, `security/rules-of-engagement.md` |
| Go tests | Passed | `security/evidence/scans/go-test.txt` |
| Go vet | Passed | `security/evidence/scans/go-vet.txt` |
| govulncheck | Passed | `security/evidence/scans/govulncheck.txt` |
| gitleaks | Passed | `security/evidence/scans/gitleaks.txt`, `security/evidence/scans/gitleaks.sarif` |
| semgrep | Reviewed with findings classified | `security/evidence/scans/semgrep.json`, `security/reports/finding-classification.md` |
| trivy filesystem scan | Completed | `security/evidence/scans/trivy-fs.json` |
| SBOM generation | Completed | `security/evidence/sbom/syft-spdx.json` |
| vulnerability matching | Completed | `security/evidence/scans/grype.json`, `security/evidence/scans/govulncheck.txt` |
| live HTTP validation | Complete | `security/evidence/scans/live-http-*20260507T213454Z.*`, `security/evidence/scans/live-https-*20260507T213454Z.*` |
| nmap service/version | Passed | `security/evidence/scans/nmap-192-168-12-192-8989-20260507T213454Z.txt` |
| nuclei low-rate baseline subset | Complete | `security/evidence/scans/nuclei-baseline-192-168-12-192-8989-20260507T213454Z.jsonl`, `security/evidence/scans/nuclei-https-baseline-192-168-12-192-8989-20260507T213454Z.jsonl` |
| OWASP ZAP baseline | Skipped | ZAP baseline runtime remains unavailable locally |

## Checks Passed

- Required readiness architecture files exist.
- `go test ./...` passed.
- `go vet ./...` passed.
- `govulncheck ./...` passed.
- `gitleaks detect --source . --redact` reported no leaks found in the local clean repo.
- Grype reported `0` matches.
- The 8 remaining Semgrep findings were classified and documented.

## Remaining Findings

- The remaining Semgrep items are limited to accepted risk, false positive, and future-hardening classifications documented in `security/reports/finding-classification.md`.
- ZAP baseline remains a tooling gap for local DAST, but it is not a blocking condition for this internal readiness status.

## Residual Risks

- Cookie `Secure` behavior in plain-HTTP mode remains a future hardening decision.
- Provider CLI and generic exec vault integrations remain intentional operator-controlled execution surfaces and should stay limited to trusted administrative configurations.
- Account lockout state remains in-memory and single-node.

## Stamp Language

This stamp records internal security-readiness status for the local clean repository state only. It does not represent formal certification, legal compliance, or third-party approval.
