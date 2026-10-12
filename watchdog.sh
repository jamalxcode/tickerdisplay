#!/bin/zsh
# Runs every minute: if the bar display is awake but the ticker isn't showing on it
# (or its server is down), relaunch it. Also reloads it when the display wakes from sleep.
# Does nothing while the display sleeps.
DIR="${0:A:h}"
DISPLAY_NAME="LC0801"
PROFILE="$HOME/.tickerbar-brave-profile"
PORT=8787

PIDS=$(pgrep -f "user-data-dir=$PROFILE" | tr '\n' ',')

STATE=$(osascript -l JavaScript -e "
ObjC.import('AppKit'); ObjC.import('CoreGraphics');
var pids = '$PIDS'.split(',').filter(Boolean).map(Number), screens = \$.NSScreen.screens, bar = null;
for (var i = 0; i < screens.count; i++) {
  var sc = screens.objectAtIndex(i);
  if (ObjC.unwrap(sc.localizedName).indexOf('$DISPLAY_NAME') >= 0) bar = sc;
}
var state;
if (!bar) state = 'no-display';
else {
  var id = ObjC.unwrap(bar.deviceDescription.objectForKey('NSScreenNumber'));
  if (\$.CGDisplayIsAsleep(id)) state = 'asleep';
  else {
    var main = screens.objectAtIndex(0).frame, f = bar.frame;
    var x = f.origin.x, y = main.size.height - f.origin.y - f.size.height, h = f.size.height;
    var wins = ObjC.castRefToObject(\$.CGWindowListCopyWindowInfo(1, 0));   // on-screen windows only
    state = 'missing';
    for (var j = 0; j < wins.count; j++) {
      var w = ObjC.deepUnwrap(wins.objectAtIndex(j)), b = w.kCGWindowBounds;
      if (pids.indexOf(w.kCGWindowOwnerPID) >= 0 && b.X == x && b.Y == y && b.Height == h) state = 'ok';
    }
  }
}
state;")

# remember the display state between runs so we can spot it waking up
STATE_FILE="$DIR/.watchdog-state"
LAST=$(cat "$STATE_FILE" 2>/dev/null)
echo "$STATE" > "$STATE_FILE"

if [[ "$LAST" == "asleep" && "$STATE" != "asleep" && "$STATE" != "no-display" ]]; then
  # TradingView widgets don't reconnect after the display sleeps: reload fresh on wake
  echo "$(date '+%F %T') display woke up — reloading ticker"
  "$DIR/start.sh"
elif [[ "$STATE" == "missing" ]] || { [[ "$STATE" == "ok" ]] && ! curl -s -o /dev/null --max-time 5 "http://localhost:$PORT/config.json"; }; then
  echo "$(date '+%F %T') ticker not showing ($STATE) — relaunching"
  "$DIR/start.sh"
fi
