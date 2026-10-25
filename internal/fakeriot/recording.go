package fakeriot

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadRecording turns a lowkey recording into a world that answers the way the real one did
func LoadRecording(dir string, now time.Time) (*World, error) {
	f, err := os.Open(filepath.Join(dir, "record.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := NewWorld(now)
	w.Players = map[string]*Player{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for n := 1; sc.Scan(); n++ {
		var e struct {
			Kind    string          `json:"kind"`
			Service string          `json:"service"`
			Path    string          `json:"path"`
			Query   string          `json:"query"`
			Status  int             `json:"status"`
			Body    json.RawMessage `json:"body"`
		}
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("record.jsonl line %d: %w", n, err)
		}
		switch e.Kind {
		case "local":
			if e.Path == "/entitlements/v1/token" {
				var v struct {
					Subject string `json:"subject"`
				}
				if json.Unmarshal(e.Body, &v) == nil && v.Subject != "" {
					w.Self = v.Subject
				}
				continue
			}
			w.Raw["local "+e.Path] = RawResponse{Status: e.Status, Body: e.Body}
		case "remote":
			key := e.Service + " " + e.Path
			if e.Query != "" {
				key += "?" + e.Query
			}
			w.Raw[key] = RawResponse{Status: e.Status, Body: e.Body}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	w.Players[w.Self] = &Player{PUUID: w.Self}
	w.PartyMembers = []string{w.Self}
	return w, nil
}
