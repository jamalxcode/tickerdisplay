#!/bin/zsh
# Weekly: refresh the top-30 lists, then reload the ticker bar so they show up.
DIR="${0:A:h}"
echo "=== $(date) ==="
cp "$DIR/config.json" "$DIR/config.lastweek.json"
if /usr/bin/python3 -I "$DIR/update_lists.py"; then
  "$DIR/start.sh"
else
  echo "Update failed; keeping the existing lists"
  cp "$DIR/config.lastweek.json" "$DIR/config.json"
fi
