# Risk Register

| Risk ID | Asset | Threat | Vulnerability | Likelihood | Impact | Risk Rating | Existing Controls | Recommended Treatment | Owner | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| RISK-001 | Git repository history | Historical credential exposure | Gitleaks found 4 redacted generic API key matches in removed `secret-creator/secret.txt` history | Unknown | High | High | Current tree ignores `secret-creator/`; gitleaks evidence captured | Owner must verify whether values were real, rotate/revoke if needed, and decide on history purge or accepted risk | Pending owner | Needs Manual Review |
| RISK-002 | Runtime authentication | MFA brute-force attempts after password challenge | Invalid TOTP attempts previously did not increment lockout | Low after patch | Medium | Medium | TOTP attempts now record failures; tests added | Monitor auth failure logs and consider shared lockout state for multi-node deployments | Pending | Treated |
| RISK-003 | Upstream proxy configuration | Admin misconfiguration or SSRF-style abuse | Service base URL validation was too loose | Low after patch | Medium | Medium | Admin-only routes, URL validation, auth/CSRF controls | Constrain admin access and egress in production deployments | Pending | Treated |
| RISK-004 | Build/runtime toolchain | Known Go standard-library vulnerability exposure | Local go1.26.2 reported called vulnerabilities | Low after patch | High | Medium | `toolchain go1.26.3`, CI Go 1.26.3, govulncheck retest | Require operators to build with Go 1.26.3 or later | Pending | Treated |
| RISK-005 | Live `orders` service path | Authorized route outage | Live upstream transport for configured `orders` service fails and gateway returns `502 upstream unavailable` | Medium | Medium | Medium | Gateway auth, route lookup, and health endpoint work; live repro evidence captured | Owner should verify live service `base_url`, upstream listener, network path, and TLS trust from gateway host; add proxy transport diagnostics if absent | Pending owner | Needs Manual Review |

## Guidance

- Link risks to findings when applicable.
- Use realistic likelihood and impact ratings.
- Track accepted risks explicitly and review them periodically.
- Do not treat unresolved pending evidence as proof that risk is low.
