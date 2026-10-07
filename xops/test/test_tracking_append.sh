#!/usr/bin/env bash
# xops/test/test_tracking_append.sh
# Tests for xops/agent/tracking_append.sh
#
# Each test_ function is called in sequence; a non-zero exit = failure.
# Tests run in isolated temp dirs so docs/tracking/tracking.csv is never touched.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
APPEND="$REPO_ROOT/xops/agent/tracking_append.sh"
source "$REPO_ROOT/xops/lib/log.sh"

# ── helpers ──────────────────────────────────────────────────────────────
PASS=0; FAIL=0

assert_exit() {
  local desc="$1" want="$2"; shift 2
  local got
  got=0; "$@" >/dev/null 2>&1 || got=$?
  if [[ "$got" -eq "$want" ]]; then
    log_ok "  PASS: $desc"
    (( PASS++ )) || true
  else
    log_err "  FAIL: $desc (expected exit $want, got $got)"
    (( FAIL++ )) || true
  fi
}

# Run the script with an isolated CSV via AVB_CSV env override.
with_csv() {
  local csv="$1"; shift
  AVB_CSV="$csv" bash "$APPEND" "$@"
}

# ── test functions ────────────────────────────────────────────────────────

test_valid_note_row_appended() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  with_csv "$csv" \
    --action=note --status=completed \
    --summary="test note" --run-id=run-test-001 >/dev/null 2>&1

  local lines; lines="$(wc -l < "$csv")"
  if [[ "$lines" -eq 2 ]]; then
    log_ok "  PASS: valid note row appended (2 lines total)"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected 2 lines, got $lines"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_valid_commit_row_defaults_to_pending() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  with_csv "$csv" \
    --action=commit --status=completed \
    --summary="feat(x): add thing" --run-id=run-test-002 >/dev/null 2>&1

  local last; last="$(tail -1 "$csv")"
  if [[ "$last" == *"pending"* ]]; then
    log_ok "  PASS: commit row has commit_sha=pending"
    (( PASS++ )) || true
  else
    log_err "  FAIL: commit_sha not set to pending — row: $last"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_missing_action_exits_64() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  local got=0
  with_csv "$csv" --status=completed --summary="oops" >/dev/null 2>&1 || got=$?

  if [[ "$got" -eq 64 ]]; then
    log_ok "  PASS: missing --action exits 64"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected exit 64, got $got"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_invalid_action_value_exits_65() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  local got=0
  with_csv "$csv" --action=INVALID --status=completed --summary="bad" >/dev/null 2>&1 || got=$?

  if [[ "$got" -eq 65 ]]; then
    log_ok "  PASS: invalid --action value exits 65"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected exit 65, got $got"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_invalid_status_value_exits_65() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  local got=0
  with_csv "$csv" --action=note --status=BADSTATUS --summary="test" >/dev/null 2>&1 || got=$?

  if [[ "$got" -eq 65 ]]; then
    log_ok "  PASS: invalid --status value exits 65"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected exit 65, got $got"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_missing_csv_exits_66() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/nonexistent/tracking.csv"
  # Do NOT create the CSV file

  local got=0
  with_csv "$csv" --action=note --status=completed --summary="no csv" >/dev/null 2>&1 || got=$?

  if [[ "$got" -eq 66 ]]; then
    log_ok "  PASS: missing CSV exits 66"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected exit 66, got $got"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_summary_with_comma_is_quoted() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  with_csv "$csv" \
    --action=note --status=completed \
    --summary="one, two, three" --run-id=run-test-003 >/dev/null 2>&1

  local last; last="$(tail -1 "$csv")"
  if [[ "$last" == *'"one, two, three"'* ]]; then
    log_ok "  PASS: summary with commas is CSV-quoted"
    (( PASS++ )) || true
  else
    log_err "  FAIL: summary not quoted — row: $last"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_noncc_commit_summary_exits_65() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  local got=0
  with_csv "$csv" \
    --action=commit --status=completed \
    --summary="added a thing without a type" --run-id=run-test-004 >/dev/null 2>&1 || got=$?

  if [[ "$got" -eq 65 ]]; then
    log_ok "  PASS: non-Conventional-Commits summary on commit row exits 65"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected exit 65, got $got"
    (( FAIL++ )) || true
  fi

  # And confirm the row was NOT appended (header only → 1 line).
  local lines; lines="$(wc -l < "$csv")"
  if [[ "$lines" -eq 1 ]]; then
    log_ok "  PASS: rejected commit row was not appended"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected 1 line (header only), got $lines"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

test_cc_commit_with_bang_and_scope_appended() {
  local tmp; tmp="$(mktemp -d)"
  local csv="$tmp/tracking.csv"
  printf 'ts_utc,run_id,agent,scope,action,status,summary,refs,commit_sha\n' > "$csv"

  with_csv "$csv" \
    --action=commit --status=completed \
    --summary="refactor(api)!: drop v1 routes" --run-id=run-test-005 >/dev/null 2>&1

  local lines; lines="$(wc -l < "$csv")"
  if [[ "$lines" -eq 2 ]]; then
    log_ok "  PASS: breaking-change Conventional-Commits summary appended"
    (( PASS++ )) || true
  else
    log_err "  FAIL: expected 2 lines, got $lines"
    (( FAIL++ )) || true
  fi
  rm -rf "$tmp"
}

# ── run all tests ─────────────────────────────────────────────────────────
test_valid_note_row_appended
test_valid_commit_row_defaults_to_pending
test_missing_action_exits_64
test_invalid_action_value_exits_65
test_invalid_status_value_exits_65
test_missing_csv_exits_66
test_summary_with_comma_is_quoted
test_noncc_commit_summary_exits_65
test_cc_commit_with_bang_and_scope_appended

printf '\n'
log_step "tracking_append: %d passed, %d failed" "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
