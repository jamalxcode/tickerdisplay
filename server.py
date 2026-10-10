#!/usr/bin/env python3
"""Tiny local server for the ticker bar: serves index.html and relays RSS news (/news).

Headlines are tagged by region using the keyword tiers in config.json ("news_priority")
and sorted tier first (Kuwait → neighbours → Arab world → Iran → region → general), newest first.
"""
import json, time, html, re, threading, urllib.request, xml.etree.ElementTree as ET
from concurrent.futures import ThreadPoolExecutor
from email.utils import parsedate_to_datetime
from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler
from pathlib import Path

PORT = 8787
ROOT = Path(__file__).parent
REFRESH = 300          # seconds between feed fetches
MAX_ITEMS = 90
_cache = {"t": 0, "items": []}

AR = "ء-ي"   # Arabic letters
AR_PREFIX = "(?:[وفبلك]?ال|[وفبلك])?"
AR_SUFFIX = "(?:ي|ية|يون|يين)?"


def config():
    return json.loads((ROOT / "config.json").read_text())


def keyword_regex(words):
    parts = []
    for w in words:
        if re.search(f"[{AR}]", w):   # Arabic: allow attached prefixes/suffixes, but not longer words
            parts.append(f"(?<![{AR}]){AR_PREFIX}{re.escape(w)}{AR_SUFFIX}(?![{AR}])")
        else:
            parts.append(r"\b" + re.escape(w) + r"(?!\w)")
    return re.compile("|".join(parts), re.I)


def clean(s):
    return html.unescape(re.sub(r"<[^>]+>", "", s or "")).strip()


def fetch(feed):
    req = urllib.request.Request(feed["url"], headers={"User-Agent": "Mozilla/5.0 TickerBar"})
    root = ET.fromstring(urllib.request.urlopen(req, timeout=12).read())
    out = []
    for it in root.iter("item"):
        title, source = clean(it.findtext("title")), feed["name"]
        src_el = it.find("source")
        if src_el is not None and src_el.text:            # Google News: "Headline - Publisher"
            source = src_el.text.strip()
            title = re.sub(r"\s+-\s+" + re.escape(source) + r"$", "", title)
        try:
            ts = parsedate_to_datetime(it.findtext("pubDate")).timestamp()
        except Exception:
            ts = 0
        if title:
            out.append({"source": source, "title": title, "ts": ts})
    return out[: feed.get("limit", 8)]


def refresh():
    cfg = config()
    tiers = [(t["region"], t["color"], keyword_regex(t["keywords"])) for t in cfg.get("news_priority", [])]
    oldest = time.time() - cfg.get("news_max_age_hours", 48) * 3600

    def safe_fetch(f):
        try:
            return fetch(f)
        except Exception as e:
            print("feed failed:", f["name"], e)
            return []

    with ThreadPoolExecutor(8) as pool:
        items = [i for batch in pool.map(safe_fetch, cfg["news"]) for i in batch]

    seen, out = set(), []
    for i in sorted(items, key=lambda i: -i["ts"]):
        key = re.sub(r"\W+", "", i["title"].lower())[:60]
        if key in seen or (i["ts"] and i["ts"] < oldest):
            continue
        seen.add(key)
        i["tier"], i["region"], i["color"] = len(tiers), "", ""
        for n, (region, color, rx) in enumerate(tiers):
            if rx.search(i["title"]):
                i["tier"], i["region"], i["color"] = n, region, color
                break
        out.append(i)
    out.sort(key=lambda i: (i["tier"], -i["ts"]))
    caps = [t.get("max", 12) for t in cfg.get("news_priority", [])] + [cfg.get("news_general_max", 20)]
    used, picked = [0] * len(caps), []
    for i in out:                       # each tier gets its own share of the crawl
        if used[i["tier"]] < caps[i["tier"]]:
            used[i["tier"]] += 1
            picked.append(i)
    if picked:
        _cache.update(t=time.time(), items=picked[:MAX_ITEMS])


def refresher():
    """Fetch feeds in the background so requests never wait on the network."""
    while True:
        try:
            refresh()
        except Exception as e:
            print("refresh failed:", e)
        time.sleep(REFRESH)


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *a, **k):
        super().__init__(*a, directory=str(ROOT), **k)

    def do_GET(self):
        if self.path.startswith("/news"):
            body = json.dumps(_cache["items"], ensure_ascii=False).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.end_headers()
            self.wfile.write(body)
        else:
            super().do_GET()

    def end_headers(self):
        self.send_header("Cache-Control", "no-store")   # always serve fresh page/config
        super().end_headers()

    def log_message(self, *a):
        pass


if __name__ == "__main__":
    print(f"Ticker bar on http://localhost:{PORT}")
    threading.Thread(target=refresher, daemon=True).start()
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
