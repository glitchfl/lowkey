package app

import (
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
)

// party is your own party - agent select and match load share it so it's fetched once per match
type party struct {
	members map[string]bool
}

// fetchParty only ever asks about your own PUUID
func (lv *live) fetchParty(p *party) {
	lv.eng.submit(&job{prio: prioParty, svc: remote.GLZ, path: "/parties/v1/players/" + lv.self}, func(body []byte, err error) {
		var id string
		if err == nil {
			id, err = riot.DecodePartyID(body)
		}
		if err != nil {
			return
		}
		lv.eng.submit(&job{prio: prioParty, svc: remote.GLZ, path: "/parties/v1/parties/" + id}, func(body []byte, err error) {
			if err != nil {
				return
			}
			members, err := riot.DecodePartyMembers(body)
			if err != nil {
				return
			}
			p.members = map[string]bool{}
			for _, m := range members {
				p.members[m] = true
			}
		})
	})
}
