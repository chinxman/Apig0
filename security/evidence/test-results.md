# Test And Scan Results

## Run Metadata

| Field | Value |
| --- | --- |
| Timestamp | 2026-05-08T21:45:00Z |
| Branch | security-readiness-evidence-pass |
| Commit hash | 27b82f0c88ef1271148755223bfb883a19d87328 |
| Operator | Codex |
| Environment | Local workspace `/Users/chinxman/Code/apig0-clean` |
| Go toolchain | go1.26.3 darwin/arm64 |

## Results

| Check | Command | Status | Evidence |
| --- | --- | --- | --- |
| Go tests | `go test ./...` | Passed | `security/evidence/scans/go-test.txt` |
| Go vet | `go vet ./...` | Passed | `security/evidence/scans/go-vet.txt` |
| govulncheck | `govulncheck ./...` | Passed | `security/evidence/scans/govulncheck.txt` |
| gitleaks history scan | `gitleaks detect --source . --redact` | Passed: no leaks found | `security/evidence/scans/gitleaks.txt`, `security/evidence/scans/gitleaks.sarif` |
| semgrep | Review current scan evidence | Completed: 8 findings remain and are classified | `security/evidence/scans/semgrep.json`, `security/reports/finding-classification.md` |
| trivy filesystem | Review current scan evidence | Completed | `security/evidence/scans/trivy-fs.json` |
| syft SBOM | Review current scan evidence | Completed | `security/evidence/sbom/syft-spdx.json` |
| grype vulnerability matching | Review current scan evidence | Passed: `0` matches | `security/evidence/scans/grype.json` |
| OWASP ZAP baseline | local/scoped target only | Skipped | ZAP baseline runtime unavailable locally |
| nuclei | local/scoped target only | Completed against both protocol variants | See authorized live validation below |

## Authorized Live Validation Results

| Check | Command | Status | Evidence |
| --- | --- | --- | --- |
| Scope confirmation | Validated against `security/scope.md` and `security/rules-of-engagement.md` | Complete | `security/scope.md`, `security/rules-of-engagement.md` |
| HTTP root check | `curl --max-time 10 http://192.168.12.192:8989/` | Completed: HTTP 400 protocol mismatch | `security/evidence/scans/live-http-root-headers-20260507T213454Z.txt`, `security/evidence/scans/live-http-root-body-20260507T213454Z.html` |
| HTTP health check | `curl --max-time 10 http://192.168.12.192:8989/healthz` | Completed: HTTP 400 protocol mismatch | `security/evidence/scans/live-http-healthz-headers-20260507T213454Z.txt`, `security/evidence/scans/live-http-healthz-body-20260507T213454Z.json` |
| HTTPS root check | `curl -k --max-time 10 https://192.168.12.192:8989/` | Completed: HTTP 200 OK | `security/evidence/scans/live-https-root-headers-20260507T213454Z.txt`, `security/evidence/scans/live-https-root-body-20260507T213454Z.html` |
| HTTPS health check | `curl -k --max-time 10 https://192.168.12.192:8989/healthz` | Completed: HTTP 200 OK | `security/evidence/scans/live-https-healthz-headers-20260507T213454Z.txt`, `security/evidence/scans/live-https-healthz-body-20260507T213454Z.json` |
| HTTPS user info check | `curl -k -sS https://192.168.12.192:8989/api/user/info -H 'Authorization: Bearer <redacted>'` | Completed: token valid for `orders` service | `security/evidence/scans/live-https-user-info-20260513T030900Z.json` |
| HTTPS `orders` check | `curl -k -sS -D ... https://192.168.12.192:8989/orders -H 'Authorization: Bearer <redacted>'` | Completed: HTTP 502 with body `upstream unavailable` | `security/evidence/scans/live-https-orders-headers-20260513T030900Z.txt`, `security/evidence/scans/live-https-orders-body-20260513T030900Z.txt` |
| HTTPS `orders/` check | `curl -k -sS -D ... https://192.168.12.192:8989/orders/ -H 'Authorization: Bearer <redacted>'` | Completed: HTTP 502 with body `upstream unavailable` | `security/evidence/scans/live-https-orders-slash-headers-20260513T030900Z.txt`, `security/evidence/scans/live-https-orders-slash-body-20260513T030900Z.txt` |
| HTTPS `orders` timing check | `curl -k -sS -o /dev/null -w ... https://192.168.12.192:8989/orders -H 'Authorization: Bearer <redacted>'` | Completed: fast failure, total `0.015147s` | `security/evidence/scans/live-https-orders-timing-20260513T030900Z.txt` |
| Nmap service/version | `nmap -sV -p 8989 --version-light --max-retries 2 --host-timeout 60s 192.168.12.192` | Completed: `8989/tcp open ssl/http Golang net/http server` | `security/evidence/scans/nmap-192-168-12-192-8989-20260507T213454Z.txt` |
| Nuclei baseline subset | `nuclei -u https://192.168.12.192:8989 -id http-missing-security-headers,options-method,robots-txt,security-txt -rl 1 -c 1` | Completed: informational matches only | `security/evidence/scans/nuclei-https-baseline-192-168-12-192-8989-20260507T213454Z.jsonl` |

## Notes

- A sandboxed `govulncheck` attempt in this final review could not reach `vuln.go.dev`; the authorized network rerun completed successfully.
- The local clean repo state is now Gitleaks-clean.
- The earlier HTTP nuclei and curl results are retained as protocol-mismatch context; the HTTPS evidence is the application-layer validation set.
- The `orders` route failure is not protocol mismatch or auth failure: live evidence shows healthy `/healthz`, valid token scope for `orders`, and gateway-generated `502 upstream unavailable` for both `/orders` and `/orders/`.
- Patch validation after adding `tls_skip_verify` support and proxy error logging: `go test ./proxy ./config -count=1` passed and `go vet ./...` passed. Full `go test ./...` remains blocked by current repository module-state issues reporting missing `go.sum` entries outside the focused patch scope.
