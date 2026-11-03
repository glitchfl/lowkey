package lobbytable

import "github.com/glitchfl/lowkey/internal/riot"

// Visibility is what lowkey may do with one lobby player
type Visibility struct {
	Stats  bool // looked up - so stats skins and met-before can show
	Hidden bool // streamer mode - never their name and never tied to anyone
}

// Rules are what one roster's visibility depends on
type Rules struct {
	Self              string
	OwnTeam           string
	MatchLoad         bool // only match load's roster has the other team
	HiddenPlayerStats bool
	EnemyLookups      bool
}

// Decide is a player's visibility on this roster alone - the guard follows it every roster so a setting
// switched off refuses their requests at once
func (r Rules) Decide(p riot.RosterPlayer) Visibility {
	hidden := p.Incognito && p.PUUID != r.Self
	enemy := r.MatchLoad && p.Team != r.OwnTeam
	return Visibility{Stats: !(hidden && !r.HiddenPlayerStats) && !(enemy && !r.EnemyLookups), Hidden: hidden}
}

// Next is a member's visibility after another roster - stats stay as first decided until the next lobby
// and streamer mode coming on between rosters still takes a name already on screen away
func (v Visibility) Next(now Visibility) Visibility {
	return Visibility{Stats: v.Stats, Hidden: v.Hidden || now.Hidden}
}
