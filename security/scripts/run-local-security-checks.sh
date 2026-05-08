#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCAN_DIR="$ROOT_DIR/security/scans"
EVIDENCE_DIR="$ROOT_DIR/security/evidence"
COMMAND_LOG="$EVIDENCE_DIR/commands-run.md"
TS="$(date -u +"%Y%m%dT%H%M%SZ")"
OVERALL_STATUS=0

export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
DEFAULT_CACHE_ROOT="${APIG0_CACHE_DIR:-/private/tmp/apig0-security-checks}"
export GOCACHE="${GOCACHE:-"$DEFAULT_CACHE_ROOT/go-build"}"
export GOMODCACHE="${GOMODCACHE:-"$DEFAULT_CACHE_ROOT/go-mod"}"

mkdir -p "$SCAN_DIR" "$EVIDENCE_DIR" "$EVIDENCE_DIR/logs" "$EVIDENCE_DIR/screenshots" "$GOCACHE" "$GOMODCACHE"
cd "$ROOT_DIR" || exit 1

relpath() {
  case "$1" in
    "$ROOT_DIR"/*) printf '%s' "${1#$ROOT_DIR/}" ;;
    *) printf '%s' "$1" ;;
  esac
}

ensure_command_log() {
  if [ ! -f "$COMMAND_LOG" ]; then
    {
      printf '# Commands Run\n\n'
      printf 'This file records security-readiness commands executed for Apig0.\n\n'
      printf '## Command Log\n\n'
      printf '| Timestamp (UTC) | Actor | Command | Output Location | Notes |\n'
      printf '| --- | --- | --- | --- | --- |\n'
    } > "$COMMAND_LOG"
  fi
}

append_command() {
  local command_text="$1"
  local output_location="$2"
  local notes="$3"
  ensure_command_log
  printf '| %s | `local-security-checks` | `%s` | `%s` | %s |\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$command_text" "$output_location" "$notes" >> "$COMMAND_LOG"
}

record_skip() {
  local outfile="$1"
  local tool="$2"
  local reason="$3"
  {
    printf '%s skipped\n' "$tool"
    printf 'timestamp_utc=%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    printf 'reason=%s\n' "$reason"
  } > "$outfile"
  printf 'Skipped %s: %s\n' "$tool" "$reason"
}

run_capture() {
  local outfile="$1"
  shift
  local command_text="$*"
  append_command "$command_text" "$(relpath "$outfile")" "Executed by local security readiness script."
  printf 'Running: %s\n' "$command_text"
  "$@" > "$outfile" 2>&1
  local status=$?
  printf '\nexit_status=%s\n' "$status" >> "$outfile"
  if [ "$status" -ne 0 ]; then
    OVERALL_STATUS=1
  fi
  return "$status"
}

run_gosec() {
  local sarif_file="$SCAN_DIR/gosec-$TS.sarif"
  local log_file="$SCAN_DIR/gosec-$TS.txt"
  local command_text="gosec -exclude-dir=.gocache -exclude-dir=.gomodcache -exclude-dir=secret-creator -exclude-dir=dist -fmt sarif -out $(relpath "$sarif_file") ./..."
  append_command "$command_text" "$(relpath "$log_file")" "Executed when gosec is installed."
  printf 'Running: %s\n' "$command_text"
  gosec -exclude-dir=.gocache -exclude-dir=.gomodcache -exclude-dir=secret-creator -exclude-dir=dist -fmt sarif -out "$sarif_file" ./... > "$log_file" 2>&1
  local status=$?
  printf '\nexit_status=%s\n' "$status" >> "$log_file"
  if [ "$status" -ne 0 ]; then
    OVERALL_STATUS=1
  fi
}

run_gitleaks() {
  local sarif_file="$SCAN_DIR/gitleaks-$TS.sarif"
  local log_file="$SCAN_DIR/gitleaks-$TS.txt"
  local command_text="gitleaks detect --source . --report-format sarif --report-path $(relpath "$sarif_file") --redact"
  append_command "$command_text" "$(relpath "$log_file")" "Executed when gitleaks is installed."
  printf 'Running: %s\n' "$command_text"
  gitleaks detect --source . --report-format sarif --report-path "$sarif_file" --redact > "$log_file" 2>&1
  local status=$?
  printf '\nexit_status=%s\n' "$status" >> "$log_file"
  if [ "$status" -ne 0 ]; then
    OVERALL_STATUS=1
  fi
}

is_truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

extract_host() {
  local target="$1"
  target="${target#http://}"
  target="${target#https://}"
  target="${target%%/*}"
  target="${target%%\?*}"
  target="${target%%#*}"
  target="${target##*@}"

  case "$target" in
    \[*\]*)
      target="${target%%]*}"
      target="${target#[}"
      ;;
    *:*)
      target="${target%%:*}"
      ;;
  esac

  printf '%s' "$target"
}

run_optional_nmap() {
  local requested="false"
  local raw_target=""
  local notes=""

  if [ -n "${APIG0_TARGET_URL:-}" ]; then
    requested="true"
    raw_target="$APIG0_TARGET_URL"
    notes="Explicit APIG0_TARGET_URL target."
  elif is_truthy "${APIG0_RUN_NMAP:-}"; then
    requested="true"
    raw_target="127.0.0.1"
    notes="Localhost scan requested by APIG0_RUN_NMAP."
  fi

  if [ "$requested" != "true" ]; then
    record_skip "$SCAN_DIR/nmap-$TS.txt" "nmap" "not requested; set APIG0_RUN_NMAP=true for localhost or APIG0_TARGET_URL for an explicit target"
    return 0
  fi

  if ! command -v nmap >/dev/null 2>&1; then
    record_skip "$SCAN_DIR/nmap-$TS.txt" "nmap" "nmap is not installed"
    return 0
  fi

  local host
  host="$(extract_host "$raw_target")"

  case "$host" in
    ""|*[!A-Za-z0-9._:-]*)
      record_skip "$SCAN_DIR/nmap-$TS.txt" "nmap" "invalid target host parsed from APIG0_TARGET_URL"
      OVERALL_STATUS=1
      return 1
      ;;
  esac

  local outfile="$SCAN_DIR/nmap-$TS.txt"
  append_command "nmap -Pn -sV $host" "$(relpath "$outfile")" "$notes"
  printf 'Running: nmap -Pn -sV %s\n' "$host"
  nmap -Pn -sV "$host" > "$outfile" 2>&1
  local status=$?
  printf '\nexit_status=%s\n' "$status" >> "$outfile"
  if [ "$status" -ne 0 ]; then
    OVERALL_STATUS=1
  fi
}

if command -v go >/dev/null 2>&1; then
  run_capture "$SCAN_DIR/go-test-$TS.txt" go test ./...
else
  record_skip "$SCAN_DIR/go-test-$TS.txt" "go test" "go is not installed or not available on PATH"
  OVERALL_STATUS=1
fi

if command -v govulncheck >/dev/null 2>&1; then
  run_capture "$SCAN_DIR/govulncheck-$TS.txt" govulncheck ./...
else
  record_skip "$SCAN_DIR/govulncheck-$TS.txt" "govulncheck" "govulncheck is not installed"
fi

if command -v gosec >/dev/null 2>&1; then
  run_gosec
else
  record_skip "$SCAN_DIR/gosec-$TS.txt" "gosec" "gosec is not installed"
fi

if command -v gitleaks >/dev/null 2>&1; then
  run_gitleaks
else
  record_skip "$SCAN_DIR/gitleaks-$TS.txt" "gitleaks" "gitleaks is not installed"
fi

run_optional_nmap

{
  printf 'Apig0 local security readiness checks\n'
  printf 'timestamp_utc=%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  printf 'scan_directory=%s\n' "$(relpath "$SCAN_DIR")"
  printf 'overall_status=%s\n' "$OVERALL_STATUS"
  printf 'note=Optional tools are skipped safely when unavailable. Nmap is not run against public targets by default.\n'
} > "$SCAN_DIR/summary-$TS.txt"

exit "$OVERALL_STATUS"
