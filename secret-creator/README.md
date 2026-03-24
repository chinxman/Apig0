# secret-creator

Standalone TOTP secret generator for Apig0. Completely separate from the gateway — no shared dependencies.

## Setup

```bash
cd secret-creator
go mod tidy
```

## Usage

```bash
# default user (devin)
go run .

# specific user
go run . alice
```

## Output

- Prints the secret and the exact `export` command to run
- Saves a `<username>-totp-qr.png` you can scan with any authenticator app

## What to do with it

1. Run it once per user
2. Scan the QR with Google Authenticator / Authy / 1Password
3. Export the env var it prints before starting the gateway:

```bash
export APIG0_TOTP_SECRET_DEVIN=<secret from output>
export APIG0_PASSWORD_DEVIN=<your chosen password>
export APIG0_SESSION_TTL=8h
go run . (in the main apig0 folder)
```
