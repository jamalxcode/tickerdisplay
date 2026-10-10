#!/bin/zsh
# Install the login item (start at login) and the weekly list refresh for this folder.
DIR="${0:A:h}"
AGENTS="$HOME/Library/LaunchAgents"
mkdir -p "$AGENTS"
for name in com.tickerbar com.tickerbar.weekly; do
  sed "s|__DIR__|$DIR|g" "$DIR/launchd/$name.plist" > "$AGENTS/$name.plist"
  launchctl bootout "gui/$(id -u)/$name" 2>/dev/null
  launchctl bootstrap "gui/$(id -u)" "$AGENTS/$name.plist"
done
echo "Installed: starts at login, lists refresh Sundays 12:00"
