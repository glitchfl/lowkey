package lobbytable

import (
	"fmt"
	"slices"
	"strings"

	"github.com/glitchfl/lowkey/internal/stats"
)

const ownPartyMarker = "●"

// likelySymbols avoid ▲ and ▼ - the diff row uses those
var likelySymbols = []string{"◆", "■", "◇", "□", "★"}

// markers maps PUUIDs to their party column symbol - a hidden player never gets one and never links others
// since groups chain through their members
func markers(lob Lobby, exact bool) map[string]string {
	out := map[string]string{}
	var own []string
	for _, m := range lob.Members {
		if !m.Visibility.Hidden && lob.Party[m.PUUID] {
			own = append(own, m.PUUID)
		}
	}
	if len(own) >= 2 {
		for _, p := range own {
			out[p] = ownPartyMarker
		}
	}
	if !lob.Mode.Teams {
		return out
	}

	var candidates []Member
	for _, m := range lob.Members {
		if !m.Visibility.Hidden && m.Player != nil && !lob.Party[m.PUUID] && m.PUUID != lob.Self {
			candidates = append(candidates, m)
		}
	}
	// letters are parties a past match's details prove - shapes are only guesses from shared teams
	if exact {
		var enemies []Member
		for _, m := range candidates {
			if m.Team != lob.OwnTeam {
				enemies = append(enemies, m)
			}
		}
		proven := map[string]bool{}
		for i, g := range groups(enemies, func(a, b Member) bool { return a.Team == b.Team && sharedParty(a.Player, b.Player) }) {
			for _, m := range g {
				out[m.PUUID] = fmt.Sprintf("[%c]", 'A'+i)
				proven[m.PUUID] = true
			}
		}
		candidates = slices.DeleteFunc(candidates, func(m Member) bool { return proven[m.PUUID] })
	}
	for i, g := range groups(candidates, func(a, b Member) bool { return a.Team == b.Team && sharedTeams(a.Player, b.Player) >= 2 }) {
		for _, m := range g {
			out[m.PUUID] = likelySymbols[i%len(likelySymbols)]
		}
	}
	return out
}

// groups joins members linked directly or through each other - a lone member is no group
func groups(ms []Member, linked func(a, b Member) bool) [][]Member {
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
	byRoot := map[int][]Member{}
	var order []int
	for i, m := range ms {
		r := find(i)
		if byRoot[r] == nil {
			order = append(order, r)
		}
		byRoot[r] = append(byRoot[r], m)
	}
	var out [][]Member
	for _, r := range order {
		if len(byRoot[r]) >= 2 {
			out = append(out, byRoot[r])
		}
	}
	return out
}

const noParty = "00000000-0000-0000-0000-000000000000"

// sharedParty is any fetched sample match where both players had the same party ID
func sharedParty(a, b *Player) bool {
	seen := map[string]bool{}
	for _, list := range [][]*stats.Match{a.Matches.Value, b.Matches.Value} {
		for _, m := range list {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			pa, pb := m.Players[a.PUUID], m.Players[b.PUUID]
			if pa != nil && pb != nil && pa.PartyID != "" && pa.PartyID != noParty && strings.EqualFold(pa.PartyID, pb.PartyID) {
				return true
			}
		}
	}
	return false
}

// sharedTeams counts fetched sample matches where both players were on the same team
func sharedTeams(a, b *Player) int {
	seen := map[string]bool{}
	n := 0
	for _, list := range [][]*stats.Match{a.Matches.Value, b.Matches.Value} {
		for _, m := range list {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			pa, pb := m.Players[a.PUUID], m.Players[b.PUUID]
			if pa != nil && pb != nil && pa.Team == pb.Team {
				n++
			}
		}
	}
	return n
}
