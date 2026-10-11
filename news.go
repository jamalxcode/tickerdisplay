package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Headlines are tagged by region using the keyword tiers in config.json ("news_priority")
// and sorted tier first (Kuwait → neighbours → Arab world → Iran → region → general), newest first.

const (
	maxItems    = 90
	defaultFeed = 8 // headlines kept per feed unless "limit" says otherwise
)

type Item struct {
	Source string  `json:"source"`
	Title  string  `json:"title"`
	TS     float64 `json:"ts"` // unix seconds, 0 if unknown
	Tier   int     `json:"tier"`
	Region string  `json:"region"`
	Color  string  `json:"color"`
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func httpGet(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 TickerBar")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden {
		// Some sites' bot protection blocks Go's TLS fingerprint but not curl's.
		resp.Body.Close()
		return curlGet(url)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s", resp.Status)
	}
	return resp.Body, nil
}

func curlGet(url string) (io.ReadCloser, error) {
	out, err := exec.Command("/usr/bin/curl", "-sfL", "--max-time", "20", "-A", "Mozilla/5.0 TickerBar", url).Output()
	if err != nil {
		return nil, fmt.Errorf("403 Forbidden (curl retry: %v)", err)
	}
	return io.NopCloser(bytes.NewReader(out)), nil
}

// ---------- region keywords ----------

const arLetters = `\x{0621}-\x{064A}`

// keywordRegex matches any keyword as a whole word. Arabic keywords may carry an attached
// prefix (و ف ب ل ك, ال) or suffix (ي ية يون يين) but must not be part of a longer word.
func keywordRegex(words []string) *regexp.Regexp {
	isArabic := regexp.MustCompile(`[` + arLetters + `]`)
	var parts []string
	for _, w := range words {
		q := regexp.QuoteMeta(w)
		if isArabic.MatchString(w) {
			parts = append(parts, `(?:^|[^`+arLetters+`])(?:[وفبلك]?ال|[وفبلك])?`+q+`(?:ي|ية|يون|يين)?(?:$|[^`+arLetters+`])`)
		} else {
			parts = append(parts, `(?:^|[^\pL\pN_])`+q+`(?:$|[^\pL\pN_])`)
		}
	}
	return regexp.MustCompile(`(?i)` + strings.Join(parts, "|"))
}

// ---------- RSS ----------

type rssItem struct {
	Title   string `xml:"title"`
	PubDate string `xml:"pubDate"`
	Source  string `xml:"source"`
}

var (
	tagRe       = regexp.MustCompile(`<[^>]+>`)
	spaceRe     = regexp.MustCompile(`\s+`)
	dateLayouts = []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST", "Mon, 02 Jan 2006 15:04 -0700", time.RFC3339, "2006-01-02 15:04:05"}
)

func clean(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(s, "")), " "))
}

func parseDate(s string) float64 {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return float64(t.Unix())
		}
	}
	return 0
}

func fetchFeed(f Feed) ([]Item, error) {
	body, err := httpGet(f.URL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	dec := xml.NewDecoder(body)
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }

	limit := f.Limit
	if limit == 0 {
		limit = defaultFeed
	}
	var out []Item
	for len(out) < limit {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(out) > 0 {
				break
			}
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "item" {
			continue
		}
		var it rssItem
		if dec.DecodeElement(&it, &start) != nil {
			continue
		}
		title, source := clean(it.Title), f.Name
		if s := clean(it.Source); s != "" { // Google News: "Headline - Publisher"
			source = s
			title = strings.TrimSuffix(title, " - "+s)
		}
		if title != "" {
			out = append(out, Item{Source: source, Title: title, TS: parseDate(it.PubDate)})
		}
	}
	return out, nil
}

// ---------- ranking ----------

var dedupeRe = regexp.MustCompile(`[^\pL\pN]+`)

func dedupeKey(title string) string {
	r := []rune(dedupeRe.ReplaceAllString(strings.ToLower(title), ""))
	if len(r) > 60 {
		r = r[:60]
	}
	return string(r)
}

func collectNews(cfg *Config) []Item {
	type tier struct {
		region, color string
		rx            *regexp.Regexp
	}
	var tiers []tier
	for _, t := range cfg.NewsPriority {
		tiers = append(tiers, tier{t.Region, t.Color, keywordRegex(t.Keywords)})
	}

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		all   []Item
		slots = make(chan struct{}, 8)
	)
	for _, f := range cfg.News {
		wg.Add(1)
		go func(f Feed) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			items, err := fetchFeed(f)
			if err != nil {
				log.Printf("feed failed: %s: %v", f.Name, err)
				return
			}
			mu.Lock()
			all = append(all, items...)
			mu.Unlock()
		}(f)
	}
	wg.Wait()

	sort.SliceStable(all, func(i, j int) bool { return all[i].TS > all[j].TS })
	oldest := float64(time.Now().Unix()) - cfg.NewsMaxAgeHours*3600
	seen := map[string]bool{}
	var out []Item
	for _, it := range all {
		key := dedupeKey(it.Title)
		if seen[key] || (it.TS != 0 && it.TS < oldest) {
			continue
		}
		seen[key] = true
		it.Tier = len(tiers)
		for n, t := range tiers {
			if t.rx.MatchString(it.Title) {
				it.Tier, it.Region, it.Color = n, t.region, t.color
				break
			}
		}
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		return out[i].TS > out[j].TS
	})

	// each tier gets its own share of the crawl
	caps := make([]int, 0, len(cfg.NewsPriority)+1)
	for _, t := range cfg.NewsPriority {
		m := t.Max
		if m == 0 {
			m = 12
		}
		caps = append(caps, m)
	}
	caps = append(caps, cfg.NewsGeneralMax)
	used := make([]int, len(caps))
	picked := []Item{}
	for _, it := range out {
		if used[it.Tier] < caps[it.Tier] && len(picked) < maxItems {
			used[it.Tier]++
			picked = append(picked, it)
		}
	}
	return picked
}
