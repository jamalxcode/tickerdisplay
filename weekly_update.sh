#!/bin/zsh
# Weekly: refresh the top crypto list, then reload the ticker bar so they show up.
DIR="${0:A:h}"
echo "=== $(date) ==="
cp "$DIR/config.json" "$DIR/config.lastweek.json"
if "$DIR/tickerbar" update-lists; then
  "$DIR/start.sh"
else
  echo "Update failed; keeping the existing lists"
  cp "$DIR/config.lastweek.json" "$DIR/config.json"
fi
