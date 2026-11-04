package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/stats"
)

// party is your own party - agent select and match load share it so it's fetched once per match
type party struct {
	members map[string]bool
}

const ownPartyMarker = "●"

// likelySymbols avoid ▲ and ▼ - the diff row uses those
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
	// letters are parties a past match's details prove - shapes are only guesses from shared teams
	if lv.cfg.Settings().ExactEnemyParties {
		var enemies []*member
		for _, m := range candidates {
			if m.team != lob.ownTeam {
				enemies = append(enemies, m)
			}
		}
		proven := map[string]bool{}
		for i, g := range groups(enemies, func(a, b *member) bool { return a.team == b.team && sharedParty(a.p, b.p) }) {
			for _, m := range g {
				out[m.puuid] = fmt.Sprintf("[%c]", 'A'+i)
				proven[m.puuid] = true
			}
		}
		candidates = slices.DeleteFunc(candidates, func(m *member) bool { return proven[m.puuid] })
	}
	for i, g := range groups(candidates, func(a, b *member) bool { return a.team == b.team && sharedTeams(a.p, b.p) >= 2 }) {
		for _, m := range g {
			out[m.puuid] = likelySymbols[i%len(likelySymbols)]
		}
	}
	return out
}

// groups joins members linked directly or through each other - a lone member is no group
func groups(ms []*member, linked func(a, b *member) bool) [][]*member {
	root := make([]int, len(ms))
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if root[i] != i {
			root[i] = find(root[i])
		}
		return root[i]
	}
	for i, a := range ms {
		for j := i + 1; j < len(ms); j++ {
			if linked(a, ms[j]) {
				root[find(j)] = find(i)
			}
		}
	}
	byRoot := map[int][]*member{}
	var order []int
	for i, m := range ms {
		r := find(i)
		if byRoot[r] == nil {
			order = append(order, r)
		}
		byRoot[r] = append(byRoot[r], m)
	}
	var out [][]*member
	for _, r := range order {
		if len(byRoot[r]) >= 2 {
			out = append(out, byRoot[r])
		}
	}
	return out
}

const noParty = "00000000-0000-0000-0000-000000000000"

// sharedParty is any fetched sample match where both players had the same party ID
func sharedParty(a, b *player) bool {
	seen := map[string]bool{}
	for _, list := range [][]*stats.Match{a.loaded(), b.loaded()} {
		for _, m := range list {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			pa, pb := m.Players[a.puuid], m.Players[b.puuid]
			if pa != nil && pb != nil && pa.PartyID != "" && pa.PartyID != noParty && strings.EqualFold(pa.PartyID, pb.PartyID) {
				return true
			}
		}
	}
	return false
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
