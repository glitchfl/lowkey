package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
	"github.com/glitchfl/lowkey/internal/riot"
)

func TestVoiceCheckInAgentSelect(t *testing.T) {
	setup := func(w *fakeriot.World) {
		w.Voice = map[string]string{
			w.Self: "MIC",
			ally1:  "connected",
			ally2:  "OFF",
			ally3:  "-",
			// ally4 omitted -> unresolvable -> "-"
		}
	}

	t.Run("enabled shows VOICE column with status in agent select", func(t *testing.T) {
		r := newRig(t, agentSelectLobby, setup)
		r.writeSettings("[match]\nvoice_check = true\n")
		r.live()
		r.screen("waiting for the next match")
		r.subscribed()
		r.enterAgentSelect()
		frame := r.settled("AGENT SELECT", "VOICE", "[MIC]", "[OFF]")

		if !strings.Contains(frame, "VOICE") {
			t.Fatalf("frame should contain VOICE column header:\n%s", frame)
		}
		if !strings.Contains(frame, "[MIC]") {
			t.Errorf("frame should contain [MIC]:\n%s", frame)
		}
		if !strings.Contains(frame, "[OFF]") {
			t.Errorf("frame should contain [OFF]:\n%s", frame)
		}

		// Ensure no remote requests were made for voice (loopback IPC only)
		for _, req := range r.fr.RemoteRequests() {
			if strings.Contains(req.Path, "voice") {
				t.Errorf("unexpected remote request for voice: %s", req.Path)
			}
		}
	})

	t.Run("disabled hides VOICE column in agent select", func(t *testing.T) {
		r := newRig(t, agentSelectLobby, setup)
		r.writeSettings("[match]\nvoice_check = false\n")
		r.live()
		r.screen("waiting for the next match")
		r.subscribed()
		r.enterAgentSelect()
		frame := r.settled("AGENT SELECT", "Dia1 50")

		if strings.Contains(frame, "VOICE") {
			t.Errorf("disabled voice check should not show VOICE header:\n%s", frame)
		}
		if strings.Contains(frame, "[MIC]") || strings.Contains(frame, "[OFF]") {
			t.Errorf("disabled voice check should not show [MIC] or [OFF]:\n%s", frame)
		}
	})

	t.Run("asks once per roster and follows what changes", func(t *testing.T) {
		r := newRig(t, agentSelectLobby, setup)
		r.writeSettings("[match]\nvoice_check = true\n")
		r.live()
		r.screen("waiting for the next match")
		r.subscribed()
		r.enterAgentSelect()
		frame := r.settled("AGENT SELECT", "VOICE", "[MIC]")
		if got := cellIn(t, frame, "VOICE", 3); got != "[OFF]" {
			t.Errorf("ally two: got %q, want [OFF]\n%s", got, frame)
		}
		if n := voiceRequests(r); n != 1 {
			t.Errorf("asked about voice %d times on entry, want once", n)
		}
		r.fr.Update(func(w *fakeriot.World) { w.Voice[ally2] = "MIC" })
		r.fr.Hover(ally1, fakeriot.Jett, true)
		settle()
		r.clock.Advance(3 * time.Second)
		eventually(t, "ally two's mic to show", func() bool { return cellIn(t, r.con.last(), "VOICE", 3) == "[MIC]" })
		if n := voiceRequests(r); n != 2 {
			t.Errorf("asked about voice %d times after one more roster, want 2", n)
		}
	})

	t.Run("column is hidden in match load even when voice_check is true", func(t *testing.T) {
		r := fromAgentSelect(t, setup)
		r.writeSettings("[match]\nvoice_check = true\n")
		r.enterMatchLoad()
		frame := r.settled("MATCH LOADING", "US - THEM", "Plat2 10")

		if strings.Contains(frame, "VOICE") {
			t.Errorf("VOICE column should not appear in match load:\n%s", frame)
		}
	})
}

func voiceRequests(r *rig) int {
	n := 0
	for _, req := range r.fr.Requests() {
		if req.Service == "local" && req.Path == "/telemetry/v1/voice" {
			n++
		}
	}
	return n
}

func TestDecodeVoiceFormats(t *testing.T) {
	tests := []struct {
		name string
		json string
		want map[string]string
	}{
		{
			name: "string status with various cases",
			json: `{"participants": [
				{"puuid": "p1", "status": "MIC"},
				{"puuid": "p2", "status": "off"},
				{"puuid": "p3", "status": "connected"},
				{"puuid": "p4", "status": "muted"},
				{"puuid": "p5", "status": "-"}
			]}`,
			want: map[string]string{
				"p1": "[MIC]",
				"p2": "[OFF]",
				"p3": "[MIC]",
				"p4": "[OFF]",
				"p5": "-",
			},
		},
		{
			name: "boolean connected and muted fields",
			json: `{"members": [
				{"puuid": "p1", "connected": true, "muted": false},
				{"puuid": "p2", "connected": true, "muted": true},
				{"puuid": "p3", "connected": false}
			]}`,
			want: map[string]string{
				"p1": "[MIC]",
				"p2": "[OFF]",
				"p3": "[OFF]",
			},
		},
		{
			name: "direct array format",
			json: `[
				{"puuid": "p1", "status": "[MIC]"},
				{"puuid": "p2", "status": "[OFF]"}
			]`,
			want: map[string]string{
				"p1": "[MIC]",
				"p2": "[OFF]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := riot.DecodeVoice([]byte(tt.json))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for k, wantVal := range tt.want {
				if got[k] != wantVal {
					t.Errorf("puuid %s: got %q, want %q", k, got[k], wantVal)
				}
			}
		})
	}
}
