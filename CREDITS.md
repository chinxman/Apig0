# Credits & Acknowledgments

> *"If I have seen further, it is by standing on the shoulders of giants."*
> — Isaac Newton

---

apig0 was not built in isolation. Every line of this project was shaped by the open-source community, the people who invented foundational security patterns, the designers who created the color schemes I look at every day, and the engineers who open-sourced the libraries that make this work safely and reliably.

This document is my attempt to say thank you — loudly, and by name — to every one of them.

---

## Go Libraries

These are the open-source libraries that power apig0 directly. Without them, this project would have taken years instead of weeks.

---

### Web Framework

#### [Gin Web Framework](https://github.com/gin-gonic/gin)
**`github.com/gin-gonic/gin v1.12.0`**
Gin is the entire HTTP backbone of apig0. Every route, every middleware chain, every response — Gin handles it. It is fast, minimal, and idiomatic Go. The design of the route groups (`/api/admin`, `/auth/*`) and the middleware stack (CORS → Monitor → Session → CSRF → RateLimit) was made elegant by Gin's composable handler architecture.

*Created by the gin-gonic team. Maintained by thinkerou and the community.*

---

#### [gin-contrib/sse](https://github.com/gin-contrib/sse)
**`github.com/gin-contrib/sse v1.1.0`**
The live monitoring dashboard in apig0 uses Server-Sent Events to stream request activity in real time to the browser. This library provides Gin-native SSE support that made that streaming endpoint clean and reliable.

*Maintained by the gin-contrib organization.*

---

### Authentication & Security

#### [pquerna/otp](https://github.com/pquerna/otp)
**`github.com/pquerna/otp v1.5.0`**
This is the library behind every TOTP code validation in apig0. Every time a user opens Google Authenticator, types a 6-digit code, and hits verify — `pquerna/otp` is what checks it. It implements RFC 6238 (TOTP) and RFC 4226 (HOTP) with a clean, well-tested Go API.

The anti-replay cache in `auth/totp.go` and the TOTP secret provisioning in the setup wizard all build directly on this library's `totp.Validate()` and `totp.GenerateCode()` functions.

*Created by Paul Querna.*

---

#### [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)
**`golang.org/x/crypto v0.48.0`**
Every user password stored in apig0 is hashed with bcrypt via this package. The `userstore.go` calls `bcrypt.GenerateFromPassword` on create and `bcrypt.CompareHashAndPassword` on login. This is the gold standard for password storage and the Go team's own extended crypto library is the right place to get it.

*Maintained by the Go Team at Google.*

---

### QR Code Generation

#### [skip2/go-qrcode](https://github.com/skip2/go-qrcode)
**`github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e`**
When apig0 provisions a new user's TOTP secret, it needs to display a scannable QR code so the user can add it to their authenticator app. `skip2/go-qrcode` generates that PNG in-memory, which `auth/totp.go` then base64-encodes into a data URI returned to the browser — no temp files, no disk writes.

*Created by skip2.*

---

#### [boombuler/barcode](https://github.com/boombuler/barcode)
**`github.com/boombuler/barcode v1.0.1`**
The underlying barcode encoding engine that `go-qrcode` depends on. Handles the actual QR matrix generation at the byte level.

*Created by Silvan Mühlemann (boombuler).*

---

### JSON & Encoding

#### [bytedance/sonic](https://github.com/bytedance/sonic)
**`github.com/bytedance/sonic v1.15.0`**
Sonic is Gin's high-performance JSON serializer on amd64. It uses JIT compilation to generate SIMD-accelerated marshaling/unmarshaling code at runtime. All the API response payloads from apig0 benefit from this under the hood when Gin selects the fastest available JSON encoder for the platform.

*Created by the ByteDance Go infrastructure team.*

---

#### [bytedance/gopkg](https://github.com/bytedance/gopkg)
**`github.com/bytedance/gopkg v0.1.3`**
ByteDance's shared Go utility library, a dependency of sonic.

*Maintained by ByteDance.*

---

#### [cloudwego/base64x](https://github.com/cloudwego/base64x)
**`github.com/cloudwego/base64x v0.1.6`**
SIMD-accelerated base64 codec used internally by sonic. The QR data URI encoding in `auth/totp.go` also uses base64, though via the standard library — this one powers the fast path inside Gin's JSON layer.

*Created by the CloudWeGo team (ByteDance's open-source infrastructure group).*

---

#### [goccy/go-json](https://github.com/goccy/go-json)
**`github.com/goccy/go-json v0.10.5`**
A fast, reflection-based JSON encoder/decoder that Gin uses as a fallback on non-amd64 platforms. Zero-allocation optimizations make it significantly faster than `encoding/json` for the API response patterns apig0 uses.

*Created by Masaaki Goshima (goccy).*

---

#### [goccy/go-yaml](https://github.com/goccy/go-yaml)
**`github.com/goccy/go-yaml v1.19.2`**
YAML parsing support used by Gin for config loading. `apig0.yaml` is parsed via this library.

*Created by Masaaki Goshima (goccy).*

---

#### [json-iterator/go](https://github.com/json-iterator/go)
**`github.com/json-iterator/go v1.1.12`**
A drop-in replacement for `encoding/json` used inside Gin's codec stack. Known for being significantly faster than the standard library by pre-computing struct layouts.

*Created by Tao Wen (json-iterator). Now community maintained.*

---

#### [ugorji/go/codec](https://github.com/ugorji/go)
**`github.com/ugorji/go/codec v1.3.1`**
A high-performance encoding codec library used by Gin for msgpack and CBOR support.

*Created by Ugorji Nwoke.*

---

#### [pelletier/go-toml](https://github.com/pelletier/go-toml)
**`github.com/pelletier/go-toml/v2 v2.2.4`**
TOML configuration file support used by Gin's config subsystem. The `apig0.yaml` config loading chain includes TOML awareness for flexibility.

*Created by Pierre-Antoine Lacaze (pelletier).*

---

### Input Validation

#### [go-playground/validator](https://github.com/go-playground/validator)
**`github.com/go-playground/validator/v10 v10.30.1`**
Every `ShouldBindJSON` call in apig0's auth handlers uses this library to validate incoming request fields (required fields, formats, lengths). The `binding:"required"` struct tags that ensure username/password/challenge fields are present before auth logic runs — that's this library doing its job.

*Created by Dean Karn (joeybloggs). Maintained by the go-playground organization.*

---

#### [go-playground/locales](https://github.com/go-playground/locales)
**`github.com/go-playground/locales v0.14.1`**

#### [go-playground/universal-translator](https://github.com/go-playground/universal-translator)
**`github.com/go-playground/universal-translator v0.18.1`**
Locale and translation support that backs the validator's human-readable error messages.

*Maintained by the go-playground organization.*

---

#### [leodido/go-urn](https://github.com/leodido/go-urn)
**`github.com/leodido/go-urn v1.4.0`**
URN parsing used internally by the validator library.

*Created by Leonardo Di Donato (leodido).*

---

### Networking & Protocol

#### [quic-go/quic-go](https://github.com/quic-go/quic-go)
**`github.com/quic-go/quic-go v0.59.0`**
A full QUIC and HTTP/3 implementation in pure Go. Gin pulls this in to support next-generation transport when available.

*Created by Lucas Clemente. Maintained by the quic-go organization.*

---

#### [quic-go/qpack](https://github.com/quic-go/qpack)
**`github.com/quic-go/qpack v0.6.0`**
QPACK header compression for HTTP/3, a dependency of quic-go.

*Maintained by the quic-go organization.*

---

#### [gabriel-vasile/mimetype](https://github.com/gabriel-vasile/mimetype)
**`github.com/gabriel-vasile/mimetype v1.4.12`**
MIME type detection by reading file magic bytes. Used by Gin when serving static content.

*Created by Gabriel Vasile.*

---

### System & Runtime

#### [mattn/go-isatty](https://github.com/mattn/go-isatty)
**`github.com/mattn/go-isatty v0.0.20`**
Detects whether stdout is connected to a terminal. Gin uses this to decide whether to colorize log output. The startup logs you see in a terminal versus piped to a file look different because of this.

*Created by Yasuhiro Matsumoto (mattn) — one of the most prolific Go open-source contributors ever.*

---

#### [klauspost/cpuid](https://github.com/klauspost/cpuid)
**`github.com/klauspost/cpuid/v2 v2.3.0`**
CPU feature detection used by sonic to decide which SIMD instruction sets are available at runtime (SSE4.2, AVX2, etc.).

*Created by Klaus Post — also the author of compress, reedsolomon, and many other high-performance Go libraries.*

---

#### [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys)
**`golang.org/x/sys v0.41.0`**
Low-level system call access for the Go extended libraries. Used for TTY detection, signal handling, and platform-specific behavior.

*Maintained by the Go Team.*

---

#### [golang.org/x/term](https://pkg.go.dev/golang.org/x/term)
**`golang.org/x/term v0.40.0`**
Terminal state management. Used in apig0 to handle the `APIG0_SHOW_QR` terminal output cleanly.

*Maintained by the Go Team.*

---

#### [golang.org/x/arch](https://pkg.go.dev/golang.org/x/arch)
**`golang.org/x/arch v0.22.0`**
Architecture-specific constants and disassembly support used by the JIT compiler in sonic.

*Maintained by the Go Team.*

---

#### [golang.org/x/net](https://pkg.go.dev/golang.org/x/net)
**`golang.org/x/net v0.51.0`**
Extended networking support — HTTP/2, WebSocket primitives, and network utilities.

*Maintained by the Go Team.*

---

#### [golang.org/x/text](https://pkg.go.dev/golang.org/x/text)
**`golang.org/x/text v0.34.0`**
Unicode, text normalization, and locale support.

*Maintained by the Go Team.*

---

#### [twitchyliquid64/golang-asm](https://github.com/twitchyliquid64/golang-asm)
**`github.com/twitchyliquid64/golang-asm v0.15.1`**
Go assembler tooling used by sonic's JIT compilation path.

*Created by twitchyliquid64.*

---

### Database & Infrastructure

#### [go.mongodb.org/mongo-driver/v2](https://github.com/mongodb/mongo-go-driver)
**`go.mongodb.org/mongo-driver/v2 v2.5.0`**
The official MongoDB Go driver. Currently a transitive dependency pulled in via Gin's codec stack. Not directly used in apig0's auth or config logic today, but present in the dependency graph.

*Created and maintained by MongoDB, Inc.*

---

#### [google.golang.org/protobuf](https://pkg.go.dev/google.golang.org/protobuf)
**`google.golang.org/protobuf v1.36.10`**
Protocol Buffers runtime for Go. Pulled in transitively by the networking and codec stack.

*Maintained by Google.*

---

### Testing

#### [stretchr/testify](https://github.com/testify/testify)
**`github.com/stretchr/testify v1.11.1`**
The assertion and mock library used in `auth/setup_test.go` and `config/vault_test.go`. `assert.Equal`, `assert.NoError`, and `require` — the whole readable test suite depends on this.

*Created by Mat Ryer and Tyler Bunnell. Maintained by the stretchr organization.*

---

#### [go.uber.org/mock](https://github.com/uber-go/mock)
**`go.uber.org/mock v0.6.0`**
Interface mocking framework used for unit testing vault backends and auth flows in isolation.

*Originally created by Google as `golang/mock`. Forked and actively maintained by Uber.*

---

#### [modern-go/concurrent](https://github.com/modern-go/concurrent) & [modern-go/reflect2](https://github.com/modern-go/reflect2)
**`github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd`**
**`github.com/modern-go/reflect2 v1.0.2`**
High-performance concurrent and reflection utilities used internally by `json-iterator/go`.

*Created by Tao Wen.*

---

---

## Security Patterns & Algorithms

The security architecture of apig0 is built on patterns that took decades and many brilliant people to develop. These are not just implementations — they are the product of hard-won security research.

---

### TOTP — Time-Based One-Time Passwords (RFC 6238)
The entire TOTP two-factor authentication flow in apig0 — secret generation, code validation, QR provisioning, the 30-second window, the ±1 period tolerance — is defined by **RFC 6238**, published by the IETF in 2011.

**Authors:** David M'Raihi, Salah Machani, Mingliang Pei, Johan Rydell
**Built on top of:** RFC 4226 (HOTP) by David M'Raihi, Frank Prereel, Mihir Bellare, Thierry Perrin, and José Silberberg

The `ValidateTOTP` function in `auth/totp.go` — including the anti-replay cache that prevents a code from being reused within its validity window — directly implements the security properties described in this RFC.

---

### Double-Submit Cookie — CSRF Protection
The CSRF protection in `middleware/csrf.go` implements the **double-submit cookie pattern**, documented and recommended by OWASP (the Open Web Application Security Project).

The pattern: set a random token as a non-HttpOnly cookie so JavaScript can read it, then require the client to echo it back in an `X-CSRF-Token` header on every state-changing request. Because an attacker's cross-origin request can't read your cookies (SameSite=Strict prevents automatic inclusion; `HttpOnly=false` on the CSRF token is intentional and by design), they cannot forge a valid header. The session cookie that actually authenticates the request *is* HttpOnly — only the CSRF token is JS-readable.

**Reference:** [OWASP Cross-Site Request Forgery Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)

---

### Token Bucket — Rate Limiting
The rate limiter in `middleware/ratelimit.go` implements the **token bucket algorithm**. Each user (or IP, for unauthenticated requests) has a virtual bucket that fills at a fixed rate (`RequestsPerMinute / 60` tokens per second) up to a `Burst` cap. Every allowed request consumes one token; when the bucket is empty, the request is rejected with HTTP 429.

The algorithm was first described formally by **James Turner** in a 1986 paper and later popularized by its use in network traffic shaping (RFC 2697, RFC 4115). It is the same algorithm used in AWS API Gateway, Stripe's rate limiter, and virtually every production system with adaptive throughput control.

---

### Brute-Force Lockout
The lockout system in `auth/lockout.go` implements a **failed-attempt counter with timed lockout**, one of the oldest and most battle-tested protections against credential stuffing and brute-force attacks. After 5 consecutive failures, the account is locked for 15 minutes.

This pattern is recommended by NIST SP 800-63B (Digital Identity Guidelines) and described in OWASP's Authentication Cheat Sheet.

---

### Challenge-Response Authentication
The two-step login flow (`/auth/login` → challenge token → `/auth/verify` → TOTP → session) is a classic **challenge-response protocol**. The login step validates the password and issues a short-lived (5-minute), single-use challenge token. Only after TOTP verification is the token consumed and a session created.

This design means that even if an attacker intercepts the challenge token, they still need the second factor — and once used, the token is gone.

---

### HttpOnly + SameSite=Strict Session Cookies
Session cookies in apig0 are set with `HttpOnly=true` (inaccessible to JavaScript, preventing XSS cookie theft) and `SameSite=Strict` (not sent on cross-site requests, preventing CSRF as a defense-in-depth measure). These are the OWASP-recommended cookie attributes, codified in RFC 6265 and the WHATWG living standard.

---

---

## Architecture Inspirations

These are the systems, tools, and projects that shaped how apig0 is structured — not because any code was copied, but because spending time with them shaped every design decision.

---

### [HashiCorp Vault](https://www.vaultproject.io/)
The entire secret backend abstraction in apig0 — the `VaultBackend` interface, KV v2 API calls, AppRole authentication, the `VAULT_TOKEN` and `VAULT_ROLE_ID` / `VAULT_SECRET_ID` env vars — is modeled directly after HashiCorp Vault's design language and API conventions.

The Vault integration in `config/vault_providers.go` speaks Vault's KV v2 HTTP API natively (`/v1/{engine}/data/{path}`). Every developer who has ever used Vault will immediately recognize the mental model.

*Vault was created by Mitchell Hashimoto and the HashiCorp team.*

---

### [Traefik](https://traefik.io/) / [Nginx](https://nginx.org/) / [Caddy](https://caddyserver.com/)
apig0 exists in the same conceptual space as Traefik, Nginx, and Caddy: a reverse proxy that sits in front of backend services and controls access. The service catalog model (name → base URL → auth type), the catch-all proxy route, and the per-service secret injection pattern are all architectural ideas refined over years of production use by these projects and their communities.

*Traefik: Emile Vauge and the containo.us team. Nginx: Igor Sysoev. Caddy: Matt Holt.*

---

### [net/http/httputil.ReverseProxy](https://pkg.go.dev/net/http/httputil#ReverseProxy)
The proxy layer in `proxy/proxy.go` wraps Go's own standard library `httputil.ReverseProxy`. The pattern of overriding `Director` to inject service credentials before forwarding — without modifying the original request object — is idiomatic Go and taught in the Go standard library documentation.

*The Go standard library. Created by the Go Team at Google.*

---

### [Auth0](https://auth0.com/) / [Okta](https://www.okta.com/)
The idea of a unified auth gateway that sits in front of your services, enforces MFA, issues session tokens, and provides an admin panel to manage users and permissions — that's the conceptual model of Auth0 and Okta, simplified down to something self-hosted and personal.

apig0 doesn't use their SDKs, but anyone who has configured Auth0 rules or Okta policies will recognize the intent.

---

### [Kong](https://konghq.com/) / [Apache APISIX](https://apisix.apache.org/) / [Envoy](https://www.envoyproxy.io/) / [Tyk](https://tyk.io/)
The gateway hardening pass that added machine tokens, path and method policy, audit logging, timeout and retry controls, and metrics exposure was shaped by spending time with serious gateway products and understanding what operators expect from them.

The goal was not to clone their breadth. The inspiration was narrower and more useful: if apig0 wants to be taken seriously, it needs credible machine access, explicit policy enforcement, observable decisions, and basic upstream resilience. Those expectations were sharpened by the standards these projects set.

*Kong: originally created by Marco Palladino and the Kong team. Apache APISIX: created by the Apache APISIX community. Envoy: created at Lyft and maintained by the CNCF community. Tyk: created by Martin Buhr and the Tyk team.*

---

### [Prometheus](https://prometheus.io/) / OpenMetrics
The `/metrics` endpoint added in the gateway hardening pass follows the operational model popularized by Prometheus: simple text exposition, easy scraping, and immediate interoperability with monitoring systems without forcing a heavy observability stack into the product.

OpenMetrics and Prometheus together helped shape the idea that apig0 should expose useful telemetry in a standard format while still keeping its built-in UI.

*Prometheus was originally created at SoundCloud and is now part of the CNCF ecosystem. OpenMetrics was developed through the Prometheus and CNCF observability communities.*

---

### Zero-Trust Access Models
The newer access-control additions in apig0 — machine auth, route-level policy, explicit deny reasoning, and gateway-mediated secret use — were also inspired by the broader zero-trust access model: authenticate strongly, authorize explicitly, and avoid handing sensitive upstream credentials directly to end users whenever the gateway can broker access safely.

That design direction owes a debt to the engineers, researchers, and operators behind identity-aware proxy systems, modern access brokers, and internal access tooling that made "who is this caller, what are they allowed to do, and why?" the central question instead of an afterthought.

---

---

## UI & Design

### [Tokyo Night](https://github.com/folke/tokyonight.nvim)
The entire color palette of the apig0 web UI — every hex value in the `:root` CSS block — is lifted directly from the **Tokyo Night** color scheme.

```
--bg:       #1a1b26   Tokyo Night background
--bg-card:  #16161e   Tokyo Night deep background
--border:   #2f334d   Tokyo Night selection/border
--text:     #c0caf5   Tokyo Night foreground
--text-dim: #565f89   Tokyo Night comment
--blue:     #7aa2f7   Tokyo Night blue
--purple:   #bb9af7   Tokyo Night purple
--green:    #9ece6a   Tokyo Night green
--yellow:   #e0af68   Tokyo Night yellow
--orange:   #ff9e64   Tokyo Night orange
--red:      #f7768e   Tokyo Night red/pink
--cyan:     #7dcfff   Tokyo Night cyan
```

Every card, button, badge, tab, and status indicator in the UI inherits these colors. If the dashboard feels at home to someone who uses Neovim, that's why.

*Created by Folke Lemaitre ([folke](https://github.com/folke)). One of the most widely used color schemes in the developer community.*

---

### Developer Monospace Fonts
The UI font stack — `'SF Mono', 'Cascadia Code', 'Fira Code', 'JetBrains Mono', monospace` — pays tribute to four beloved developer fonts, each designed for clarity in code and terminal environments:

- **SF Mono** — Designed by Apple, released with Xcode and the macOS Terminal. Optimized for retina displays.
- **[Cascadia Code](https://github.com/microsoft/cascadia-code)** — Designed by Microsoft for the Windows Terminal and VS Code. Features beautiful programming ligatures. *Created by Aaron Bell.*
- **[Fira Code](https://github.com/tonsky/FiraCode)** — Created by **Nikita Prokopov ([tonsky](https://github.com/tonsky))**, one of the most beloved monospace fonts in the community. Features ligatures that make operators like `=>`, `!=`, and `->` visually distinct.
- **[JetBrains Mono](https://www.jetbrains.com/lp/mono/)** — Designed by JetBrains specifically for developers, with increased letter height, adjusted character forms, and ligature support. *Created by Philipp Nurullin and Konstantin Bulenkov.*

---

### Server-Sent Events (SSE)
The live dashboard streams real-time request events to the browser using the **Server-Sent Events** specification — a W3C/WHATWG standard that lets a server push text events over a persistent HTTP connection. No WebSocket handshake, no polling, no external dependencies. Just a `text/event-stream` content type and a `for` loop.

*Specified by the WHATWG HTML Living Standard.*

---

---

## Standards & RFCs Referenced

| Standard | Description |
|---|---|
| RFC 6238 | TOTP: Time-Based One-Time Password Algorithm |
| RFC 4226 | HOTP: HMAC-Based One-Time Password Algorithm |
| RFC 6265 | HTTP State Management Mechanism (Cookies) |
| RFC 7617 | The 'Basic' HTTP Authentication Scheme |
| RFC 9110 | HTTP Semantics |
| RFC 2697 | A Single Rate Three Color Marker (Token Bucket) |
| NIST SP 800-63B | Digital Identity Guidelines: Authentication |
| OWASP ASVS | Application Security Verification Standard |

---

## A Personal Note

Every library listed in this document represents someone who chose to share their work instead of keeping it private. Every RFC represents engineers who standardized their ideas instead of siloing them. Every color in the UI exists because Folke opened a GitHub repo instead of just keeping a theme for himself.

Open source is a gift economy. This project exists because of it.

Thank you.

---

*Generated from source analysis of apig0. All library versions reflect go.mod at time of writing.*
