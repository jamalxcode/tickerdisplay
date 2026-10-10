#!/bin/zsh
# Launch the ticker bar on the slim display — no mouse needed.
DIR="${0:A:h}"
DISPLAY_NAME="LC0801"          # part of the bar display's name
PORT=8787
PROFILE="$HOME/.tickerbar-brave-profile"

# 1. build the Go program if it's missing or the code changed (then restart the server)
export PATH="/opt/homebrew/bin:/usr/local/go/bin:/usr/local/bin:$PATH"
if [[ ! -x "$DIR/tickerbar" || -n "$(find "$DIR" -maxdepth 1 \( -name '*.go' -o -name go.mod \) -newer "$DIR/tickerbar")" ]]; then
  (cd "$DIR" && go build -o tickerbar .) || { echo "Build failed"; exit 1; }
  pkill -f "$DIR/tickerbar serve"; sleep 1
fi

# 2. start the local server if it isn't already running
if ! curl -s -o /dev/null "http://localhost:$PORT/config.json"; then
  nohup "$DIR/tickerbar" serve > "$DIR/server.log" 2>&1 &
  for i in {1..20}; do curl -s -o /dev/null "http://localhost:$PORT/config.json" && break; sleep 0.25; done
fi

# 3. find the bar display's position (converted to top-left screen coordinates)
#    (at login the display can take a moment to appear, so keep trying for ~90s)
for attempt in {1..45}; do
  read X Y W H < <(osascript -l JavaScript -e "
ObjC.import('AppKit');
var s = \$.NSScreen.screens, main = s.objectAtIndex(0).frame, r = '';
for (var i = 0; i < s.count; i++) {
  var sc = s.objectAtIndex(i), f = sc.frame;
  if (ObjC.unwrap(sc.localizedName).indexOf('$DISPLAY_NAME') >= 0)
    r = [f.origin.x, main.size.height - f.origin.y - f.size.height, f.size.width, f.size.height].join(' ');
}
r;")
  [[ -n "$X" ]] && break
  sleep 2
done
if [[ -z "$X" ]]; then echo "Bar display '$DISPLAY_NAME' not found"; exit 1; fi

# 4. close any previous ticker window, then open a fresh full-screen one on the bar
pkill -f "user-data-dir=$PROFILE" 2>/dev/null; sleep 1

# never show the toolbar/title bar in full screen, no analytics notice (ticker profile only)
"$DIR/tickerbar" brave-prefs "$PROFILE"

open -na "Brave Browser" --args \
  --user-data-dir="$PROFILE" \
  "http://localhost:$PORT" \
  --window-position="$X,$Y" --window-size="$W,$H" \
  --kiosk --no-first-run --no-default-browser-check \
  --disable-session-crashed-bubble --noerrdialogs --hide-crash-restore-bubble
echo "Ticker bar launched at $X,$Y (${W}x${H})"
