package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	listenAddr = "127.0.0.1:8787"
	refreshInt = 5 * time.Minute // between feed fetches
)

var news struct {
	sync.RWMutex
	items []Item
}

// refresher fetches feeds in the background so requests never wait on the network.
func refresher() {
	for {
		if cfg, err := loadConfig(); err != nil {
			log.Printf("refresh failed: %v", err)
		} else if items := collectNews(cfg); len(items) > 0 {
			news.Lock()
			news.items = items
			news.Unlock()
		}
		time.Sleep(refreshInt)
	}
}

func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store") // always serve fresh page/config
		h.ServeHTTP(w, r)
	})
}

func serve() error {
	go refresher()
	mux := http.NewServeMux()
	mux.HandleFunc("/news", func(w http.ResponseWriter, r *http.Request) {
		news.RLock()
		items := news.items
		news.RUnlock()
		if items == nil {
			items = []Item{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(items)
	})
	mux.Handle("/", http.FileServer(http.Dir(baseDir())))
	log.Printf("Ticker bar on http://%s", listenAddr)
	return http.ListenAndServe(listenAddr, noStore(mux))
}
