package fakeriot

import "time"

type Shape int

const (
	Nested Shape = iota
	Flat
	Hybrid
)

const (
	LoopMenus   = "MENUS"
	LoopPregame = "PREGAME"
	LoopIngame  = "INGAME"
)

// FlowRange is the provisioning flow your presence shows in the Range
const FlowRange = "ShootingRange"

const (
	Astra     = "41fb69c1-4189-7b37-f117-bcaf1e96f1bf"
	Breach    = "5f8d3a7f-467b-97f3-062c-13acf203c006"
	Brimstone = "9f0d8ba9-4140-b941-57d3-a7ad57c6b417"
	Chamber   = "22697a3d-45bf-8dd7-4fec-84a9e28c69d7"
	Clove     = "1dbf2edd-4729-0984-3115-daa5eed44993"
	Cypher    = "117ed9e3-49f3-6512-3ccf-0cada7e3823b"
	Fade      = "dade69b4-4f5a-8528-247b-219e5a1facd6"
	Jett      = "add6443a-41bd-e414-f6ad-e58d267f4e95"
	KAYO      = "601dbbe7-43ce-be57-2a40-4abd24953621"
	Killjoy   = "1e58de9c-4950-5125-93e9-a0aee9f98746"
	Neon      = "bb2a4828-46eb-8cd1-e765-15848195d751"
	Omen      = "8e253930-4c05-31dd-1b6c-968525494517"
	Phoenix   = "eb93336a-449b-9c1b-0a54-a891f7921d69"
	Raze      = "f94c3b30-42be-e959-889c-5aa313dba261"
	Reyna     = "a3bfb853-43b2-7238-a4f1-ad90e9e46bcc"
	Sage      = "569fdd95-4d10-43ab-ca70-79becc718b46"
	Skye      = "6f2a04ca-43e0-be17-7f36-b3908627744d"
	Sova      = "320b2a48-4d9b-a075-30f1-1f93a9b638fa"
	Viper     = "707eab51-4836-f488-046a-cda6bf494859"
	Yoru      = "7f94d92c-4234-0a36-9646-3a87eb8b5c89"
)

// real act IDs so the embedded tier sets apply to them
const (
	ActCurrent  = "d816f426-48ea-f052-117f-9697a155b319"
	ActLast     = "8102cd81-43a0-d0d7-bd59-47b8fe9bed1b"
	ActEarlier  = "4f0864e2-40af-28a4-de2c-0e9e64e75f23"
	ActEp1      = "0530b9c4-4980-f2ee-df5d-09864cd00542"
	ActEp2      = "97b6e739-44cc-ffa7-49ad-398ba502ceb0"
	ActEp4      = "3e47230a-463c-a301-eb7d-67bb60357d4f"
	ActEp5      = "67e373c7-48f7-b422-641b-079ace30b427"
	EpisodeV26  = "3737c391-497a-6e82-aeb5-cc9f701f72e2"
	EpisodeV26a = "15a01c8a-4637-7b7b-de7b-9f906719c020"
)

const (
	MapAscent = "/Game/Maps/Ascent/Ascent"
	MapHaven  = "/Game/Maps/Triad/Triad"
	MapBind   = "/Game/Maps/Duality/Duality"
	MapLotus  = "/Game/Maps/Jam/Jam"
)

const (
	ModeBomb       = "/Game/GameModes/Bomb/BombGameMode.BombGameMode_C"
	ModeDeathmatch = "/Game/GameModes/Deathmatch/DeathmatchGameMode.DeathmatchGameMode_C"
	ModeTDM        = "/Game/GameModes/HURM/HURMGameMode.HURMGameMode_C"
	ModeSwiftplay  = "/Game/GameModes/_Development/Swiftplay_EndOfRoundCredits/Swiftplay_EoRCredits_GameMode.Swiftplay_EoRCredits_GameMode_C"
)

const (
	DefaultClientVersion = "release-13.08-shipping-20-5715960"
	DefaultSelf          = "5e1f0000-0000-4000-8000-000000000001"
)

type World struct {
	Self    string
	Players map[string]*Player
	Matches map[string]*Match
	Seasons []Season

	RiotClient bool
	Valorant   bool
	Region     string

	Loop            string
	Flow            string // provisioning flow in your presence - empty is Matchmaking
	Queue           string
	QueueInPresence bool
	PartyID         string
	PartyMembers    []string
	PresenceVersion string
	StaticVersion   string
	Shape           Shape

	Pregame  *LiveGame
	CoreGame *LiveGame

	// recorded responses keyed by "service path?query" - served before anything modeled above
	Raw map[string]RawResponse

	TokenLifetime time.Duration
	// zero gives the entitlements token the same lifetime as the access token
	EntitlementLifetime time.Duration
	TokenNotReady       int // entitlements answers "not ready" this many more times

	// what valorant-api.com knows beyond lowkey's snapshot - agent UUID or map URL to name
	StaticAgents map[string]string
	StaticMaps   map[string]string
}

type RawResponse struct {
	Status int
	Body   []byte
}

type Player struct {
	PUUID     string
	Name      string
	Tag       string
	Incognito bool
	Ranks     map[string]map[string]SeasonInfo
	Updates   []Update // newest first like the real endpoint
}

type SeasonInfo struct {
	Tier               int
	RR                 int
	Wins               int
	WinsWithPlacements int
	Games              int
	WinsByTier         map[string]int // nil is sent as null like brand-new accounts get
}

type Update struct {
	MatchID string
	Start   time.Time
	Earned  int
}

type Season struct {
	ID     string
	Name   string
	Type   string
	Start  time.Time
	End    time.Time
	Active bool
}

type Match struct {
	ID      string
	Queue   string
	Map     string
	Mode    string
	Start   time.Time
	Players []MatchPlayer
	Teams   []TeamResult
	Rounds  []Round
	// community docs say roundTime - the spec says timeSinceRoundStartMillis - lowkey must read both
	KillTimeField string
	// Heavy fills in kill positions plus economies and scores so the JSON is as big as a real match
	Heavy bool
}

type MatchPlayer struct {
	PUUID string
	// the Riot ID at the time of the match - empty uses the player's current one
	Name   string
	Tag    string
	Team   string
	Agent  string
	Score  int
	Rounds int
	Kills  int
	Deaths int
}

type TeamResult struct {
	ID        string
	Won       bool
	RoundsWon int
}

type Round struct {
	Kills  []Kill
	Damage []Hit
}

type Kill struct {
	Killer string
	Victim string
	At     int
}

type Hit struct {
	From   string
	To     string
	Damage int
	Head   int
	Body   int
	Leg    int
}

type LiveGame struct {
	ID       string
	Queue    string // core-game sends it in MatchmakingData - empty sends MatchmakingData null
	Map      string
	Mode     string
	Ranked   bool
	Players  []LivePlayer
	Observer []string
}

type LivePlayer struct {
	PUUID string
	Team  string
	Agent string
	State string
}

func NewWorld(now time.Time) *World {
	w := &World{
		Self:            DefaultSelf,
		Players:         map[string]*Player{},
		Matches:         map[string]*Match{},
		RiotClient:      true,
		Valorant:        true,
		Region:          "eu",
		Loop:            LoopMenus,
		QueueInPresence: true,
		PartyID:         "9a870000-0000-4000-8000-000000000001",
		PresenceVersion: DefaultClientVersion,
		StaticVersion:   DefaultClientVersion,
		Shape:           Nested,
		Raw:             map[string]RawResponse{},
		TokenLifetime:   time.Hour,
		StaticAgents:    map[string]string{Jett: "Jett", Sova: "Sova", Omen: "Omen"},
		StaticMaps:      map[string]string{MapAscent: "Ascent"},
	}
	w.PartyMembers = []string{w.Self}
	w.Players[w.Self] = &Player{PUUID: w.Self, Name: "Kestrel", Tag: "EUW"}
	w.Seasons = DefaultSeasons()
	return w
}

// mirrors the content service in october 2026 - names repeat and the order isn't chronological
func DefaultSeasons() []Season {
	d := func(s string) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			panic(err)
		}
		return t
	}
	return []Season{
		{ID: ActEp1, Name: "ACT II", Type: "act", Start: d("2020-08-04T00:00:00Z"), End: d("2020-10-13T00:00:00Z")},
		{ID: ActEp2, Name: "ACT I", Type: "act", Start: d("2021-01-12T00:00:00Z"), End: d("2021-03-02T00:00:00Z")},
		{ID: ActEp4, Name: "ACT III", Type: "act", Start: d("2022-04-26T00:00:00Z"), End: d("2022-06-21T00:00:00Z")},
		{ID: ActEp5, Name: "ACT I", Type: "act", Start: d("2022-06-21T00:00:00Z"), End: d("2022-08-23T00:00:00Z")},
		{ID: EpisodeV26a, Name: "V26", Type: "episode", Start: d("2026-01-07T00:00:00Z"), End: d("2026-06-24T00:00:00Z")},
		{ID: EpisodeV26, Name: "V26", Type: "episode", Start: d("2026-06-24T00:00:00Z"), End: d("2027-01-06T00:00:00Z"), Active: true},
		{ID: ActEarlier, Name: "ACT IV", Type: "act", Start: d("2026-06-24T00:00:00Z"), End: d("2026-08-19T00:00:00Z")},
		{ID: ActCurrent, Name: "ACT VI", Type: "act", Start: d("2026-10-14T00:00:00Z"), End: d("2027-01-06T00:00:00Z"), Active: true},
		{ID: ActLast, Name: "ACT V", Type: "act", Start: d("2026-08-19T00:00:00Z"), End: d("2026-10-14T00:00:00Z")},
	}
}

func (w *World) Player(puuid string) *Player {
	p, ok := w.Players[puuid]
	if !ok {
		p = &Player{PUUID: puuid}
		w.Players[puuid] = p
	}
	return p
}

func (p *Player) SetRank(queue, act string, info SeasonInfo) {
	if p.Ranks == nil {
		p.Ranks = map[string]map[string]SeasonInfo{}
	}
	if p.Ranks[queue] == nil {
		p.Ranks[queue] = map[string]SeasonInfo{}
	}
	p.Ranks[queue][act] = info
}

func (w *World) AddMatch(m *Match) {
	w.Matches[m.ID] = m
}
