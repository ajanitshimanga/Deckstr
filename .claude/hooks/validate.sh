#!/usr/bin/env bash
# Validates the project (build + test) only when source files have changed
# since the last successful validation. This avoids blocking pure-chat turns
# with unnecessary work.
#
# Detection strategy: a stamp file is touched after a successful run. On the
# next invocation, find any tracked source files newer than the stamp. If
# none, skip. If any, validate and re-stamp on success.

set -e

# Fall back to script's parent directory if env var isn't set (manual invocation)
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "$0")/../.." && pwd)}"
cd "$PROJECT_DIR" || exit 0

STAMP="$PROJECT_DIR/.claude/.last-validation"

# Directories to exclude from change detection - generated code, deps, build output
EXCLUDES=(
  -not -path "./node_modules/*"
  -not -path "./frontend/node_modules/*"
  -not -path "./frontend/wailsjs/*"
  -not -path "./frontend/dist/*"
  -not -path "./build/bin/*"
  -not -path "./.git/*"
)

# Skip if nothing has changed since last successful run
if [ -f "$STAMP" ]; then
  CHANGED=$(find . \
    \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" \) \
    -newer "$STAMP" \
    "${EXCLUDES[@]}" \
    2>/dev/null | head -1)

  if [ -z "$CHANGED" ]; then
    echo "[validate] No source changes since last run - skipping"
    exit 0
  fi
fi

echo "[validate] Source changes detected - running build + tests"

# Run validation; touch stamp only if everything passes
if go build ./... && go test ./... && npm run build --prefix frontend; then
  touch "$STAMP"
  echo "[validate] All checks passed"
else
  echo "[validate] Validation failed"
  exit 1
fi
