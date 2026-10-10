# Ticker Display

A CNBC-style scrolling market and news ticker for a slim 1920×480 bar display on macOS.

- **Status bar:** clock, NYSE open / pre-market / after-hours / closed, world clocks
- **Four price rows** (TradingView Ticker Tape widgets): markets & FX, top 30 stocks, top 30 crypto, top 30 commodities
- **News crawl:** 28 RSS sources, prioritised by region — Kuwait → neighbours (Saudi Arabia, Iraq) → Arab world → Iran → wider region → general — with coloured region badges and English + Arabic keyword matching

## Requirements

- macOS, Python 3 (built-in `/usr/bin/python3`, no packages needed)
- Brave Browser (launched in kiosk mode with its own profile, so your normal browser is untouched)

## Usage

```bash
./start.sh            # start the server and open the ticker full-screen on the bar display
./stop.sh             # close it
./install.sh          # start at login + refresh the top-30 lists every Sunday 12:00
python3 update_lists.py   # re-rank the top-30 stocks and crypto now
```

`start.sh` finds the bar display by name (`DISPLAY_NAME`, default `LC0801`) and places the window by code — no mouse needed.

## Files

| File | Purpose |
|---|---|
| `index.html` | The ticker page (layout, clocks, news crawl, TradingView widgets) |
| `server.py` | Local server on port 8787: serves the page and relays/prioritises news at `/news` |
| `config.json` | Ticker symbols, news sources, region keywords and per-region slot counts, world clocks |
| `update_lists.py` | Rebuilds the top-30 rows: stocks by 30-day avg dollar volume (TradingView screener), crypto by 24h volume (CoinGecko, stablecoins skipped), commodities by typical futures volume |
| `weekly_update.sh` | Weekly job: back up config, refresh lists, reload the bar |
| `launchd/` | Login and weekly job templates used by `install.sh` |

## Customising

Edit `config.json`:

- `rows` — ticker tape rows (`[TradingView symbol, label]`)
- `news` — RSS sources (`limit` = max headlines per source)
- `news_priority` — region tiers: keywords, badge colour, `max` headlines per tier
- `news_general_max`, `news_max_age_hours`

Scroll speed of the news crawl: `--news-speed` at the top of `index.html`.
