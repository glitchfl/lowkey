package app

import (
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/stats"
)

// party is your own party - agent select and match load share it so it's fetched once per match
type party struct {
	members map[string]bool
}

const ownPartyMarker = "●"

// likelySymbols avoid ▲ and ▼, which the diff row uses
var likelySymbols = []string{"◆", "■", "◇", "□", "★"}

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

// markers maps PUUIDs to their party column symbol - hidden players never get one
func (lv *live) markers(lob *lobby) map[string]string {
	out := map[string]string{}
	var own []string
	for _, m := range lob.members {
		if !m.hidden && lob.party != nil && lob.party.members[m.puuid] {
			own = append(own, m.puuid)
		}
	}
	if len(own) >= 2 {
		for _, p := range own {
			out[p] = ownPartyMarker
		}
	}
	if !lob.mode.Teams {
		return out
	}

	var candidates []*member
	for _, m := range lob.members {
		inParty := lob.party != nil && lob.party.members[m.puuid]
		if !m.hidden && m.p != nil && !inParty && m.puuid != lv.self {
			candidates = append(candidates, m)
		}
	}
	group := map[string]string{}
	var find func(string) string
	find = func(p string) string {
		if group[p] == "" || group[p] == p {
			return p
		}
		root := find(group[p])
		group[p] = root
		return root
	}
	for i, a := range candidates {
		for _, b := range candidates[i+1:] {
			if a.team == b.team && sharedTeams(a.p, b.p) >= 2 {
				group[find(b.puuid)] = find(a.puuid)
			}
		}
	}
	size := map[string]int{}
	for _, m := range candidates {
		size[find(m.puuid)]++
	}
	symbol := map[string]string{}
	for _, m := range candidates {
		root := find(m.puuid)
		if size[root] < 2 {
			continue
		}
		if _, ok := symbol[root]; !ok {
			symbol[root] = likelySymbols[len(symbol)%len(likelySymbols)]
		}
		out[m.puuid] = symbol[root]
	}
	return out
}

// sharedTeams counts fetched sample matches where both players were on the same team
func sharedTeams(a, b *player) int {
	seen := map[string]bool{}
	n := 0
	for _, list := range [][]*stats.Match{a.loaded(), b.loaded()} {
		for _, m := range list {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			pa, pb := m.Players[a.puuid], m.Players[b.puuid]
			if pa != nil && pb != nil && pa.Team == pb.Team {
				n++
			}
		}
	}
	return n
}
