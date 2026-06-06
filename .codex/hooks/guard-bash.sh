#!/usr/bin/env bash
# guard-bash.sh — PreToolUse guard that blocks catastrophic Bash commands.
#
# Wired into .claude/settings.json as a PreToolUse hook (matcher: "Bash").
# Reads the tool-call JSON on stdin, extracts the command, and exits 2 with a
# reason on stderr when it matches a known-dangerous pattern. Exit 2 tells
# Claude Code to BLOCK the call and feed the stderr text back to the model.
#
# This is a safety net (especially when defaultMode is bypassPermissions), not
# a sandbox. It only catches the obvious, near-never-intentional disasters.
# Tune GUARD_PATTERNS for your project.

# --- read stdin payload ---
STDIN_JSON="$(cat 2>/dev/null || true)"
[ -n "$STDIN_JSON" ] || exit 0   # nothing to inspect

# --- extract tool name + command (jq if present, else best-effort grep) ---
if command -v jq >/dev/null 2>&1; then
  TOOL="$(printf '%s' "$STDIN_JSON" | jq -r '.tool_name // empty' 2>/dev/null)"
  CMD="$(printf '%s' "$STDIN_JSON" | jq -r '.tool_input.command // empty' 2>/dev/null)"
else
  TOOL="$(printf '%s' "$STDIN_JSON" | grep -oE '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed -E 's/.*"([^"]*)"$/\1/')"
  CMD="$(printf '%s' "$STDIN_JSON" | grep -oE '"command"[[:space:]]*:[[:space:]]*"([^"\\]|\\.)*"' | head -1 | sed -E 's/^"command"[[:space:]]*:[[:space:]]*"//; s/"$//')"
fi

# Only guard Bash. (Empty TOOL = unknown payload shape; still inspect CMD.)
[ -n "$TOOL" ] && [ "$TOOL" != "Bash" ] && exit 0
[ -n "$CMD" ] || exit 0

# Normalize whitespace for matching.
NORM="$(printf '%s' "$CMD" | tr '\n\t' '  ' | sed -E 's/  +/ /g')"

# --- dangerous patterns (extended regex, case-insensitive) ---
# Each entry: "regex|||human-readable reason"
GUARD_PATTERNS=(
  'rm[[:space:]]+(-[a-z]*[rf][a-z]*[[:space:]]+)+(/|/\*|~|~/|\$HOME)([[:space:]]|$)|||rm -rf on / ~ or $HOME — would wipe the filesystem or home dir'
  ':\(\)[[:space:]]*\{[[:space:]]*:[[:space:]]*\|[[:space:]]*:[[:space:]]*&[[:space:]]*\}[[:space:]]*;[[:space:]]*:|||fork bomb'
  'mkfs(\.[a-z0-9]+)?[[:space:]]|||mkfs — formats a filesystem'
  'dd[[:space:]]+.*of=/dev/(disk|sd|nvme|hd)|||dd writing to a raw disk device'
  '>[[:space:]]*/dev/(disk|sd|nvme|hd)[a-z0-9]*|||redirect overwriting a raw disk device'
  'chmod[[:space:]]+(-[a-z]*R[a-z]*[[:space:]]+)?(0?777)[[:space:]]+/([[:space:]]|$)|||chmod 777 on / — opens up the whole filesystem'
  '(curl|wget)[[:space:]]+.*\|[[:space:]]*(sudo[[:space:]]+)?(ba)?sh([[:space:]]|$)|||piping a remote script straight into a shell'
)

# Special case: git force-push. ERE has no negative lookahead, so handle the
# "force but not force-with-lease" exception in code.
if printf '%s' "$NORM" | grep -iqE 'git[[:space:]]+push' \
  && printf '%s' "$NORM" | grep -iqE '(--force([[:space:]]|$)|[[:space:]]-f([[:space:]]|$))' \
  && ! printf '%s' "$NORM" | grep -iqE -- '--force-with-lease'; then
  printf 'BLOCKED by guard-bash.sh: %s\n  command: %s\n' \
    'git push --force (use --force-with-lease instead)' "$CMD" >&2
  exit 2
fi

for entry in "${GUARD_PATTERNS[@]}"; do
  regex="${entry%%|||*}"
  reason="${entry##*|||}"
  if printf '%s' "$NORM" | grep -iqE "$regex" 2>/dev/null; then
    printf 'BLOCKED by guard-bash.sh: %s\n  command: %s\n' "$reason" "$CMD" >&2
    exit 2
  fi
done

exit 0
