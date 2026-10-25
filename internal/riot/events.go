package riot

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/coder/websocket"
)

const (
	EventMessaging = "OnJsonApiEvent_riot-messaging-service_v1_message"
	EventPresences = "OnJsonApiEvent_chat_v4_presences"
)

type Event struct {
	Name      string
	URI       string
	EventType string
	Data      json.RawMessage
	Raw       []byte
}

type Events struct {
	c *websocket.Conn
}

// Events opens the local WebSocket and subscribes to each event with WAMP opcode 5
func (l *Local) Events(ctx context.Context, names ...string) (*Events, error) {
	c, _, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(l.base, "https")+"/", &websocket.DialOptions{
		HTTPClient: l.ws,
		HTTPHeader: http.Header{"Authorization": {l.auth}},
	})
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(16 << 20)
	for _, name := range names {
		msg, _ := json.Marshal([]any{5, name})
		if err := c.Write(ctx, websocket.MessageText, msg); err != nil {
			c.CloseNow()
			return nil, err
		}
	}
	return &Events{c: c}, nil
}

// Next blocks until the next WAMP event (opcode 8) - anything else on the socket is skipped
func (e *Events) Next(ctx context.Context) (Event, error) {
	for {
		_, data, err := e.c.Read(ctx)
		if err != nil {
			return Event{}, err
		}
		var frame []json.RawMessage
		if json.Unmarshal(data, &frame) != nil || len(frame) < 3 {
			continue
		}
		var op int
		if json.Unmarshal(frame[0], &op) != nil || op != 8 {
			continue
		}
		var payload struct {
			Data      json.RawMessage `json:"data"`
			EventType string          `json:"eventType"`
			URI       string          `json:"uri"`
		}
		ev := Event{Raw: data}
		json.Unmarshal(frame[1], &ev.Name)
		json.Unmarshal(frame[2], &payload)
		ev.Data, ev.EventType, ev.URI = payload.Data, payload.EventType, payload.URI
		return ev, nil
	}
}

func (e *Events) Close() { e.c.CloseNow() }

var lifecycleRE = regexp.MustCompile(`/ares-(pregame|core-game)/(?:pregame|core-game)/v1/matches/([0-9a-fA-F-]{36})`)

type Lifecycle struct {
	Pregame bool // otherwise core-game
	MatchID string
}

func ParseLifecycle(ev Event) (Lifecycle, bool) {
	m := lifecycleRE.FindStringSubmatch(ev.URI)
	if m == nil {
		return Lifecycle{}, false
	}
	return Lifecycle{Pregame: m[1] == "pregame", MatchID: strings.ToLower(m[2])}, true
}

// DedupeKey is the same for an event delivered under both /message/ and /messages/
func (ev Event) DedupeKey() string {
	return strings.Replace(ev.URI, "/messages/", "/message/", 1) + " " + string(ev.Data)
}

// OwnPresence reads a presences event - mentioned is false when it isn't about your Valorant presence
func OwnPresence(ev Event, self string) (priv Private, present, mentioned bool) {
	var data struct {
		Presences []Presence `json:"presences"`
	}
	if json.Unmarshal(ev.Data, &data) != nil {
		return Private{}, false, false
	}
	for _, p := range data.Presences {
		if !strings.EqualFold(p.PUUID, self) || p.Product != "valorant" {
			continue
		}
		if strings.EqualFold(ev.EventType, "Delete") || p.Private == nil {
			return Private{}, false, true
		}
		priv, err := ParsePrivate(*p.Private)
		if err != nil {
			continue
		}
		return priv, true, true
	}
	return Private{}, false, false
}
