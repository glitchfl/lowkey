// catalog builds a phase-by-phase catalogue of what the Riot Client exposes locally - field names,
// types and enum-like values only. It finds which no-argument GETs exist once, listens passively to
// every websocket event, and sweeps the real GETs once each time the game's phase changes.
// It talks to 127.0.0.1 only and never stores names, IDs or free text.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glitchfl/lowkey/internal/platform"
	"github.com/glitchfl/lowkey/internal/riot"
)

var (
	out      string
	base     string
	auth     string
	client   = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} // 127.0.0.1 only
	phaseMu  sync.Mutex
	phase    = "unknown"
	writeMu  sync.Mutex
	excluded = regexp.MustCompile(`(?i)token|auth|login|password|credential|cookie|jwt|secret|messages|mailbox|pay|commerce|xbox|playstation|nintendo|integrationtest|keypair|sanitizer|debug|swagger|help|cert|key`)
	uuidRE   = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	idishRE  = idMatcher{}
	enumKey  = regexp.MustCompile(`(?i)(state|phase|status|^type$|mode|queue|flow|^product|platform|region|shard|patchline|loop|^map|side|team|eventtype|permission|reason|code$|kind|source|tier|result|role|category|provider|availability|activity|^locale)`)
	enumVal  = regexp.MustCompile(`^[A-Za-z/][A-Za-z0-9_./ :-]{0,47}$`)
)

// idMatcher says whether a key or path part is an ID - long, and letters mixed with digits, so field
// names like partyOwnerMatchScoreAllyTeam and paths like riot-messaging-service stay readable
type idMatcher struct{}

var (
	longToken = regexp.MustCompile(`^[A-Za-z0-9_-]{16,}$`)
	hasDigit  = regexp.MustCompile(`[0-9]`)
	hasLetter = regexp.MustCompile(`[A-Za-z]`)
)

func (idMatcher) MatchString(s string) bool {
	return longToken.MatchString(s) && hasDigit.MatchString(s) && hasLetter.MatchString(s)
}

func main() {
	out = os.Args[1]
	helpPath := os.Args[2]
	os.MkdirAll(out, 0o755)
	platform.Prepare() // below normal priority and EcoQoS, like lowkey

	data, err := os.ReadFile(filepath.Join(os.Getenv("LOCALAPPDATA"), "Riot Games", "Riot Client", "Config", "lockfile"))
	must(err)
	lf, err := riot.ParseLockfile(data)
	must(err)
	base = fmt.Sprintf("https://127.0.0.1:%d", lf.Port)
	auth = "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:"+lf.Password))

	routes := discover(helpPath)
	logf("%d routes exist", len(routes))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go listen(ctx, lf)
	watch(ctx, routes)
}

func logf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}

func must(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func get(path string) (int, []byte, error) {
	req, _ := http.NewRequest("GET", base+path, nil)
	req.Header.Set("Authorization", auth)
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, body, nil
}

// ---- discovery

var wordRE = regexp.MustCompile(`[A-Z][a-z0-9]*|[0-9]+`)

func kebab(words []string) string { return strings.ToLower(strings.Join(words, "-")) }

// candidates turns GetPlayerAccountAliasesV1Active into the paths it could be - the name drops where the slashes were
func candidates(name string) []string {
	m := regexp.MustCompile(`^Get([A-Za-z]+?)(V[0-9]+)([A-Za-z0-9]*)$`).FindStringSubmatch(name)
	if m == nil {
		return nil
	}
	ns, ver, rest := wordRE.FindAllString(m[1], -1), strings.ToLower(m[2]), wordRE.FindAllString(m[3], -1)
	nsVariants := []string{kebab(ns)}
	for i := 1; i < len(ns) && i <= 3; i++ {
		nsVariants = append(nsVariants, kebab(ns[:i])+"/"+kebab(ns[i:]))
	}
	var restVariants []string
	switch n := len(rest); {
	case n == 0:
		restVariants = []string{""}
	case n <= 4:
		for mask := 0; mask < 1<<(n-1); mask++ {
			var segs []string
			cur := []string{rest[0]}
			for i := 1; i < n; i++ {
				if mask&(1<<(i-1)) != 0 {
					segs = append(segs, kebab(cur))
					cur = nil
				}
				cur = append(cur, rest[i])
			}
			segs = append(segs, kebab(cur))
			restVariants = append(restVariants, strings.Join(segs, "/"))
		}
	default:
		restVariants = []string{kebab(rest), strings.ToLower(strings.Join(rest, "/")),
			strings.ToLower(rest[0]) + "/" + kebab(rest[1:]), kebab(rest[:len(rest)-1]) + "/" + strings.ToLower(rest[len(rest)-1])}
	}
	var outPaths []string
	for _, r := range restVariants {
		for _, n := range nsVariants {
			p := "/" + n + "/" + ver
			if r != "" {
				p += "/" + r
			}
			outPaths = append(outPaths, p)
		}
	}
	return outPaths
}

func discover(helpPath string) []string {
	cache := filepath.Join(out, "routes.json")
	if b, err := os.ReadFile(cache); err == nil {
		var routes []string
		if json.Unmarshal(b, &routes) == nil && len(routes) > 0 {
			return routes
		}
	}
	var help struct {
		Functions []struct {
			Name      string `json:"name"`
			Arguments []struct {
				Optional bool `json:"optional"`
			} `json:"arguments"`
		} `json:"functions"`
	}
	b, err := os.ReadFile(helpPath)
	must(err)
	must(json.Unmarshal(b, &help))
	var routes []string
	tried, skipped := 0, 0
	for _, f := range help.Functions {
		if !strings.HasPrefix(f.Name, "Get") {
			continue
		}
		required := false
		for _, a := range f.Arguments {
			required = required || !a.Optional
		}
		if required {
			continue
		}
		if excluded.MatchString(f.Name) {
			skipped++
			continue
		}
		for _, p := range candidates(f.Name) {
			time.Sleep(40 * time.Millisecond)
			tried++
			status, body, err := get(p)
			if err != nil || (status == 404 && bytes.Contains(body, []byte("Invalid URI format"))) {
				continue
			}
			routes = append(routes, p)
			break
		}
	}
	sort.Strings(routes)
	b, _ = json.MarshalIndent(routes, "", " ")
	os.WriteFile(cache, b, 0o644)
	logf("discovery: %d requests, %d functions skipped as sensitive", tried, skipped)
	return routes
}

// ---- shapes

type shaper struct{ enums map[string]map[string]bool }

func maskKey(k string) string {
	if uuidRE.MatchString(k) || idishRE.MatchString(k) {
		return "<id>"
	}
	return k
}

func (s *shaper) shape(v any, key string, depth int) any {
	switch x := v.(type) {
	case map[string]any:
		if depth > 6 {
			return "{...}"
		}
		o := map[string]any{}
		for k, val := range x {
			mk := maskKey(k)
			if _, seen := o[mk]; seen {
				continue
			}
			if k == "private" || k == "payload" {
				if str, ok := val.(string); ok {
					if inner, ok := decodeInner(str); ok {
						o[mk+"(decoded)"] = s.shape(inner, k, depth+1)
						continue
					}
				}
			}
			o[mk] = s.shape(val, k, depth+1)
		}
		return o
	case []any:
		if len(x) == 0 {
			return []any{}
		}
		// every element's fields together - a list of presences mixes products and each has its own
		var merged any
		for i, el := range x {
			if i == 50 {
				break
			}
			merged = merge(merged, s.shape(el, key, depth+1))
		}
		return []any{merged, fmt.Sprintf("len %d", len(x))}
	case string:
		if enumKey.MatchString(key) && enumVal.MatchString(x) && !uuidRE.MatchString(x) && !idishRE.MatchString(x) {
			if s.enums[key] == nil {
				s.enums[key] = map[string]bool{}
			}
			if len(s.enums[key]) < 12 {
				s.enums[key][x] = true
			}
			return "string:" + x
		}
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

func merge(a, b any) any {
	ma, okA := a.(map[string]any)
	mb, okB := b.(map[string]any)
	if !okA || !okB {
		if a == nil || a == "null" {
			return b
		}
		return a
	}
	for k, v := range mb {
		ma[k] = merge(ma[k], v)
	}
	return ma
}

func decodeInner(s string) (any, bool) {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		if _, ok := v.(map[string]any); ok {
			return v, true
		}
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && json.Unmarshal(b, &v) == nil {
		return v, true
	}
	return nil, false
}

func maskPath(p string) string {
	p = uuidRE.ReplaceAllString(p, "<id>")
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if idishRE.MatchString(part) || strings.Contains(part, "@") || strings.HasPrefix(part, "sip:") {
			parts[i] = "<id>"
		}
	}
	return strings.Join(parts, "/")
}

func record(file string, entry map[string]any) {
	writeMu.Lock()
	defer writeMu.Unlock()
	f, err := os.OpenFile(filepath.Join(out, file), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(entry)
	f.Write(append(b, '\n'))
}

func currentPhase() string {
	phaseMu.Lock()
	defer phaseMu.Unlock()
	return phase
}

// ---- passive websocket

func listen(ctx context.Context, lf riot.Lockfile) {
	l := riot.NewLocal(lf, riot.RiotRoots())
	evs, err := l.Events(ctx, "OnJsonApiEvent")
	if err != nil {
		logf("websocket: %v", err)
		return
	}
	defer evs.Close()
	seen := map[string]bool{}
	for {
		ev, err := evs.Next(ctx)
		if err != nil {
			logf("websocket stopped: %v", err)
			return
		}
		var v any
		json.Unmarshal(ev.Data, &v)
		s := &shaper{enums: map[string]map[string]bool{}}
		sh := s.shape(v, "", 0)
		resource := ""
		if m, ok := v.(map[string]any); ok {
			if r, ok := m["resource"].(string); ok {
				resource = maskPath(r)
			}
		}
		b, _ := json.Marshal(sh)
		key := currentPhase() + "|" + maskPath(ev.URI) + "|" + ev.EventType + "|" + resource + "|" + string(b)
		if seen[key] {
			continue
		}
		seen[key] = true
		record("events.jsonl", map[string]any{"time": time.Now().Format(time.RFC3339), "phase": currentPhase(),
			"uri": maskPath(ev.URI), "eventType": ev.EventType, "resource": resource, "shape": sh, "enums": s.enums})
	}
}

// ---- phase watch and sweeps

func selfPUUID() string {
	_, body, err := get("/chat/v1/session")
	if err != nil {
		return ""
	}
	var s struct {
		PUUID string `json:"puuid"`
	}
	json.Unmarshal(body, &s)
	return s.PUUID
}

func loopState(self string) string {
	_, body, err := get("/chat/v4/presences")
	if err != nil {
		return ""
	}
	var p struct {
		Presences []struct {
			PUUID   string `json:"puuid"`
			Product string `json:"product"`
			Private string `json:"private"`
		} `json:"presences"`
	}
	json.Unmarshal(body, &p)
	for _, pr := range p.Presences {
		if pr.PUUID != self || pr.Product != "valorant" {
			continue
		}
		v, ok := decodeInner(pr.Private)
		if !ok {
			return ""
		}
		m := v.(map[string]any)
		for _, k := range []string{"sessionLoopState"} {
			if s, ok := m[k].(string); ok {
				return s
			}
		}
		if mp, ok := m["matchPresenceData"].(map[string]any); ok {
			if s, ok := mp["sessionLoopState"].(string); ok {
				return s
			}
		}
	}
	return "NOT_IN_VALORANT"
}

func watch(ctx context.Context, routes []string) {
	self := selfPUUID()
	delays := map[string]time.Duration{"PREGAME": 9 * time.Second, "INGAME": 40 * time.Second, "MENUS": 15 * time.Second}
	last := ""
	var pending *time.Timer
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		st := loopState(self)
		if st != "" && st != last {
			phaseMu.Lock()
			phase = st
			phaseMu.Unlock()
			logf("phase %s", st)
			last = st
			if pending != nil {
				pending.Stop()
			}
			d, ok := delays[st]
			if !ok {
				d = 5 * time.Second
			}
			want := st
			pending = time.AfterFunc(d, func() {
				if currentPhase() == want {
					sweep(want, routes)
				}
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func sweep(ph string, routes []string) {
	start := time.Now()
	for _, p := range routes {
		if currentPhase() != ph {
			logf("sweep of %s cut short - phase changed", ph)
			return
		}
		time.Sleep(250 * time.Millisecond)
		status, body, err := get(p)
		if err != nil {
			continue
		}
		var v any
		entry := map[string]any{"time": time.Now().Format(time.RFC3339), "phase": ph, "path": p, "status": status, "bytes": len(body)}
		if json.Unmarshal(body, &v) == nil {
			s := &shaper{enums: map[string]map[string]bool{}}
			entry["shape"], entry["enums"] = s.shape(v, "", 0), s.enums
		}
		record("local.jsonl", entry)
	}
	logf("swept %d routes in %s for %s", len(routes), time.Since(start).Round(time.Second), ph)
}
