package record

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// your own identity becomes these in every recording
const (
	SelfPUUID = "5e1f5e1f-5e1f-4e1f-8e1f-5e1f5e1f5e1f"
	SelfName  = "self"
	SelfTag   = "self"
)

var (
	uuidRE      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	jwtRE       = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)
	authTokenRE = regexp.MustCompile(`(-riotclient-auth-token=)[^\s"]+`)
)

var tokenKeys = map[string]bool{
	"accesstoken": true, "access_token": true, "token": true, "idtoken": true, "id_token": true,
	"entitlements_token": true, "refresh_token": true, "authorization": true,
}

var subjectKeys = []string{"subject", "Subject", "puuid", "PUUID", "Puuid"}

// any UUID that isn't public static data counts as identifying - match and party IDs included
type Scrubber struct {
	mu      sync.Mutex
	public  func(string) bool
	self    string
	kept    map[string]bool
	fakes   map[string]string
	names   map[string]string
	tags    map[string]string
	secrets []string
}

func NewScrubber(public func(string) bool) *Scrubber {
	if public == nil {
		public = func(string) bool { return false }
	}
	return &Scrubber{
		public: public,
		kept:   map[string]bool{},
		fakes:  map[string]string{},
		names:  map[string]string{},
		tags:   map[string]string{},
	}
}

func (s *Scrubber) SetSelf(puuid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setSelf(puuid)
}

func (s *Scrubber) setSelf(puuid string) {
	s.self = strings.ToLower(puuid)
	if _, ok := s.fakes[s.self]; !ok {
		s.fakes[s.self] = SelfPUUID
	}
}

// Secret registers a value that must never appear - the lockfile password for one
func (s *Scrubber) Secret(v string) {
	if v == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets = append(s.secrets, v)
}

func (s *Scrubber) Text(v string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text(v)
}

func (s *Scrubber) text(v string) string {
	for _, secret := range s.secrets {
		v = strings.ReplaceAll(v, secret, "<scrubbed>")
	}
	v = jwtRE.ReplaceAllString(v, "<token>")
	v = authTokenRE.ReplaceAllString(v, "${1}<scrubbed>")
	return uuidRE.ReplaceAllStringFunc(v, s.uuid)
}

func (s *Scrubber) uuid(real string) string {
	key := strings.ToLower(real)
	if s.kept[key] || s.public(key) {
		return real
	}
	if fake, ok := s.fakes[key]; ok {
		return fake
	}
	fake := fmt.Sprintf("fa4e0000-0000-4000-8000-%012d", len(s.fakes)+1)
	s.fakes[key] = fake
	return fake
}

// JSON scrubs a response body - anything that isn't JSON comes back as a scrubbed JSON string
func (s *Scrubber) JSON(body []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var v any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		out, _ := json.Marshal(s.text(string(body)))
		return out
	}
	s.learn(v)
	out, err := json.Marshal(s.walk(v))
	if err != nil {
		out, _ = json.Marshal(s.text(string(body)))
	}
	return out
}

// learn picks up your PUUID from the entitlements response and keeps act IDs from the content service
func (s *Scrubber) learn(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if sub, ok := m["subject"].(string); ok && m["accessToken"] != nil && s.self == "" {
		s.setSelf(sub)
	}
	if seasons, ok := m["Seasons"].([]any); ok {
		for _, season := range seasons {
			if sm, ok := season.(map[string]any); ok {
				if id, ok := sm["ID"].(string); ok {
					s.kept[strings.ToLower(id)] = true
				}
			}
		}
	}
}

func (s *Scrubber) isSelf(m map[string]any) bool {
	if s.self == "" {
		return false
	}
	for _, k := range subjectKeys {
		if v, ok := m[k].(string); ok && strings.EqualFold(v, s.self) {
			return true
		}
	}
	return false
}

func (s *Scrubber) walk(v any) any {
	switch t := v.(type) {
	case map[string]any:
		self := s.isSelf(t)
		_, presence := t["puuid"]
		out := make(map[string]any, len(t))
		for k, val := range t {
			key := s.text(k)
			lower := strings.ToLower(k)
			switch {
			case tokenKeys[lower]:
				out[key] = "<scrubbed>"
			case lower == "gamename" || lower == "game_name" || (lower == "name" && presence):
				out[key] = s.fake(s.names, val, self, SelfName, "player")
			case lower == "tagline" || lower == "game_tag" || lower == "tag_line":
				out[key] = s.fake(s.tags, val, self, SelfTag, "t")
			case lower == "private":
				out[key] = s.private(val)
			default:
				out[key] = s.walk(val)
			}
		}
		return out
	case []any:
		for i := range t {
			t[i] = s.walk(t[i])
		}
		return t
	case string:
		return s.text(t)
	}
	return v
}

func (s *Scrubber) fake(table map[string]string, v any, self bool, selfFake, prefix string) any {
	real, ok := v.(string)
	if !ok || real == "" {
		return v
	}
	if fake, ok := table[real]; ok {
		return fake
	}
	fake := fmt.Sprintf("%s%d", prefix, len(table)+1)
	if self {
		fake = selfFake
	}
	table[real] = fake
	return fake
}

// private presence is base64 JSON
func (s *Scrubber) private(v any) any {
	enc, ok := v.(string)
	if !ok {
		return s.walk(v)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return s.text(enc)
	}
	var inner any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if dec.Decode(&inner) != nil {
		return "<scrubbed>"
	}
	out, err := json.Marshal(s.walk(inner))
	if err != nil {
		return "<scrubbed>"
	}
	return base64.StdEncoding.EncodeToString(out)
}
