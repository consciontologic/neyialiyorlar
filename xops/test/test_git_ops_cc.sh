#!/usr/bin/env bash
# xops/test/test_git_ops_cc.sh
# Tests the Conventional-Commits gate in xops/makefile/git_ops.py.
#
# This validates the pure `is_conventional_commit` helper (the last line of
# defense before `make git` writes a commit subject). No git repo is needed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MAKEFILE_DIR="$REPO_ROOT/xops/makefile"
source "$REPO_ROOT/xops/lib/log.sh"

PASS=0; FAIL=0

# assert_cc <subject> <expected: 0=valid|1=invalid>
assert_cc() {
  local subject="$1" want="$2"
  local got=0
  PYTHONPATH="$MAKEFILE_DIR" python3 -c "
import sys
from git_ops import is_conventional_commit
sys.exit(0 if is_conventional_commit(sys.argv[1]) else 1)
" "$subject" >/dev/null 2>&1 || got=$?
  if [[ "$got" -eq "$want" ]]; then
    log_ok "  PASS: ($([[ $want -eq 0 ]] && echo valid || echo invalid)) '$subject'"
    (( PASS++ )) || true
  else
    log_err "  FAIL: '$subject' expected rc=$want got rc=$got"
    (( FAIL++ )) || true
  fi
}

# ── valid Conventional Commits subjects ───────────────────────────────────
assert_cc "feat(auth): add JWT validation middleware" 0
assert_cc "fix: correct null deref" 0
assert_cc "refactor(api)!: drop v1 routes" 0
assert_cc "docs(skills): expand MCP setup" 0
assert_cc "chore: bump deps" 0

# ── invalid subjects (must be rejected) ───────────────────────────────────
assert_cc "added a thing without a type" 1
assert_cc "feature(auth): wrong type word" 1
assert_cc "feat add colon-less subject" 1
assert_cc "feat:" 1
assert_cc "" 1

printf '\n'
log_step "git_ops conventional-commits: %d passed, %d failed" "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
