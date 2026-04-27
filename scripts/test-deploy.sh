#!/usr/bin/env bash
# Integration test: 6 deploy scenarios (plain/enc-stdin/enc-flag) × (no prefix/with prefix).
#
# Environment variables (all optional if defaults are fine):
#   CUBBIT_ACCESS_KEY   — default: minioadmin
#   CUBBIT_SECRET_KEY   — default: minioadmin
#   CUBBIT_BUCKET       — required (or use --bucket)
#   CUBBIT_ENDPOINT     — default: https://s3.cubbit.eu
#   CUBBIT_PAGES_BIN    — path to cubbit-pages binary
#   CUBBIT_PAGES_SITE   — path to test site directory
#   CUBBIT_PAGES_PASSWORD  — encryption password for tests
#   CUBBIT_PAGES_PREFIX    — prefix used for prefix tests
#
# Flags:
#   --bucket   <name>
#   --endpoint <url>
#   --binary   <path>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BIN="${CUBBIT_PAGES_BIN:-$ROOT/bin/cubbit-pages}"
SITE="${CUBBIT_PAGES_SITE:-$ROOT/testdata/site}"
PASSWORD="${CUBBIT_PAGES_PASSWORD:-test-password-123}"
PREFIX="${CUBBIT_PAGES_PREFIX:-test-prefix}"
ENDPOINT="${CUBBIT_ENDPOINT:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bucket)   BUCKET="$2";   shift 2 ;;
    --endpoint) ENDPOINT="$2"; shift 2 ;;
    --binary)   BIN="$2";      shift 2 ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

BUCKET="${BUCKET:-${CUBBIT_BUCKET:-}}"
AK="${CUBBIT_ACCESS_KEY:-minioadmin}"
SK="${CUBBIT_SECRET_KEY:-minioadmin}"

if [[ -z "$BUCKET" ]]; then
  echo "Error: bucket required (--bucket or CUBBIT_BUCKET)" >&2
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  echo "Error: binary not found or not executable: $BIN" >&2
  exit 1
fi
if [[ ! -d "$SITE" ]]; then
  echo "Error: test site not found: $SITE" >&2
  exit 1
fi

PASS=0
FAIL=0

run_test() {
  local name="$1"
  shift
  echo -n "  $name ... "
  if output=$("$@" 2>&1); then
    echo "OK"
    PASS=$((PASS + 1))
  else
    echo "FAIL"
    echo "$output" | sed 's/^/    /'
    FAIL=$((FAIL + 1))
  fi
}

echo ""
echo "cubbit-pages integration tests"
echo "  binary:   $BIN"
echo "  bucket:   $BUCKET"
echo "  site:     $SITE"
echo "  endpoint: ${ENDPOINT:-default}"
echo ""

BASE_FLAGS=(--access-key "$AK" --secret-key "$SK" --bucket "$BUCKET")
if [[ -n "$ENDPOINT" ]]; then
  BASE_FLAGS+=(--endpoint "$ENDPOINT")
fi

echo "── No prefix ──────────────────────────────────"

run_test "1. plain" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}"

run_test "2. encrypt (password via stdin)" \
  bash -c "printf '%s\n' '$PASSWORD' | '$BIN' deploy '$SITE' ${BASE_FLAGS[*]} --encrypt"

run_test "3. encrypt (--password flag)" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}" --encrypt --password "$PASSWORD"

echo ""
echo "── With --prefix $PREFIX ──────────────────────"

run_test "4. plain + prefix" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}" --prefix "$PREFIX"

run_test "5. encrypt stdin + prefix" \
  bash -c "printf '%s\n' '$PASSWORD' | '$BIN' deploy '$SITE' ${BASE_FLAGS[*]} --encrypt --prefix '$PREFIX'"

run_test "6. encrypt flag + prefix" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}" --encrypt --password "$PASSWORD" --prefix "$PREFIX"

echo ""
echo "────────────────────────────────────────────────"
echo "  Passed: $PASS / $((PASS + FAIL))"
if [[ $FAIL -gt 0 ]]; then
  echo "  Failed: $FAIL"
  exit 1
fi
echo ""
