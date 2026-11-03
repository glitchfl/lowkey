// voiceprobe listens passively to every event on the Riot Client's local websocket.
// It sends no requests. Voice-chat events are saved raw (they hold real ids, so the
// file stays in the scratchpad) and printed with ids masked and values reduced to shapes.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glitchfl/lowkey/internal/riot"
)

var (
	uuidRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	sipRE  = regexp.MustCompile(`sip:[^/"]+`)
	numRE  = regexp.MustCompile(`/[0-9]{3,}`)
)

func mask(s string) string {
	s = uuidRE.ReplaceAllString(s, "<id>")
	s = sipRE.ReplaceAllString(s, "sip:<x>")
	return numRE.ReplaceAllString(s, "/<n>")
}

func shape(v any, depth int) string {
	switch x := v.(type) {
	case map[string]any:
		if depth > 4 {
			return "{...}"
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+shape(x[k], depth+1))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		if len(x) == 0 {
			return "list[0]"
		}
		return fmt.Sprintf("list[%d] of %s", len(x), shape(x[0], depth+1))
	case string:
		if len(x) < 24 && !strings.ContainsAny(x, "0123456789@#") {
			return fmt.Sprintf("str %q", x)
		}
		return "str"
	case bool:
		return fmt.Sprintf("bool %v", x)
	case float64:
		return fmt.Sprintf("num %v", x)
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}

func main() {
	out := os.Args[1]
	lockPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "Riot Games", "Riot Client", "Config", "lockfile")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		fmt.Println("lockfile:", err)
		os.Exit(1)
	}
	lf, err := riot.ParseLockfile(data)
	if err != nil {
		fmt.Println("lockfile:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	l := riot.NewLocal(lf, riot.RiotRoots())
	evs, err := l.Events(ctx, "OnJsonApiEvent")
	if err != nil {
		fmt.Println("websocket:", err)
		os.Exit(1)
	}
	defer evs.Close()

	raw, _ := os.OpenFile(filepath.Join(out, "voice-events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer raw.Close()
	fmt.Println(time.Now().Format("15:04:05"), "listening to every local event")

	seen := map[string]int{}
	for {
		ev, err := evs.Next(ctx)
		if err != nil {
			fmt.Println("stopped:", err)
			return
		}
		now := time.Now().Format("15:04:05.000")
		uri := mask(ev.URI)
		if !strings.Contains(strings.ToLower(ev.URI), "voice") {
			// other traffic: one line the first time each masked uri shows up, so we see what else flows
			if seen[uri]++; seen[uri] == 1 {
				fmt.Printf("%s other %-7s %s\n", now, ev.EventType, uri)
			}
			continue
		}
		fmt.Fprintf(raw, "{\"time\":%q,\"event\":%s}\n", time.Now().Format(time.RFC3339Nano), ev.Raw)
		var v any
		json.Unmarshal(ev.Data, &v)
		fmt.Printf("%s VOICE %-7s %s\n    %s\n", now, ev.EventType, uri, mask(shape(v, 0)))
	}
}
