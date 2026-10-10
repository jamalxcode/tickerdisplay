// Command tickerbar runs the ticker bar: a local page/news server, the weekly
// top-30 list refresh, and the Brave profile tweaks used by start.sh.
//
//	tickerbar serve                  serve index.html and /news on 127.0.0.1:8787
//	tickerbar update-lists           re-rank the top-30 stocks and crypto in config.json
//	tickerbar brave-prefs <profile>  hide Brave's full-screen toolbar and analytics notice
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config mirrors config.json. Field order here is the order written back to disk.
type Config struct {
	Rows            []Row      `json:"rows"`
	News            []Feed     `json:"news"`
	Clocks          [][]string `json:"clocks"`
	NewsPriority    []Tier     `json:"news_priority"`
	NewsMaxAgeHours float64    `json:"news_max_age_hours"`
	NewsGeneralMax  int        `json:"news_general_max"`
}

type Row struct {
	Label   string     `json:"label"`
	Symbols [][]string `json:"symbols"` // [TradingView symbol, label]
}

type Feed struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Limit int    `json:"limit,omitempty"`
}

type Tier struct {
	Region   string   `json:"region"`
	Color    string   `json:"color"`
	Keywords []string `json:"keywords"`
	Max      int      `json:"max"`
}

// baseDir is the folder holding config.json and index.html: the binary's own folder.
func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

func configPath() string { return filepath.Join(baseDir(), "config.json") }

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	cfg := &Config{NewsMaxAgeHours: 48, NewsGeneralMax: 20}
	return cfg, json.Unmarshal(data, cfg)
}

func saveConfig(cfg *Config) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep "S&P 500" and URLs readable
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(configPath(), buf.Bytes(), 0o644)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tickerbar serve | update-lists | brave-prefs <profile-dir>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve()
	case "update-lists":
		err = updateLists()
	case "brave-prefs":
		if len(os.Args) < 3 {
			err = fmt.Errorf("brave-prefs needs the profile folder")
		} else {
			err = bravePrefs(os.Args[2])
		}
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tickerbar:", err)
		os.Exit(1)
	}
}
