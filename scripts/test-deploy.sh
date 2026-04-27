#!/usr/bin/env bash
# Integration test: 6 deploy scenarios (plain/enc-stdin/enc-flag) × (no prefix/with prefix).
# After each encrypted deploy, downloads _verify.enc from the bucket and decrypts it with
# Node.js (same Web Crypto API used by the service worker) to verify the full roundtrip.
#
# Environment variables (all optional if defaults are fine):
#   CUBBIT_ACCESS_KEY      — default: minioadmin
#   CUBBIT_SECRET_KEY      — default: minioadmin
#   CUBBIT_BUCKET          — required (or use --bucket)
#   CUBBIT_ENDPOINT        — default: https://s3.cubbit.eu
#   CUBBIT_PAGES_BIN       — path to cubbit-pages binary
#   CUBBIT_PAGES_SITE      — path to test site directory
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
  echo "Error: bucket required (--bucket or CUBBIT_BUCKET)" >&2; exit 1
fi
if [[ ! -x "$BIN" ]]; then
  echo "Error: binary not found or not executable: $BIN" >&2; exit 1
fi
if [[ ! -d "$SITE" ]]; then
  echo "Error: test site not found: $SITE" >&2; exit 1
fi

HAS_NODE=false
if command -v node &>/dev/null; then
  HAS_NODE=true
fi

PASS=0
FAIL=0
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# AWS CLI flags for S3 operations (always needs an endpoint for Cubbit)
S3_ENDPOINT="${ENDPOINT:-https://s3.cubbit.eu}"
S3_FLAGS=(--no-cli-pager --endpoint-url "$S3_ENDPOINT")

run_test() {
  local name="$1"; shift
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

# After an encrypted deploy, downloads _verify.enc and about.html.enc from the bucket
# and decrypts both with Node.js (same Web Crypto API used by the service worker).
verify_decrypt() {
  local label="$1"
  local key_prefix="$2"  # empty or "prefix/"
  local password="$3"

  if ! $HAS_NODE; then
    echo "  ${label} (decrypt verify) ... SKIP (node not found)"
    return
  fi

  local original_about="$SITE/about.html"

  # Check canary (_verify.enc)
  local canary_key="${key_prefix}_verify.enc"
  local canary_file="$TMPDIR/verify_${label}_canary.enc"
  local canary_err
  echo -n "  ${label} (canary) ... "
  if canary_err=$(AWS_ACCESS_KEY_ID="$AK" AWS_SECRET_ACCESS_KEY="$SK" \
        aws s3 cp "s3://$BUCKET/$canary_key" "$canary_file" "${S3_FLAGS[@]}" 2>&1) \
     && canary_err=$(node "$SCRIPT_DIR/verify-decrypt.mjs" "$canary_file" "$password" 2>&1); then
    echo "OK"
    PASS=$((PASS + 1))
  else
    echo "FAIL — $canary_err"
    FAIL=$((FAIL + 1))
  fi

  # Check about.html.enc decrypts to match original about.html
  local asset_key="${key_prefix}about.html.enc"
  local asset_file="$TMPDIR/verify_${label}_about.enc"
  local asset_err
  echo -n "  ${label} (about.html.enc) ... "
  if asset_err=$(AWS_ACCESS_KEY_ID="$AK" AWS_SECRET_ACCESS_KEY="$SK" \
        aws s3 cp "s3://$BUCKET/$asset_key" "$asset_file" "${S3_FLAGS[@]}" 2>&1) \
     && asset_err=$(node "$SCRIPT_DIR/verify-decrypt.mjs" \
          "$asset_file" "$password" --compare "$original_about" 2>&1); then
    echo "OK"
    PASS=$((PASS + 1))
  else
    echo "FAIL — $asset_err"
    FAIL=$((FAIL + 1))
  fi
}

echo ""
echo "cubbit-pages integration tests"
echo "  binary:   $BIN"
echo "  bucket:   $BUCKET"
echo "  site:     $SITE"
echo "  endpoint: ${ENDPOINT:-default}"
echo "  node:     $($HAS_NODE && echo "$(node --version)" || echo "not found — decrypt verify will be skipped")"
echo ""

BASE_FLAGS=(--access-key "$AK" --secret-key "$SK" --bucket "$BUCKET")
if [[ -n "$ENDPOINT" ]]; then
  BASE_FLAGS+=(--endpoint "$ENDPOINT")
fi

echo "── No prefix ──────────────────────────────────"

run_test "1. plain" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}"

run_test "2. encrypt (stdin)" \
  bash -c "printf '%s\n' '$PASSWORD' | '$BIN' deploy '$SITE' ${BASE_FLAGS[*]} --encrypt"
verify_decrypt "2" "" "$PASSWORD"

run_test "3. encrypt (--password)" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}" --encrypt --password "$PASSWORD"
verify_decrypt "3" "" "$PASSWORD"

echo ""
echo "── With --prefix $PREFIX ──────────────────────"

run_test "4. plain + prefix" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}" --prefix "$PREFIX"

run_test "5. encrypt stdin + prefix" \
  bash -c "printf '%s\n' '$PASSWORD' | '$BIN' deploy '$SITE' ${BASE_FLAGS[*]} --encrypt --prefix '$PREFIX'"
verify_decrypt "5" "$PREFIX/" "$PASSWORD"

run_test "6. encrypt flag + prefix" \
  "$BIN" deploy "$SITE" "${BASE_FLAGS[@]}" --encrypt --password "$PASSWORD" --prefix "$PREFIX"
verify_decrypt "6" "$PREFIX/" "$PASSWORD"

echo ""
echo "────────────────────────────────────────────────"
echo "  Passed: $PASS / $((PASS + FAIL))"
if [[ $FAIL -gt 0 ]]; then
  echo "  Failed: $FAIL"
  exit 1
fi
echo ""
