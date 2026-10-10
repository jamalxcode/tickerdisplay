#!/usr/bin/env python3
"""Refresh config.json with the top 30 stocks, crypto and commodities by trading volume.

Stocks:      TradingView screener, US common stocks ranked by 30-day average dollar volume.
Crypto:      CoinGecko, ranked by 24h volume (stablecoins skipped), mapped to a TradingView pair.
Commodities: fixed ranking by typical global futures volume (no free live source),
             using free CFD symbols that TradingView widgets are allowed to show.
"""
import json, urllib.request
from pathlib import Path

CONFIG = Path(__file__).parent / "config.json"
TOP = 30
UA = {"User-Agent": "Mozilla/5.0 TickerBar", "Content-Type": "application/json"}

COMMODITIES = [  # (TradingView symbol, label) — most traded first
    ("TVC:USOIL", "WTI CRUDE"), ("TVC:UKOIL", "BRENT"), ("TVC:GOLD", "GOLD"),
    ("CAPITALCOM:NATURALGAS", "NAT GAS"), ("TVC:SILVER", "SILVER"), ("CAPITALCOM:XCUUSD", "COPPER"),
    ("CAPITALCOM:SOYBEAN", "SOYBEANS"), ("CAPITALCOM:CORN", "CORN"), ("CAPITALCOM:GASOLINE", "GASOLINE"),
    ("CAPITALCOM:HEATINGOIL", "HEATING OIL"), ("CAPITALCOM:GASOIL", "GASOIL"), ("CAPITALCOM:SOYBEANOIL", "SOY OIL"),
    ("CAPITALCOM:SOYBEANMEAL", "SOY MEAL"), ("CAPITALCOM:WHEAT", "WHEAT"), ("CAPITALCOM:SB", "SUGAR"),
    ("TVC:PLATINUM", "PLATINUM"), ("CAPITALCOM:LIVECATTLE", "LIVE CATTLE"), ("CAPITALCOM:COFFEE", "COFFEE"),
    ("CAPITALCOM:COCOA", "COCOA"), ("CAPITALCOM:LEANHOGS", "LEAN HOGS"), ("CAPITALCOM:COTTON", "COTTON"),
    ("PEPPERSTONE:ALUMINIUM", "ALUMINIUM"), ("TVC:PALLADIUM", "PALLADIUM"), ("CAPITALCOM:NICKEL", "NICKEL"),
    ("PEPPERSTONE:ZINC", "ZINC"), ("CAPITALCOM:TSI", "IRON ORE"), ("CAPITALCOM:FEEDERCATTLE", "FEEDER CATTLE"),
    ("CAPITALCOM:LEAD", "LEAD"), ("CAPITALCOM:OATS", "OATS"), ("CAPITALCOM:RR", "ROUGH RICE"),
]

STABLES = {"USDT", "USDC", "DAI", "FDUSD", "TUSD", "USDE", "USDS", "PYUSD", "USD1", "USDG", "RLUSD", "USDD",
           "BUSD", "FRAX", "GUSD", "USDP", "EURC", "XAUT", "PAXG", "USDTB", "USDF", "BFUSD"}


def get(url, body=None):
    data = json.dumps(body).encode() if body is not None else None
    return json.load(urllib.request.urlopen(urllib.request.Request(url, data=data, headers=UA), timeout=30))


def top_stocks():
    res = get("https://scanner.tradingview.com/america/scan", {
        "columns": ["name", "description", "close", "average_volume_30d_calc"],
        "filter": [{"left": "type", "operation": "equal", "right": "stock"},
                   {"left": "is_primary", "operation": "equal", "right": True}],
        "sort": {"sortBy": "Value.Traded", "sortOrder": "desc"}, "range": [0, 400]})
    rows = []
    for r in res["data"]:
        name, desc, close, avg = r["d"]
        if close and avg:
            rows.append((close * avg, r["s"], name, desc))
    rows.sort(reverse=True)
    out, seen = [], set()
    for _, sym, name, desc in rows:
        company = desc.split(" Inc")[0].split(" Class")[0].split(",")[0]   # GOOG/GOOGL → one Alphabet
        if company in seen:
            continue
        seen.add(company)
        out.append([sym, name])
        if len(out) == TOP:
            break
    return out


def top_crypto():
    coins = get("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=volume_desc&per_page=150&page=1")
    pairs = []
    for c in coins:
        sym, price = c["symbol"].upper(), c.get("current_price") or 0
        if sym in STABLES or 0.97 < price < 1.03 or not sym.isalnum():
            continue
        pairs.append(sym)
    # keep only coins TradingView actually lists, preferring Binance, then Coinbase, then Bybit
    cands = [f"{ex}:{s}{q}" for s in pairs for ex, q in (("BINANCE", "USDT"), ("COINBASE", "USD"), ("BYBIT", "USDT"))]
    listed = {r["s"] for r in get("https://scanner.tradingview.com/crypto/scan",
                                  {"symbols": {"tickers": cands}, "columns": ["close"]})["data"]}
    out = []
    for s in pairs:
        pick = next((p for p in (f"BINANCE:{s}USDT", f"COINBASE:{s}USD", f"BYBIT:{s}USDT") if p in listed), None)
        if pick and all(p[1] != s for p in out):
            out.append([pick, s])
        if len(out) == TOP:
            break
    return out


def main():
    cfg = json.loads(CONFIG.read_text())
    stocks, crypto = top_stocks(), top_crypto()
    rows = {r["label"]: r for r in cfg["rows"]}
    markets = rows.get("MARKETS", {"label": "MARKETS", "symbols": []})
    for r in cfg["rows"]:          # FX pairs move into the markets row
        for sym in r["symbols"]:
            if sym[0].startswith("FX:") and sym not in markets["symbols"]:
                markets["symbols"].append(sym)
    cfg["rows"] = [
        markets,
        {"label": "TOP 30 STOCKS", "symbols": stocks},
        {"label": "TOP 30 CRYPTO", "symbols": crypto},
        {"label": "TOP 30 COMMODITIES", "symbols": [list(c) for c in COMMODITIES[:TOP]]},
    ]
    CONFIG.write_text(json.dumps(cfg, indent=2, ensure_ascii=False))
    print("stocks:", " ".join(s[1] for s in stocks))
    print("crypto:", " ".join(s[1] for s in crypto))
    print("commodities:", len(COMMODITIES[:TOP]))


if __name__ == "__main__":
    main()
