#!/usr/bin/env bash
# run.sh — manual integration smoke test for textwatcher.
#
# Usage:
#   ./test/run.sh           # run all checks
#   ./test/run.sh --keep    # keep the temp directory for inspection
#
# The script:
#   1. Builds the binary.
#   2. Creates a temp working directory.
#   3. Starts the watcher in the background.
#   4. Creates a new .md file with Chinese punctuation.
#   5. Waits for normalization, then shows a before/after diff.
#   6. Runs a few more targeted checks.
#   7. Cleans up (unless --keep).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BINARY="/tmp/textwatcher-smoke"
KEEP=false

for arg in "$@"; do
  [[ "$arg" == "--keep" ]] && KEEP=true
done

# ── colours ────────────────────────────────────────────────────────────────
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW="\033[0;33m"
CYAN="\033[0;36m"
RESET="\033[0m"

pass() { echo -e "${GREEN}[PASS]${RESET} $*"; }
fail() { echo -e "${RED}[FAIL]${RESET} $*"; EXIT_CODE=1; }
info() { echo -e "${CYAN}[INFO]${RESET} $*"; }
section() { echo -e "\n${YELLOW}━━ $* ━━${RESET}"; }

EXIT_CODE=0
WATCHER_PID=""

cleanup() {
  [[ -n "$WATCHER_PID" ]] && kill "$WATCHER_PID" 2>/dev/null || true
  if [[ "$KEEP" == true ]]; then
    info "temp dir kept at: $WORK_DIR"
  else
    rm -rf "$WORK_DIR"
  fi
}
trap cleanup EXIT

# ── 1. Build ────────────────────────────────────────────────────────────────
section "Building binary"
cd "$ROOT_DIR"
go build -o "$BINARY" ./cmd/textwatcher
info "binary: $BINARY"

# ── 2. Prepare temp dir ──────────────────────────────────────────────────────
WORK_DIR="$(mktemp -d)"
info "work dir: $WORK_DIR"

# ── 3. Start watcher ─────────────────────────────────────────────────────────
section "Starting watcher"
"$BINARY" --dir "$WORK_DIR" --debounce 200ms --log-level debug 2>&1 \
  | sed 's/^/  [watcher] /' &
WATCHER_PID=$!
sleep 0.5   # wait for fsnotify to register directories

# ── helper: assert file contains / does not contain a string ─────────────────
assert_contains() {
  local file="$1" needle="$2" label="$3"
  if grep -qF -- "$needle" "$file"; then
    pass "$label"
  else
    fail "$label — expected to find: $needle"
    echo "    file content:"
    sed 's/^/    /' "$file"
  fi
}

assert_not_contains() {
  local file="$1" needle="$2" label="$3"
  if ! grep -qF -- "$needle" "$file"; then
    pass "$label"
  else
    fail "$label — did not expect to find: $needle"
    echo "    file content:"
    sed 's/^/    /' "$file"
  fi
}

wait_and_diff() {
  local file="$1" before="$2"
  sleep 0.6   # debounce (200ms) + processing round-trip
  local after
  after="$(cat "$file")"
  if [[ "$before" != "$after" ]]; then
    info "diff (before → after):"
    diff <(echo "$before") <(echo "$after") | sed 's/^/    /' || true
  else
    info "(no changes detected)"
  fi
}

# ── 4. Test: new file created after watcher starts ───────────────────────────
section "Check: new file created after watcher starts"
NEW_FILE="$WORK_DIR/new.md"
BEFORE="你好，世界。这是一个测试！ERP系统和JSON数据。"
echo "$BEFORE" > "$NEW_FILE"
wait_and_diff "$NEW_FILE" "$BEFORE"

assert_not_contains "$NEW_FILE" "，"  "Chinese comma replaced"
assert_not_contains "$NEW_FILE" "。"  "Chinese period replaced"
assert_not_contains "$NEW_FILE" "！"  "Chinese exclamation replaced"
assert_contains     "$NEW_FILE" "ERP 系统"  "CJK/Latin space inserted (ERP)"
assert_contains     "$NEW_FILE" "JSON 数据" "CJK/Latin space inserted (JSON)"

# ── 5. Test: fenced code block must not be modified ──────────────────────────
section "Check: fenced code block preserved"
FENCE_FILE="$WORK_DIR/fence.md"
cat > "$FENCE_FILE" <<'EOF'
outside，paragraph

```go
// 这里不应该改，fmt.Println("你好，世界")
```

outside again，ok
EOF
BEFORE_FENCE="$(cat "$FENCE_FILE")"
wait_and_diff "$FENCE_FILE" "$BEFORE_FENCE"

# Outside the fence: comma replaced
assert_not_contains "$FENCE_FILE" "outside，paragraph" "comma outside fence replaced"
# Inside the fence: content unchanged
assert_contains "$FENCE_FILE" '// 这里不应该改，fmt.Println("你好，世界")' "fence content preserved"

# ── 6. Test: pre-existing file must NOT be touched (no --scan-on-start) ──────
section "Check: pre-existing file NOT modified (no --scan-on-start)"
PRE_FILE="$WORK_DIR/pre_existing.md"
# Write before the watcher started — but watcher is already running, so we
# need to simulate "was there before startup" by having the watcher skip it.
# We restart with a fresh dir for this check.
PRE_DIR="$(mktemp -d)"
PRE_CONTENT="苹果、香蕉、橙子。"
echo "$PRE_CONTENT" > "$PRE_DIR/pre.md"

kill "$WATCHER_PID" 2>/dev/null || true
sleep 0.2

"$BINARY" --dir "$PRE_DIR" --debounce 200ms --log-level info 2>&1 \
  | sed 's/^/  [watcher2] /' &
WATCHER_PID=$!
sleep 0.6   # wait longer than debounce

ACTUAL="$(cat "$PRE_DIR/pre.md")"
if [[ "$ACTUAL" == "$PRE_CONTENT" ]]; then
  pass "pre-existing file was not touched"
else
  fail "pre-existing file was modified unexpectedly"
  diff <(echo "$PRE_CONTENT") <(echo "$ACTUAL") | sed 's/^/    /' || true
fi
rm -rf "$PRE_DIR"

# ── 7. Test: sample fixture ──────────────────────────────────────────────────
section "Check: full sample fixture"
kill "$WATCHER_PID" 2>/dev/null || true
sleep 0.2

FIXTURE_DIR="$(mktemp -d)"
cp "$SCRIPT_DIR/fixtures/sample.md" "$FIXTURE_DIR/sample.md"

"$BINARY" --dir "$FIXTURE_DIR" --debounce 200ms --log-level info 2>&1 \
  | sed 's/^/  [watcher3] /' &
WATCHER_PID=$!
sleep 0.3

# Trigger a write event by appending a newline.
echo "" >> "$FIXTURE_DIR/sample.md"
sleep 0.6

info "normalized sample.md:"
sed 's/^/    /' "$FIXTURE_DIR/sample.md"

assert_contains     "$FIXTURE_DIR/sample.md" "ERP 系统" "sample: ERP spacing"
assert_contains     "$FIXTURE_DIR/sample.md" "JSON 数据" "sample: JSON spacing"
assert_contains     "$FIXTURE_DIR/sample.md" "这个 Agent" "sample: Agent spacing"

# Outside-fence lines must use ASCII punctuation.
assert_contains "$FIXTURE_DIR/sample.md" "这是一个示例文档, " "sample: Chinese comma replaced outside fence"
assert_contains "$FIXTURE_DIR/sample.md" "请务必注意以下几点! " "sample: Chinese exclamation replaced outside fence"

# Fenced code block inside sample must be untouched.
assert_contains "$FIXTURE_DIR/sample.md" 'fmt.Println("你好，世界！")' \
  "sample: fence content preserved"

rm -rf "$FIXTURE_DIR"

# ── Summary ──────────────────────────────────────────────────────────────────
section "Summary"
if [[ "$EXIT_CODE" -eq 0 ]]; then
  echo -e "${GREEN}All checks passed.${RESET}"
else
  echo -e "${RED}Some checks failed.${RESET}"
fi
exit "$EXIT_CODE"
