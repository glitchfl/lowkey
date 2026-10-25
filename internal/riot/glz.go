package riot

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Roster struct {
	MatchID  string
	Queue    string
	HasQueue bool
	Map      string
	Mode     string
	Players  []RosterPlayer
}

type RosterPlayer struct {
	PUUID     string
	Team      string
	Agent     string
	Incognito bool
}

type rosterPlayer struct {
	Subject        string `json:"Subject"`
	TeamID         string `json:"TeamID"`
	CharacterID    string `json:"CharacterID"`
	PlayerIdentity struct {
		Incognito bool `json:"Incognito"`
	} `json:"PlayerIdentity"`
}

func (p rosterPlayer) decode(team string) RosterPlayer {
	if team == "" {
		team = p.TeamID
	}
	return RosterPlayer{
		PUUID: strings.ToLower(p.Subject), Team: team, Agent: strings.ToLower(p.CharacterID),
		Incognito: p.PlayerIdentity.Incognito,
	}
}

// DecodeMatchID reads the pregame or core-game players endpoint
func DecodeMatchID(body []byte) (string, error) {
	var v struct {
		MatchID string `json:"MatchID"`
	}
	if err := json.Unmarshal(body, &v); err != nil || !IsUUID(v.MatchID) {
		return "", fmt.Errorf("no valid match ID in %q", truncate(body))
	}
	return strings.ToLower(v.MatchID), nil
}

// DecodePregame reads your own team only - Riot never fills EnemyTeam during agent select
func DecodePregame(body []byte) (Roster, error) {
	var v struct {
		ID       string  `json:"ID"`
		MapID    string  `json:"MapID"`
		Mode     string  `json:"Mode"`
		QueueID  *string `json:"QueueID"`
		AllyTeam *struct {
			TeamID  string         `json:"TeamID"`
			Players []rosterPlayer `json:"Players"`
		} `json:"AllyTeam"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Roster{}, fmt.Errorf("pregame match isn't JSON: %w", err)
	}
	r := Roster{MatchID: strings.ToLower(v.ID), Map: v.MapID, Mode: v.Mode}
	if v.QueueID != nil {
		r.Queue, r.HasQueue = *v.QueueID, true
	}
	if v.AllyTeam != nil {
		for _, p := range v.AllyTeam.Players {
			if IsUUID(p.Subject) {
				r.Players = append(r.Players, p.decode(v.AllyTeam.TeamID))
			}
		}
	}
	return r, nil
}

func DecodeCoreGame(body []byte) (Roster, error) {
	var v struct {
		MatchID         string         `json:"MatchID"`
		MapID           string         `json:"MapID"`
		ModeID          string         `json:"ModeID"`
		Players         []rosterPlayer `json:"Players"`
		MatchmakingData *struct {
			QueueID string `json:"QueueID"`
		} `json:"MatchmakingData"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Roster{}, fmt.Errorf("core-game match isn't JSON: %w", err)
	}
	r := Roster{MatchID: strings.ToLower(v.MatchID), Map: v.MapID, Mode: v.ModeID}
	if v.MatchmakingData != nil {
		r.Queue, r.HasQueue = v.MatchmakingData.QueueID, true
	}
	for _, p := range v.Players {
		if IsUUID(p.Subject) {
			r.Players = append(r.Players, p.decode(""))
		}
	}
	return r, nil
}

func truncate(b []byte) string {
	if len(b) > 80 {
		return string(b[:80]) + "..."
	}
	return string(b)
}

func DecodePartyID(body []byte) (string, error) {
	var v struct {
		CurrentPartyID string `json:"CurrentPartyID"`
	}
	if err := json.Unmarshal(body, &v); err != nil || !IsUUID(v.CurrentPartyID) {
		return "", fmt.Errorf("no valid party ID in %q", truncate(body))
	}
	return strings.ToLower(v.CurrentPartyID), nil
}

func DecodePartyMembers(body []byte) ([]string, error) {
	var v struct {
		Members []struct {
			Subject string `json:"Subject"`
		} `json:"Members"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("party isn't JSON: %w", err)
	}
	var out []string
	for _, m := range v.Members {
		if IsUUID(m.Subject) {
			out = append(out, strings.ToLower(m.Subject))
		}
	}
	return out, nil
}
