package main

// Refresh config.json with the top 6 crypto and top 30 commodities by trading volume.
//
// Crypto:      CoinGecko, ranked by 24h volume (stablecoins skipped), mapped to a TradingView pair.
// Commodities: fixed ranking by typical global futures volume (no free live source),
//              using free CFD symbols that TradingView widgets are allowed to show.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const (
	top            = 30
	topCryptoCount = 6 // stablecoins never count
)

var commodities = [][]string{ // [TradingView symbol, label] — most traded first
	{"TVC:USOIL", "WTI CRUDE"}, {"TVC:UKOIL", "BRENT"}, {"TVC:GOLD", "GOLD"},
	{"CAPITALCOM:NATURALGAS", "NAT GAS"}, {"TVC:SILVER", "SILVER"}, {"CAPITALCOM:XCUUSD", "COPPER"},
	{"CAPITALCOM:SOYBEAN", "SOYBEANS"}, {"CAPITALCOM:CORN", "CORN"}, {"CAPITALCOM:GASOLINE", "GASOLINE"},
	{"CAPITALCOM:HEATINGOIL", "HEATING OIL"}, {"CAPITALCOM:GASOIL", "GASOIL"}, {"CAPITALCOM:SOYBEANOIL", "SOY OIL"},
	{"CAPITALCOM:SOYBEANMEAL", "SOY MEAL"}, {"CAPITALCOM:WHEAT", "WHEAT"}, {"CAPITALCOM:SB", "SUGAR"},
	{"TVC:PLATINUM", "PLATINUM"}, {"CAPITALCOM:LIVECATTLE", "LIVE CATTLE"}, {"CAPITALCOM:COFFEE", "COFFEE"},
	{"CAPITALCOM:COCOA", "COCOA"}, {"CAPITALCOM:LEANHOGS", "LEAN HOGS"}, {"CAPITALCOM:COTTON", "COTTON"},
	{"PEPPERSTONE:ALUMINIUM", "ALUMINIUM"}, {"TVC:PALLADIUM", "PALLADIUM"}, {"CAPITALCOM:NICKEL", "NICKEL"},
	{"PEPPERSTONE:ZINC", "ZINC"}, {"CAPITALCOM:TSI", "IRON ORE"}, {"CAPITALCOM:FEEDERCATTLE", "FEEDER CATTLE"},
	{"CAPITALCOM:LEAD", "LEAD"}, {"CAPITALCOM:OATS", "OATS"}, {"CAPITALCOM:RR", "ROUGH RICE"},
}

var stablecoins = map[string]bool{}

func init() {
	for _, s := range strings.Fields("USDT USDC DAI FDUSD TUSD USDE USDS PYUSD USD1 USDG RLUSD USDD " +
		"BUSD FRAX GUSD USDP EURC XAUT PAXG USDTB USDF BFUSD") {
		stablecoins[s] = true
	}
}

var apiClient = &http.Client{Timeout: 30 * time.Second}

// getJSON GETs url, or POSTs body as JSON when body is non-nil, and decodes the reply into out.
func getJSON(url string, body any, out any) error {
	method, payload := "GET", []byte(nil)
	if body != nil {
		method = "POST"
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 TickerBar")
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type scanResult struct {
	Data []struct {
		S string `json:"s"`
		D []any  `json:"d"`
	} `json:"data"`
}

func isAlnum(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

func topCrypto() ([][]string, error) {
	var coins []struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"current_price"`
	}
	err := getJSON("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=volume_desc&per_page=150&page=1", nil, &coins)
	if err != nil {
		return nil, err
	}
	var syms []string
	for _, c := range coins {
		s := strings.ToUpper(c.Symbol)
		if stablecoins[s] || (c.Price > 0.97 && c.Price < 1.03) || !isAlnum(s) {
			continue
		}
		syms = append(syms, s)
	}

	// keep only coins TradingView actually lists, preferring Binance, then Coinbase, then Bybit
	pairs := func(s string) []string {
		return []string{"BINANCE:" + s + "USDT", "COINBASE:" + s + "USD", "BYBIT:" + s + "USDT"}
	}
	var cands []string
	for _, s := range syms {
		cands = append(cands, pairs(s)...)
	}
	var res scanResult
	if err := getJSON("https://scanner.tradingview.com/crypto/scan", map[string]any{
		"symbols": map[string]any{"tickers": cands}, "columns": []string{"close"},
	}, &res); err != nil {
		return nil, err
	}
	listed := map[string]bool{}
	for _, r := range res.Data {
		listed[r.S] = true
	}

	var out [][]string
	used := map[string]bool{}
	for _, s := range syms {
		if used[s] {
			continue
		}
		for _, p := range pairs(s) {
			if listed[p] {
				out = append(out, []string{p, s})
				used[s] = true
				break
			}
		}
		if len(out) == topCryptoCount {
			break
		}
	}
	return out, nil
}

func updateLists() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	crypto, err := topCrypto()
	if err != nil {
		return fmt.Errorf("crypto: %w", err)
	}

	markets := Row{Label: "MARKETS"}
	for _, r := range cfg.Rows {
		if r.Label == "MARKETS" {
			markets = r
		}
	}
	have := map[string]bool{}
	for _, s := range markets.Symbols {
		have[s[0]] = true
	}
	for _, r := range cfg.Rows { // FX pairs move into the markets row
		for _, s := range r.Symbols {
			if strings.HasPrefix(s[0], "FX:") && !have[s[0]] {
				markets.Symbols = append(markets.Symbols, s)
				have[s[0]] = true
			}
		}
	}

	cfg.Rows = []Row{
		markets,
		{Label: "TOP 6 CRYPTO", Symbols: crypto},
		{Label: "TOP 30 COMMODITIES", Symbols: commodities[:min(top, len(commodities))]},
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	names := func(rows [][]string) string {
		var n []string
		for _, r := range rows {
			n = append(n, r[1])
		}
		return strings.Join(n, " ")
	}
	fmt.Println("crypto:", names(crypto))
	fmt.Println("commodities:", min(top, len(commodities)))
	return nil
}
