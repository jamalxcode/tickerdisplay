#!/bin/zsh
# Close the ticker bar window and stop its server.
DIR="${0:A:h}"
pkill -f "user-data-dir=$HOME/.tickerbar-brave-profile"
pkill -f "$DIR/tickerbar serve"
echo "Ticker bar stopped"
