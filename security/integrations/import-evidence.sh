#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCE_DIR="${APIG0_SECURITY_LAB_PATH:-"$ROOT_DIR/../apig0-security-lab/outputs"}"
EVIDENCE_DIR="$ROOT_DIR/security/evidence"
IMPORT_ROOT="$EVIDENCE_DIR/imported"
INDEX_FILE="$EVIDENCE_DIR/evidence-index.md"
COMMAND_LOG="$EVIDENCE_DIR/commands-run.md"
TS="$(date -u +"%Y%m%dT%H%M%SZ")"
IMPORT_DIR="$IMPORT_ROOT/apig0-security-lab-$TS"

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
  printf '| %s | `external-evidence-import` | `security/integrations/import-evidence.sh` | `%s` | Imported approved artifact types from `%s`. |\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$(relpath "$IMPORT_DIR")" "$SOURCE_DIR" >> "$COMMAND_LOG"
}

approved_type() {
  case "$1" in
    *.sarif|*.SARIF|*.json|*.JSON|*.xml|*.XML|*.html|*.HTML|*.md|*.MD|*.txt|*.TXT|*.log|*.LOG|*.csv|*.CSV|*.png|*.PNG|*.jpg|*.JPG|*.jpeg|*.JPEG|*.webp|*.WEBP|*.pdf|*.PDF|*.zip|*.ZIP|*.tar|*.TAR|*.tar.gz|*.TAR.GZ|*.tgz|*.TGZ)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

if [ ! -d "$SOURCE_DIR" ]; then
  printf 'External security lab output directory not found: %s\n' "$SOURCE_DIR" >&2
  printf 'Set APIG0_SECURITY_LAB_PATH to an existing outputs directory if needed.\n' >&2
  exit 0
fi

mkdir -p "$IMPORT_ROOT"

suffix=1
while [ -e "$IMPORT_DIR" ]; do
  IMPORT_DIR="$IMPORT_ROOT/apig0-security-lab-$TS-$suffix"
  suffix=$((suffix + 1))
done

mkdir -p "$IMPORT_DIR"

count=0
skipped=0

while IFS= read -r artifact; do
  if ! approved_type "$artifact"; then
    skipped=$((skipped + 1))
    continue
  fi

  relative_path="${artifact#$SOURCE_DIR/}"
  dest="$IMPORT_DIR/$relative_path"
  mkdir -p "$(dirname "$dest")"

  if [ -e "$dest" ]; then
    cp -p "$dest" "$dest.backup-$TS"
  fi

  cp -p "$artifact" "$dest"
  count=$((count + 1))
done < <(find "$SOURCE_DIR" -type f)

append_command

{
  printf '\n## External Security Lab Import %s\n\n' "$TS"
  printf '| Field | Value |\n'
  printf '| --- | --- |\n'
  printf '| Import directory | `%s` |\n' "$(relpath "$IMPORT_DIR")"
  printf '| Source directory | `%s` |\n' "$SOURCE_DIR"
  printf '| Imported artifacts | `%s` |\n' "$count"
  printf '| Skipped files | `%s` |\n' "$skipped"
  printf '| Status | `Pending validation` |\n'
  printf '| Notes | Evidence imported from external lab must be reviewed before findings are updated. |\n'
} >> "$INDEX_FILE"

printf 'Imported %s approved artifact(s) into %s\n' "$count" "$(relpath "$IMPORT_DIR")"
if [ "$skipped" -gt 0 ]; then
  printf 'Skipped %s file(s) with unapproved extensions.\n' "$skipped"
fi
