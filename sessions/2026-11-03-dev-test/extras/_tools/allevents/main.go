// allevents subscribes to every websocket event the Riot Client's help lists except OnJsonApiEvent
// (the catalog already has that one) and records each event name with its data's field shape once.
// Passive, lockfile access only, nothing stored but field names and types.
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
	"time"

	"github.com/glitchfl/lowkey/internal/platform"
	"github.com/glitchfl/lowkey/internal/riot"
)

var idRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|^[A-Za-z0-9_-]*[0-9][A-Za-z0-9_-]*$`)

func shape(v any, d int) any {
	switch x := v.(type) {
	case map[string]any:
		if d > 5 {
			return "{...}"
		}
		o := map[string]any{}
		for k, val := range x {
			if len(k) >= 16 && idRE.MatchString(k) {
				k = "<id>"
			}
			o[k] = shape(val, d+1)
		}
		return o
	case []any:
		if len(x) == 0 {
			return []any{}
		}
		return []any{shape(x[0], d+1), fmt.Sprintf("len %d", len(x))}
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}

func main() {
	out, helpPath := os.Args[1], os.Args[2]
	platform.Prepare()
	var help struct {
		Events []struct {
			Name string `json:"name"`
		} `json:"events"`
	}
	b, err := os.ReadFile(helpPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	json.Unmarshal(b, &help)
	var names []string
	for _, e := range help.Events {
		if e.Name != "OnJsonApiEvent" {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	data, err := os.ReadFile(filepath.Join(os.Getenv("LOCALAPPDATA"), "Riot Games", "Riot Client", "Config", "lockfile"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	lf, err := riot.ParseLockfile(data)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	evs, err := riot.NewLocal(lf, riot.RiotRoots()).Events(ctx, names...)
	if err != nil {
		fmt.Println("websocket:", err)
		os.Exit(1)
	}
	defer evs.Close()
	fmt.Printf("%s subscribed to %d event types\n", time.Now().Format("15:04:05"), len(names))
	seen := map[string]bool{}
	f, _ := os.OpenFile(filepath.Join(out, "other-events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer f.Close()
	for {
		ev, err := evs.Next(ctx)
		if err != nil {
			fmt.Println("stopped:", err)
			return
		}
		var v any
		json.Unmarshal(ev.Data, &v)
		sh, _ := json.Marshal(shape(v, 0))
		key := ev.Name + "|" + string(sh)
		if seen[key] {
			continue
		}
		seen[key] = true
		line, _ := json.Marshal(map[string]any{"time": time.Now().Format(time.RFC3339), "event": ev.Name, "eventType": ev.EventType, "shape": json.RawMessage(sh)})
		f.Write(append(line, '\n'))
		fmt.Printf("%s new %s\n", time.Now().Format("15:04:05"), ev.Name)
	}
}
