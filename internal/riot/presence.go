package riot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type Private struct {
	SessionLoopState   string
	PartyID            string
	PartyClientVersion string
	QueueID            string
	HasQueueID         bool
	ProvisioningFlow   string
	Shape              string // flat / nested / hybrid
}

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
	p.PartyID, _ = find("partyId")
	p.PartyClientVersion, _ = find("partyClientVersion")
	p.QueueID, p.HasQueueID = find("queueId")
	p.ProvisioningFlow, _ = find("provisioningFlow")
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
