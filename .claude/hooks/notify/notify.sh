#!/usr/bin/env bash
# notify.sh — cross-platform desktop notification for Claude Code hooks.
# Usage: notify.sh <event>     event = "stop" | "idle"
# Reads optional JSON on stdin (Stop hook payload) and uses stop_hook_active to avoid loops.

EVENT="${1:-stop}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Defaults — overridable via notify.config.sh.
NOTIFY_ENABLED=true
NOTIFY_SOUND=true
NOTIFY_SOUND_MACOS="Glass"
NOTIFY_TITLE="Claude Code"

CONFIG_FILE="$SCRIPT_DIR/notify.config.sh"
[ -f "$CONFIG_FILE" ] && . "$CONFIG_FILE"

[ "$NOTIFY_ENABLED" = "true" ] || exit 0

# Stop-hook loop guard: skip if Claude is already in a follow-up Stop pass.
if [ "$EVENT" = "stop" ]; then
  STDIN_JSON="$(cat 2>/dev/null || true)"
  if [ -n "$STDIN_JSON" ]; then
    if command -v jq >/dev/null 2>&1; then
      [ "$(printf '%s' "$STDIN_JSON" | jq -r '.stop_hook_active // false' 2>/dev/null)" = "true" ] && exit 0
    else
      printf '%s' "$STDIN_JSON" | grep -qE '"stop_hook_active"[[:space:]]*:[[:space:]]*true' && exit 0
    fi
  fi
fi

PROJECT="$(basename "${CLAUDE_PROJECT_DIR:-$PWD}")"
case "$EVENT" in
  stop) BODY="Done — $PROJECT" ;;
  idle) BODY="Waiting for input — $PROJECT" ;;
  *)    BODY="$EVENT — $PROJECT" ;;
esac

# Strip quotes that would break inline AppleScript / PowerShell strings.
SAFE_BODY="${BODY//\"/}"; SAFE_BODY="${SAFE_BODY//\'/}"
SAFE_TITLE="${NOTIFY_TITLE//\"/}"; SAFE_TITLE="${SAFE_TITLE//\'/}"

notify_macos() {
  local sound=""
  [ "$NOTIFY_SOUND" = "true" ] && [ -n "$NOTIFY_SOUND_MACOS" ] && sound=" sound name \"$NOTIFY_SOUND_MACOS\""
  osascript -e "display notification \"$SAFE_BODY\" with title \"$SAFE_TITLE\"$sound" >/dev/null 2>&1
}

notify_linux() {
  if command -v notify-send >/dev/null 2>&1; then
    notify-send "$SAFE_TITLE" "$SAFE_BODY" >/dev/null 2>&1
    if [ "$NOTIFY_SOUND" = "true" ] && command -v canberra-gtk-play >/dev/null 2>&1; then
      canberra-gtk-play -i message-new-instant >/dev/null 2>&1 &
    fi
  else
    printf '\a' >&2
  fi
}

notify_windows() {
  if ! command -v powershell.exe >/dev/null 2>&1; then
    printf '\a' >&2
    return
  fi
  local ps="if (Get-Module -ListAvailable -Name BurntToast) { New-BurntToastNotification -Text '$SAFE_TITLE','$SAFE_BODY' } else { Add-Type -AssemblyName System.Windows.Forms | Out-Null; [System.Windows.Forms.MessageBox]::Show('$SAFE_BODY','$SAFE_TITLE') | Out-Null }"
  powershell.exe -NoProfile -NonInteractive -Command "$ps" >/dev/null 2>&1 || printf '\a' >&2
}

OS_NAME="$(uname -s 2>/dev/null || echo Unknown)"
case "$OS_NAME" in
  Darwin*)
    notify_macos
    ;;
  Linux*)
    if [ -r /proc/version ] && grep -qi microsoft /proc/version; then
      notify_windows
    else
      notify_linux
    fi
    ;;
  MINGW*|MSYS*|CYGWIN*)
    notify_windows
    ;;
  *)
    printf '\a' >&2
    ;;
esac

exit 0
