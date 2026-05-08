# Authorized Live Validation Report

## Metadata

| Field | Value |
| --- | --- |
| Date/time | 2026-05-08T21:45:00Z |
| Branch | security-readiness-evidence-pass |
| Commit hash | 27b82f0c88ef1271148755223bfb883a19d87328 |
| Operator | Codex |
| Authorized target | `https://192.168.12.192:8989` |
| Authorized host/port | `192.168.12.192:8989` only |

## Scope Confirmation

Validated against `security/scope.md` and `security/rules-of-engagement.md` before testing. No third-party, public IP, other LAN host, or other port was tested.

## Results

| Check | Result | Evidence |
| --- | --- | --- |
| HTTP root request | `HTTP/1.0 400 Bad Request`; response body: `Client sent an HTTP request to an HTTPS server.` | `security/evidence/scans/live-http-root-headers-20260507T213454Z.txt`, `security/evidence/scans/live-http-root-body-20260507T213454Z.html` |
| HTTP `/healthz` request | `HTTP/1.0 400 Bad Request`; response body: `Client sent an HTTP request to an HTTPS server.` | `security/evidence/scans/live-http-healthz-headers-20260507T213454Z.txt`, `security/evidence/scans/live-http-healthz-body-20260507T213454Z.json` |
| HTTPS root request | `HTTP/2 200 OK` with Apig0 Web UI HTML and security headers | `security/evidence/scans/live-https-root-headers-20260507T213454Z.txt`, `security/evidence/scans/live-https-root-body-20260507T213454Z.html` |
| HTTPS `/healthz` request | `HTTP/2 200 OK` with `{"status":"ok"}` | `security/evidence/scans/live-https-healthz-headers-20260507T213454Z.txt`, `security/evidence/scans/live-https-healthz-body-20260507T213454Z.json` |
| Nmap service/version on port `8989` | Host up; `8989/tcp open ssl/http Golang net/http server` | `security/evidence/scans/nmap-192-168-12-192-8989-20260507T213454Z.txt` |
| Nuclei baseline subset against authorized `https://` URL | 12 informational header matches against the HTTPS application response | `security/evidence/scans/nuclei-https-baseline-192-168-12-192-8989-20260507T213454Z.jsonl` |
| OWASP ZAP baseline | Skipped; runtime remains unavailable locally | `security/reports/tooling-gaps.md` |

## Interpretation

The authorized service is reachable on port `8989` over HTTPS/TLS and returns the Apig0 Web UI and `/healthz` response over TLS. The HTTP responses are protocol-mismatch artifacts; the HTTPS checks are the application-layer evidence for this target.

## Recommended Next Step

Run ZAP baseline once a compatible ZAP runtime or Docker is available, or accept the current nuclei and curl evidence if no further DAST tooling will be installed locally.
