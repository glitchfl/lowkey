package riot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Private struct {
	SessionLoopState   string
	ProvisioningFlow   string
	PartyID            string
	PartyClientVersion string
	QueueID            string
	HasQueueID         bool
	Shape              string // flat / nested / hybrid
}

const (
	LoopMenus   = "MENUS"
	LoopPregame = "PREGAME"
	LoopIngame  = "INGAME"
)

// FlowRange is the provisioning flow of the Range - in your presence and in its core-game match
const FlowRange = "ShootingRange"

// fields moved under these objects in later presence versions - some clients mix both layouts
var nestedPresence = []string{"matchPresenceData", "partyPresenceData", "playerPresenceData"}

func ParsePrivate(encoded string) (Private, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return Private{}, fmt.Errorf("private presence isn't base64: %w", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return Private{}, fmt.Errorf("private presence isn't JSON: %w", err)
	}
	var nested []map[string]json.RawMessage
	for _, key := range nestedPresence {
		var m map[string]json.RawMessage
		if json.Unmarshal(top[key], &m) == nil && m != nil {
			nested = append(nested, m)
		}
	}

	topHits, nestedHits := 0, 0
	find := func(key string) (string, bool) {
		if v, ok := jsonString(top[key]); ok {
			topHits++
			return v, true
		}
		for _, m := range nested {
			if v, ok := jsonString(m[key]); ok {
				nestedHits++
				return v, true
			}
		}
		return "", false
	}

	var p Private
	var ok bool
	if p.SessionLoopState, ok = find("sessionLoopState"); !ok {
		p.SessionLoopState, _ = find("partyOwnerSessionLoopState")
	}
	if p.ProvisioningFlow, ok = find("provisioningFlow"); !ok {
		p.ProvisioningFlow, _ = find("partyOwnerProvisioningFlow")
	}
	p.PartyID, _ = find("partyId")
	p.PartyClientVersion, _ = find("partyClientVersion")
	p.QueueID, p.HasQueueID = find("queueId")
	switch {
	case nestedHits == 0:
		p.Shape = "flat"
	case topHits == 0:
		p.Shape = "nested"
	default:
		p.Shape = "hybrid"
	}
	return p, nil
}

// InRange is your presence in the Range - you can queue from it so it isn't a match
func (p Private) InRange() bool {
	return strings.EqualFold(p.SessionLoopState, LoopIngame) && p.ProvisioningFlow == FlowRange
}

func jsonString(raw json.RawMessage) (string, bool) {
	if raw == nil {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}
