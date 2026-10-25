package fakeriot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	EventMessaging = "OnJsonApiEvent_riot-messaging-service_v1_message"
	EventPresences = "OnJsonApiEvent_chat_v4_presences"
)

type wsConn struct {
	c    *websocket.Conn
	mu   sync.Mutex
	subs map[string]bool
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20)
	conn := &wsConn{c: c, subs: map[string]bool{}}
	s.mu.Lock()
	s.ws[conn] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.ws, conn)
		s.mu.Unlock()
		c.CloseNow()
	}()
	for {
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var msg []json.RawMessage
		if json.Unmarshal(data, &msg) != nil || len(msg) < 2 {
			continue
		}
		var op int
		var name string
		json.Unmarshal(msg[0], &op)
		json.Unmarshal(msg[1], &name)
		conn.mu.Lock()
		switch op {
		case 5:
			conn.subs[name] = true
		case 6:
			delete(conn.subs, name)
		}
		conn.mu.Unlock()
	}
}

func (s *Server) closeWS() {
	s.mu.Lock()
	conns := make([]*wsConn, 0, len(s.ws))
	for c := range s.ws {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		c.c.CloseNow()
	}
}

func (s *Server) Subscribers(event string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for c := range s.ws {
		c.mu.Lock()
		if c.subs[event] {
			n++
		}
		c.mu.Unlock()
	}
	return n
}

// WaitSubscribed blocks until some connection subscribed to every event
func (s *Server) WaitSubscribed(ctx context.Context, events ...string) error {
	for {
		ok := true
		for _, e := range events {
			if s.Subscribers(e) == 0 {
				ok = false
			}
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// Emit sends a WAMP event (opcode 8) to every subscriber
func (s *Server) Emit(event, uri, eventType string, data any) int {
	frame := mustJSON([]any{8, event, obj{"data": data, "eventType": eventType, "uri": uri}})
	s.mu.Lock()
	var targets []*wsConn
	for c := range s.ws {
		c.mu.Lock()
		if c.subs[event] {
			targets = append(targets, c)
		}
		c.mu.Unlock()
	}
	s.mu.Unlock()
	for _, c := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		c.c.Write(ctx, websocket.MessageText, frame)
		cancel()
	}
	return len(targets)
}

func (s *Server) emitLifecycle(service, resource string, duplicate bool) {
	s.mu.Lock()
	s.messages++
	id := s.messages
	s.mu.Unlock()
	// a duplicate is the same message under a second path - genuine updates get their own id
	data := obj{
		"ackRequired": false, "id": fmt.Sprintf("rms-%d", id), "payload": "", "resource": resource, "service": service,
		"timestamp": s.now().UnixMilli(), "version": "1.0",
	}
	s.Emit(EventMessaging, "/riot-messaging-service/v1/message/"+resource, "Create", data)
	if duplicate {
		s.Emit(EventMessaging, "/riot-messaging-service/v1/messages/"+resource, "Create", data)
	}
}

// EmitPregame is the agent select lifecycle event - duplicate sends it under both message paths
func (s *Server) EmitPregame(matchID string, duplicate bool) {
	s.emitLifecycle("ares-pregame", "ares-pregame/pregame/v1/matches/"+matchID, duplicate)
}

func (s *Server) EmitCoreGame(matchID string, duplicate bool) {
	s.emitLifecycle("ares-core-game", "ares-core-game/core-game/v1/matches/"+matchID, duplicate)
}

// EmitPresence pushes your current presence the way the chat service does
func (s *Server) EmitPresence() {
	s.mu.Lock()
	var body struct {
		Presences []json.RawMessage `json:"presences"`
	}
	json.Unmarshal(s.world.presences(), &body)
	valorant := s.world.Valorant
	self := s.world.Self
	s.mu.Unlock()
	if !valorant {
		s.Emit(EventPresences, "/chat/v4/presences", "Delete", obj{"presences": []obj{{"puuid": self, "product": "valorant"}}})
		return
	}
	s.Emit(EventPresences, "/chat/v4/presences", "Update", obj{"presences": body.Presences[len(body.Presences)-1:]})
}
