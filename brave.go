package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// bravePrefs edits the ticker's own Brave profile: never show the toolbar/title bar in
// full screen, and turn off the product-analytics notice (choosing "Disable").
func bravePrefs(profile string) error {
	set := func(file string, edit func(map[string]any)) error {
		data, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			return nil // first launch: Brave hasn't created it yet
		} else if err != nil {
			return err
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber() // keep Brave's large integer timestamps exact
		var prefs map[string]any
		if err := dec.Decode(&prefs); err != nil {
			return err
		}
		edit(prefs)
		out, err := json.Marshal(prefs)
		if err != nil {
			return err
		}
		return os.WriteFile(file, out, 0o600)
	}
	child := func(m map[string]any, key string) map[string]any {
		c, ok := m[key].(map[string]any)
		if !ok {
			c = map[string]any{}
			m[key] = c
		}
		return c
	}
	p3a := func(m map[string]any) {
		p := child(child(m, "brave"), "p3a")
		p["enabled"] = false
		p["notice_acknowledged"] = true
	}

	if err := set(filepath.Join(profile, "Default", "Preferences"), func(m map[string]any) {
		child(m, "browser")["show_fullscreen_toolbar"] = false
		p3a(m)
	}); err != nil {
		return err
	}
	return set(filepath.Join(profile, "Local State"), p3a)
}
