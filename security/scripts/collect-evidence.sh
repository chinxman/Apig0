#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVIDENCE_DIR="$ROOT_DIR/security/evidence"
SCAN_DIR="$ROOT_DIR/security/scans"
REPORT_DIR="$ROOT_DIR/security/reports"
CONTROL_DIR="$ROOT_DIR/security/controls"
COMMAND_LOG="$EVIDENCE_DIR/commands-run.md"
INDEX_FILE="$EVIDENCE_DIR/evidence-index.md"
TS="$(date -u +"%Y%m%dT%H%M%SZ")"
BUNDLE_ROOT="$EVIDENCE_DIR/bundles"
BUNDLE_DIR="$BUNDLE_ROOT/apig0-evidence-$TS"

mkdir -p "$BUNDLE_ROOT"

suffix=1
while [ -e "$BUNDLE_DIR" ]; do
  BUNDLE_DIR="$BUNDLE_ROOT/apig0-evidence-$TS-$suffix"
  suffix=$((suffix + 1))
done

mkdir -p "$BUNDLE_DIR/scans" "$BUNDLE_DIR/logs" "$BUNDLE_DIR/screenshots" "$BUNDLE_DIR/reports" "$BUNDLE_DIR/controls"

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
  ensure_command_log
  printf '| %s | `evidence-collector` | `security/scripts/collect-evidence.sh` | `%s` | Created timestamped evidence bundle. |\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$(relpath "$BUNDLE_DIR")" >> "$COMMAND_LOG"
}

copy_files_from_dir() {
  local source_dir="$1"
  local dest_dir="$2"

  if [ ! -d "$source_dir" ]; then
    return 0
  fi

  find "$source_dir" -maxdepth 1 -type f ! -name '.gitkeep' -exec cp -p {} "$dest_dir/" \;
}

copy_files_from_dir "$SCAN_DIR" "$BUNDLE_DIR/scans"
copy_files_from_dir "$EVIDENCE_DIR/logs" "$BUNDLE_DIR/logs"
copy_files_from_dir "$EVIDENCE_DIR/screenshots" "$BUNDLE_DIR/screenshots"
copy_files_from_dir "$REPORT_DIR" "$BUNDLE_DIR/reports"
copy_files_from_dir "$CONTROL_DIR" "$BUNDLE_DIR/controls"

if [ -n "${APIG0_LOG_PATH:-}" ] && [ -f "$APIG0_LOG_PATH" ]; then
  cp -p "$APIG0_LOG_PATH" "$BUNDLE_DIR/logs/"
fi

append_command

{
  printf '\n## Evidence Bundle %s\n\n' "$TS"
  printf '| Field | Value |\n'
  printf '| --- | --- |\n'
  printf '| Bundle | `%s` |\n' "$(relpath "$BUNDLE_DIR")"
  printf '| Created UTC | `%s` |\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  printf '| Source scans | `%s` |\n' "$(relpath "$SCAN_DIR")"
  printf '| Source logs | `%s` |\n' "$(relpath "$EVIDENCE_DIR/logs")"
  printf '| Status | `Collected` |\n'
} >> "$INDEX_FILE"

printf 'Evidence bundle created: %s\n' "$(relpath "$BUNDLE_DIR")"
