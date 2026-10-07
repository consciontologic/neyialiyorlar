#!/usr/bin/env bash
# xops/test/run_tests.sh — Run the xops/makefile test suite
# Tests for: track_ops.py, git_ops.py, roadmap_ops.py, doctor.py

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "🧪 Running xops test suite..."
cd "$REPO_ROOT"

# Test that key Python modules can be imported
python3 -c "import sys; sys.path.insert(0, 'xops/makefile'); from track_ops import *; print('✓ track_ops imports')"
python3 -c "import sys; sys.path.insert(0, 'xops/makefile'); from git_ops import *; print('✓ git_ops imports')"
python3 -c "import sys; sys.path.insert(0, 'xops/makefile'); from roadmap_ops import *; print('✓ roadmap_ops imports')"
python3 -c "import sys; sys.path.insert(0, 'xops/makefile'); from doctor import *; print('✓ doctor imports')"

# Test that make targets are wired
echo "✓ Verifying make targets..."
make help | grep -q "make git" && echo "✓ git target exists"
make help | grep -q "make track.add" && echo "✓ track.add target exists"
make help | grep -q "make roadmap.status" && echo "✓ roadmap.status target exists"
make help | grep -q "make doctor" && echo "✓ doctor target exists"

# Discover and run any test_*.sh unit tests (framework bash tests).
TEST_FILES=$(find "$SCRIPT_DIR" -maxdepth 1 -name 'test_*.sh' | sort)
if [ -n "$TEST_FILES" ]; then
  echo "✓ Running test_*.sh unit tests..."
  for tf in $TEST_FILES; do
    echo "  → $(basename "$tf")"
    bash "$tf"
  done
fi

echo "✅ All xops tests passed"
