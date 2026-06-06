# notify.config.example.sh — template for personal notification overrides.
# Copy to notify.config.sh (gitignored) and edit; no restart needed.
#
#   cp .claude/hooks/notify/notify.config.example.sh .claude/hooks/notify/notify.config.sh

# Master switch. false = no notifications at all.
NOTIFY_ENABLED=true

# Audible cue. Visual notification still fires when false.
NOTIFY_SOUND=true

# macOS sound name. Any file in /System/Library/Sounds/ (drop the .aiff).
# Examples: Glass, Ping, Submarine, Hero, Funk.
NOTIFY_SOUND_MACOS="Glass"

# Title prefix shown in the notification window.
NOTIFY_TITLE="Claude Code"
