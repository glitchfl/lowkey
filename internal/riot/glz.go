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
	// Over is a core-game match that already ended - Riot keeps answering for it after the end
	Over bool
	// Range is the Range's own core-game match - lowkey treats the Range as the menus
	Range   bool
	Players []RosterPlayer
}

// Has says whether a player is in the roster - a match you left goes on without you
func (r Roster) Has(puuid string) bool {
	for _, p := range r.Players {
		if p.PUUID == puuid {
			return true
		}
	}
	return false
}

type RosterPlayer struct {
	PUUID     string
	Team      string
	Agent     string
	Incognito bool
	// Hover is agent select only - the agent is picked but not locked in yet
	Hover bool
	// Level is the account level - zero when the player hides it
	Level int
}

type rosterPlayer struct {
	Subject                 string `json:"Subject"`
	TeamID                  string `json:"TeamID"`
	CharacterID             string `json:"CharacterID"`
	CharacterSelectionState string `json:"CharacterSelectionState"`
	PlayerIdentity          struct {
		Incognito        bool `json:"Incognito"`
		AccountLevel     int  `json:"AccountLevel"`
		HideAccountLevel bool `json:"HideAccountLevel"`
	} `json:"PlayerIdentity"`
}

func (p rosterPlayer) decode(team string) RosterPlayer {
	if team == "" {
		team = p.TeamID
	}
	rp := RosterPlayer{
		PUUID: strings.ToLower(p.Subject), Team: team, Agent: strings.ToLower(p.CharacterID),
		Incognito: p.PlayerIdentity.Incognito, Hover: p.CharacterSelectionState == "selected",
	}
	if !p.PlayerIdentity.HideAccountLevel {
		rp.Level = p.PlayerIdentity.AccountLevel
	}
	return rp
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
		MatchID          string         `json:"MatchID"`
		State            string         `json:"State"`
		ProvisioningFlow string         `json:"ProvisioningFlow"`
		MapID            string         `json:"MapID"`
		ModeID           string         `json:"ModeID"`
		Players          []rosterPlayer `json:"Players"`
		MatchmakingData  *struct {
			QueueID string `json:"QueueID"`
		} `json:"MatchmakingData"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Roster{}, fmt.Errorf("core-game match isn't JSON: %w", err)
	}
	r := Roster{
		MatchID: strings.ToLower(v.MatchID), Map: v.MapID, Mode: v.ModeID,
		Over: v.State == "CLOSED" || v.State == "POST_GAME", Range: v.ProvisioningFlow == FlowRange,
	}
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

// Loadouts maps each player to what they equipped on every weapon - every UUID lowercased
type Loadouts map[string]map[string]Gun

// Gun is one weapon's skin and buddy - Buddy is empty on a gun without one
type Gun struct {
	Skin  string
	Buddy string
}

// a weapon's sockets also hold its chroma and skin level and buddy level - the buddy socket carries the
// buddy's own UUID the way VRY reads it
const (
	skinSocket  = "bcef87d6-209b-46c6-8b19-fbe40bd95abc"
	buddySocket = "77258665-71d1-4623-bc72-44db9bd5b3b3"
)

// DecodeLoadouts reads the core-game loadouts - Riot has sent the player both inside the loadout and beside it
func DecodeLoadouts(body []byte) (Loadouts, error) {
	var v struct {
		Loadouts []struct {
			Subject string `json:"Subject"`
			Loadout struct {
				Subject string `json:"Subject"`
				Items   map[string]struct {
					Sockets map[string]struct {
						Item struct {
							ID string `json:"ID"`
						} `json:"Item"`
					} `json:"Sockets"`
				} `json:"Items"`
			} `json:"Loadout"`
		} `json:"Loadouts"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("loadouts aren't JSON: %w", err)
	}
	out := Loadouts{}
	for _, l := range v.Loadouts {
		subject := l.Loadout.Subject
		if subject == "" {
			subject = l.Subject
		}
		if !IsUUID(subject) {
			continue
		}
		guns := map[string]Gun{}
		for weapon, item := range l.Loadout.Items {
			var g Gun
			for socket, s := range item.Sockets {
				if !IsUUID(s.Item.ID) {
					continue
				}
				switch strings.ToLower(socket) {
				case skinSocket:
					g.Skin = strings.ToLower(s.Item.ID)
				case buddySocket:
					g.Buddy = strings.ToLower(s.Item.ID)
				}
			}
			if g.Skin != "" {
				guns[strings.ToLower(weapon)] = g
			}
		}
		out[strings.ToLower(subject)] = guns
	}
	return out, nil
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
