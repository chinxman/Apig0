# Commands Run

This file records security-readiness commands executed for Apig0. It supports traceability for local checks, external evidence imports, and audit-review preparation.

## Current Status

- Local security checks: Pending
- External security lab import: Pending external lab initialization
- Advanced scan validation: Pending

## Command Log

| Timestamp (UTC) | Actor | Command | Output Location | Notes |
| --- | --- | --- | --- | --- |
| Pending | `security/scripts/run-local-security-checks.sh` | `go test ./...` | `security/scans/` | Runs when local checks are executed. |
| Pending | `security/scripts/run-local-security-checks.sh` | Optional installed scanners | `security/scans/` | `govulncheck`, `gosec`, `gitleaks`, and local-only or explicitly targeted `nmap` runs are recorded here when executed. |
| Pending | `security/integrations/import-evidence.sh` | Import approved external artifacts | `security/evidence/imported/` | Pending external lab initialization. |
| 2026-05-07T03:39:50Z | `local-security-checks` | `go test ./...` | `security/scans/go-test-20260507T033950Z.txt` | Executed by local security readiness script. |
| 2026-05-07T03:39:52Z | `local-security-checks` | `govulncheck ./...` | `security/scans/govulncheck-20260507T033950Z.txt` | Executed by local security readiness script. |
| 2026-05-07T03:39:53Z | `local-security-checks` | `gosec -fmt sarif -out security/scans/gosec-20260507T033950Z.sarif ./...` | `security/scans/gosec-20260507T033950Z.txt` | Executed when gosec is installed. |
| 2026-05-07T03:41:26Z | `local-security-checks` | `gitleaks detect --source . --report-format sarif --report-path security/scans/gitleaks-20260507T033950Z.sarif --redact` | `security/scans/gitleaks-20260507T033950Z.txt` | Executed when gitleaks is installed. |
| 2026-05-07T10:34:59Z | `local-security-checks` | `go test ./...` | `security/scans/go-test-20260507T103459Z.txt` | Executed by local security readiness script. |
| 2026-05-07T10:35:18Z | `local-security-checks` | `govulncheck ./...` | `security/scans/govulncheck-20260507T103459Z.txt` | Executed by local security readiness script. |
| 2026-05-07T10:35:20Z | `local-security-checks` | `gosec -exclude-dir=.gocache -exclude-dir=.gomodcache -exclude-dir=secret-creator -exclude-dir=dist -fmt sarif -out security/scans/gosec-20260507T103459Z.sarif ./...` | `security/scans/gosec-20260507T103459Z.txt` | Executed when gosec is installed. |
| 2026-05-07T10:36:45Z | `local-security-checks` | `gitleaks detect --source . --report-format sarif --report-path security/scans/gitleaks-20260507T103459Z.sarif --redact` | `security/scans/gitleaks-20260507T103459Z.txt` | Executed when gitleaks is installed. |
