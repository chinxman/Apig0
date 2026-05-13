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
| HTTPS `/api/user/info` with provided token | `HTTP/2 200 OK`; token valid for service `orders` | `security/evidence/scans/live-https-user-info-20260513T030900Z.json` |
| HTTPS `/orders` with provided token | `HTTP/2 502 Bad Gateway`; body `upstream unavailable` | `security/evidence/scans/live-https-orders-headers-20260513T030900Z.txt`, `security/evidence/scans/live-https-orders-body-20260513T030900Z.txt` |
| HTTPS `/orders/` with provided token | `HTTP/2 502 Bad Gateway`; body `upstream unavailable` | `security/evidence/scans/live-https-orders-slash-headers-20260513T030900Z.txt`, `security/evidence/scans/live-https-orders-slash-body-20260513T030900Z.txt` |
| HTTPS `/orders` timing sample | `HTTP 502` in about `15ms` total | `security/evidence/scans/live-https-orders-timing-20260513T030900Z.txt` |
| Nmap service/version on port `8989` | Host up; `8989/tcp open ssl/http Golang net/http server` | `security/evidence/scans/nmap-192-168-12-192-8989-20260507T213454Z.txt` |
| Nuclei baseline subset against authorized `https://` URL | 12 informational header matches against the HTTPS application response | `security/evidence/scans/nuclei-https-baseline-192-168-12-192-8989-20260507T213454Z.jsonl` |
| OWASP ZAP baseline | Skipped; runtime remains unavailable locally | `security/reports/tooling-gaps.md` |

## Interpretation

The authorized service is reachable on port `8989` over HTTPS/TLS and returns the Apig0 Web UI and `/healthz` response over TLS. The provided bearer token is valid for service `orders`, but both `/orders` and `/orders/` return gateway-generated `502` responses with body `upstream unavailable`. Local code review of `proxy/proxy.go` shows both path variants normalize to the same upstream target and that this body is emitted only when reverse proxy transport fails, making upstream connectivity, upstream TLS trust, or live `base_url` configuration the primary causes to inspect.

## Recommended Next Step

Inspect live `orders` service configuration and gateway host logs to determine whether failure is caused by unreachable upstream host, refused connection, TLS certificate validation failure, DNS failure, or timeout; then retest `GET /orders` and `GET /orders/` after correction.
