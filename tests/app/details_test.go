package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const (
	allyOneWin  = "d1d1d1d1-0000-4000-8000-000000000001"
	allyOneLoss = "d1d1d1d1-0000-4000-8000-000000000002"
)

// allyOneRecord gives ally one two competitive matches with assists and round scores - one won on Jett and
// one lost on Sova - plus a level and a buddy and a last act that peaked in Ascendant
func allyOneRecord(w *fakeriot.World) {
	a1 := w.Player(ally1)
	a1.Level = 87
	a1.SetRank("competitive", fakeriot.ActLast, fakeriot.SeasonInfo{Tier: 20, Wins: 18, WinsWithPlacements: 18, Games: 30,
		WinsByTier: map[string]int{"20": 10, "21": 1}})
	a1.SetRank("unrated", fakeriot.ActCurrent, fakeriot.SeasonInfo{Wins: 5, WinsWithPlacements: 5, Games: 8})
	a1.Updates = []fakeriot.Update{{MatchID: allyOneWin, Earned: 21}, {MatchID: allyOneLoss, Earned: -17}}
	ranked := func(id, mapURL string, ago time.Duration, agent string, assists, ours, theirs int, s line5) {
		m := &fakeriot.Match{
			ID: id, Queue: "competitive", Map: mapURL, Mode: fakeriot.ModeBomb, Start: testStart.Add(-ago),
			Players: []fakeriot.MatchPlayer{
				{PUUID: ally1, Team: "Blue", Agent: agent, Score: s.score, Rounds: s.rounds, Kills: s.kills, Deaths: s.deaths, Assists: assists},
				{PUUID: enemy, Team: "Red", Agent: fakeriot.Omen, Score: 1000, Rounds: s.rounds, Kills: 1, Deaths: 1},
			},
			Teams: []fakeriot.TeamResult{{ID: "Blue", Won: ours > theirs, RoundsWon: ours}, {ID: "Red", Won: theirs > ours, RoundsWon: theirs}},
		}
		addRounds(m, ally1, s)
		w.AddMatch(m)
	}
	ranked(allyOneWin, fakeriot.MapHaven, 2*time.Hour, fakeriot.Jett, 7, 13, 9,
		line5{score: 4400, rounds: 22, kills: 22, deaths: 11, head: 6, body: 14, damage: 3300, firstBloods: 3})
	ranked(allyOneLoss, fakeriot.MapLotus, 26*time.Hour, fakeriot.Sova, 4, 7, 13,
		line5{score: 2000, rounds: 20, kills: 10, deaths: 15, head: 2, body: 8, damage: 2400, firstBloods: 1})
	wear(w, ally1, fakeriot.Vandal, fakeriot.ElderflameVandal)
	for i := range w.CoreGame.Players {
		if p := &w.CoreGame.Players[i]; p.PUUID == ally1 {
			p.Buddies = map[string]string{fakeriot.Vandal: fakeriot.SentinelsBuddy}
		}
	}
}

// flat squeezes the column padding out of a frame so whole rows can be compared
func flat(frame string) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(frame, "\n\f"), "\n") {
		lines = append(lines, strings.Join(strings.Fields(l), " "))
	}
	return lines
}

func hasRow(frame, row string) bool {
	for _, l := range flat(frame) {
		if l == row {
			return true
		}
	}
	return false
}

func wantRows(t *testing.T, frame string, rows ...string) {
	t.Helper()
	for _, row := range rows {
		if !hasRow(frame, row) {
			t.Errorf("no row %q in:\n%s", row, frame)
		}
	}
}

func pickedRow(frame string) string {
	for _, l := range flat(frame) {
		if strings.HasPrefix(l, "▸ ") {
			return strings.TrimPrefix(l, "▸ ")
		}
	}
	return ""
}

const detailsOn = "player_details = true\n"

// inMatchLoad walks a keyboard-driven lowkey with player details on into a match load and waits for the full table
func inMatchLoad(t *testing.T, setup ...func(w *fakeriot.World)) *rig {
	r := newRig(t, append([]func(w *fakeriot.World){matchLobby, lobbySkins}, setup...)...)
	r.writeSettings(detailsOn)
	r.keyboard()
	r.live()
	r.screen("[LOBBY]", "waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "AllyOne#A1")
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "US - THEM", "Elderflame")
	return r
}

func TestArrowsPickAPlayerAndEnterShowsEverythingKnownAboutThem(t *testing.T) {
	r := inMatchLoad(t, allyOneRecord)
	frame := r.con.last()
	if strings.Contains(frame, "▸") || !strings.Contains(frame, "↑↓ pick a player") {
		t.Errorf("before any key the table should look as always and say how to pick:\n%s", frame)
	}
	before := len(r.fr.RemoteRequests())

	r.press(downKey, downKey)
	frame = r.screen("enter for details")
	if got := pickedRow(frame); !strings.HasPrefix(got, "AllyOne#A1 Jett") {
		t.Errorf("two presses down should pick the first ally after you, got %q\n%s", got, frame)
	}

	r.press(enterKey)
	frame = r.settled("esc back", "THIS ACT")
	golden(t, "details", frame)
	wantRows(t, frame,
		"MATCH LOADING · Ascent · Defenders · Competitive",
		"PLAYER AGENT TEAM LEVEL RANK PEAK LAST TREND",
		"AllyOne#A1 Jett ally 87 Dia1 50 Asc1 Dia3 +4",
		"THIS ACT GAMES W-L WR",
		"Competitive 20 10-10 50%",
		"Unrated 8 5-3 63%",
		"COMPETITIVE N W-L K/D KDA HS% ACS ADR FB%",
		"all 2 1-1 1.23 1.65 27% 152 136 10%",
		"Jett 1 1-0 2.00 2.64 30% 200 150 14%",
		"Sova 1 0-1 0.67 0.93 20% 100 120 5%",
		"WHEN MAP AGENT RESULT K/D/A K/D HS% ACS ADR FB RR",
		"2h Haven Jett W 13-9 22/11/7 2.00 30% 200 150 3 +21",
		"1d Lotus Sova L 7-13 10/15/4 0.67 20% 100 120 1 -17",
		"GUN SKIN BUDDY",
		"Vandal Elderflame Sentinels of Light",
		"standard: Classic, Sheriff, Phantom, Operator, Melee",
		"ACT RANK PEAK W-L WR",
		"V26 A6 Dia1 Dia1 10-10 50%",
		"V26 A5 Dia3 Asc1 18-12 60%",
	)
	if strings.Contains(frame, "SESSION") || strings.Contains(frame, "ENEMIES") {
		t.Errorf("the details take the table's place:\n%s", frame)
	}

	r.press(escKey)
	frame = r.screen("US - THEM", "enter for details")
	if got := pickedRow(frame); !strings.HasPrefix(got, "AllyOne#A1") {
		t.Errorf("esc goes back to the table with the same player picked, got %q\n%s", got, frame)
	}
	r.press(escKey)
	frame = r.screen("↑↓ pick a player")
	if strings.Contains(frame, "▸") {
		t.Errorf("a second esc drops the pick:\n%s", frame)
	}
	settle()
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("picking and opening a player sent %d requests, want none", after-before)
	}
}

func TestYourOwnDetailsCarryEveryActYouPlayed(t *testing.T) {
	r := inMatchLoad(t)
	r.con.setHeight(60)
	r.press(downKey, enterKey)
	frame := r.settled("esc back", "E4 A3")
	// episode 4's immortal 2 was tier 22 back then and is tier 25 today
	wantRows(t, frame,
		"Kestrel#EUW Sova ally 120 Dia3 12 Imm2 Dia2 +14",
		"Competitive 21 12-9 57%",
		"Unrated 10 7-3 70%",
		"V26 A6 Dia3 Dia3 12-9 57%",
		"V26 A5 Dia2 Dia2 20-18 53%",
		"E4 A3 Imm2 Imm2 30-20 60%",
	)
}

func TestHiddenPlayersDetailsShowEverythingButTheirName(t *testing.T) {
	r := inMatchLoad(t, hiddenSamples)
	r.press(downKey, downKey, downKey, downKey, enterKey)
	frame := r.settled("esc back", "THIS ACT")
	wantRows(t, frame,
		"hidden Omen ally 120 Rad 900 Rad - -",
		"Competitive 100 90-10 90%",
		"all 2 2-0 3.00 3.00 25% 200 150 20%",
		"Vandal Prime",
	)
	if !hasRow(frame, "1d Haven Omen W 0-0 30/10/0 3.00 25% 200 150 4") {
		t.Errorf("the hidden ally's own matches should be listed:\n%s", frame)
	}
	if strings.Contains(frame, "MET") {
		t.Errorf("meeting a hidden player stays off their details too:\n%s", frame)
	}
}

func TestAHiddenAccountLevelStaysHidden(t *testing.T) {
	r := inMatchLoad(t, func(w *fakeriot.World) { w.Player(ally1).HideLevel = true })
	r.press(downKey, downKey, enterKey)
	frame := r.settled("esc back", "AllyOne#A1")
	if strings.Contains(frame, "LEVEL") || strings.Contains(frame, "120") {
		t.Errorf("a player who hides their level shouldn't have one shown:\n%s", frame)
	}
}

func TestPlayersLowkeyDidntLookUpSayWhy(t *testing.T) {
	r := newRig(t, matchLobby)
	r.writeSettings("enemy_lookups = false\n" + detailsOn)
	r.keyboard()
	r.live()
	r.screen("[LOBBY]", "waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "US - THEM")
	// you and four allies come first
	r.press(downKey, downKey, downKey, downKey, downKey, downKey, enterKey)
	frame := r.settled("esc back", "Raze")
	if !strings.Contains(frame, "not looked up - switched off in the settings") || strings.Contains(frame, "THIS ACT") {
		t.Errorf("an enemy with enemy lookups off should only say why:\n%s", frame)
	}
}

func TestLeftAndRightWalkThePlayersAndANewLobbyTakesTheScreenBack(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.writeSettings(detailsOn)
	r.keyboard()
	r.live()
	r.screen("[LOBBY]", "waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "AllyOne#A1")
	r.press(downKey, enterKey)
	frame := r.settled("esc back", "Kestrel#EUW")
	if strings.Contains(frame, "TEAM") {
		t.Errorf("agent select is all allies - no team column:\n%s", frame)
	}
	r.press(rightKey)
	r.settled("esc back", "AllyOne#A1", "(Jett)")
	r.press(leftKey, leftKey)
	r.settled("esc back", "Sage")

	var dodged *fakeriot.LiveGame
	r.presence(func(w *fakeriot.World) { w.Loop, w.Pregame, dodged = fakeriot.LoopMenus, nil, w.Pregame })
	frame = r.screen("waiting for the next match")
	if strings.Contains(frame, "esc back") {
		t.Errorf("a dodge closes the details:\n%s", frame)
	}
	const requeued = "9e000000-0000-4000-8000-000000000002"
	r.fr.Update(func(w *fakeriot.World) {
		next := *dodged
		next.ID = requeued
		w.Pregame = &next
	})
	r.enterAgentSelectWith(requeued)
	frame = r.settled("AGENT SELECT", "AllyOne#A1", "↑↓ pick a player")
	if strings.Contains(frame, "▸") {
		t.Errorf("a new agent select starts with nobody picked:\n%s", frame)
	}
}

func TestAnOpenDetailsSurvivesTheCutoffAndTheMatchEnding(t *testing.T) {
	r := inMatchLoad(t, allyOneRecord)
	r.press(downKey, downKey, enterKey)
	r.settled("esc back", "AllyOne#A1")
	before := len(r.fr.RemoteRequests())

	r.clock.Advance(40 * time.Second)
	r.settled("IN MATCH", "esc back")
	r.press(rightKey)
	r.settled("IN MATCH", "Killjoy", "esc back")
	r.press(leftKey)
	frame := r.settled("IN MATCH", "AllyOne#A1", "Haven")
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("details mid-round sent %d requests, want none", after-before)
	}

	r.clock.Advance(35 * time.Minute)
	r.presence(func(w *fakeriot.World) { w.Loop, w.CoreGame = fakeriot.LoopMenus, nil })
	frame = r.screen("LAST MATCH", "esc back", "AllyOne#A1")
	if !hasRow(frame, "Vandal Elderflame Sentinels of Light") {
		t.Errorf("last match keeps the details open:\n%s", frame)
	}
}

func TestLongDetailsScrollAndStopAtTheEnd(t *testing.T) {
	r := inMatchLoad(t, allyOneRecord)
	r.con.setHeight(16)
	r.press(downKey, downKey, enterKey)
	frame := r.settled("esc back", "below")
	if n := len(flat(frame)); n > 15 {
		t.Errorf("a 16 row window got %d lines:\n%s", n, frame)
	}
	if !strings.HasPrefix(flat(frame)[0], "[LOBBY]") || !strings.HasPrefix(flat(frame)[1], "MATCH LOADING") {
		t.Errorf("the tab bar and header stay on top:\n%s", frame)
	}

	for range 40 {
		r.press(downKey)
	}
	frame = r.settled("esc back", "V26 A5")
	last := flat(frame)[len(flat(frame))-1]
	if !strings.HasPrefix(last, "↑") || strings.Contains(last, "below") {
		t.Errorf("scrolled to the end the last line should only point up, got %q\n%s", last, frame)
	}
	r.press(upKey)
	frame = r.settled("esc back", "↓ 1 below")
	if hasRow(frame, "V26 A5 Dia3 Asc1 18-12 60%") {
		t.Errorf("one press up from the end should scroll right away:\n%s", frame)
	}
}

func TestTheDetailsGunsComeEvenWithTheSkinColumnOff(t *testing.T) {
	noSkins := `columns = ["name", "agent", "rank"]` + "\n"
	t.Run("with player details on", func(t *testing.T) {
		r := newRig(t, matchLobby, lobbySkins)
		r.writeSettings(noSkins + detailsOn)
		r.keyboard()
		r.live()
		r.screen("[LOBBY]", "waiting for the next match")
		r.subscribed()
		r.enterMatchLoad()
		r.settled("MATCH LOADING", "US - THEM")
		r.press(downKey, enterKey)
		frame := r.settled("esc back", "Reaver")
		wantRows(t, frame, "Phantom Ion", "Vandal Reaver")
		if n := requestsExact(r, loadouts); n != 1 {
			t.Errorf("loadouts fetched %d times, want once", n)
		}
	})
	for _, c := range []struct {
		name, settings string
		keyboard       bool
	}{
		{"with player details off", noSkins, true},
		{"without a keyboard", noSkins + detailsOn, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, matchLobby, lobbySkins)
			r.writeSettings(c.settings)
			if c.keyboard {
				r.keyboard()
			}
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.enterMatchLoad()
			r.settled("MATCH LOADING", "US - THEM")
			if n := requestsExact(r, loadouts); n != 0 {
				t.Errorf("nobody can open the details or see a skin column - loadouts fetched %d times", n)
			}
		})
	}
}

func TestTheDetailsHaveNoKeysWithoutAKeyboard(t *testing.T) {
	r := newRig(t, matchLobby)
	r.writeSettings(detailsOn)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "US - THEM")
	if strings.Contains(frame, "pick a player") {
		t.Errorf("no keyboard means nothing to pick with:\n%s", frame)
	}
}

func TestTheArrowsPickNobodyUntilPlayerDetailsAreSwitchedOn(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins, allyOneRecord)
	r.keyboard()
	r.live()
	r.screen("[LOBBY]", "waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "US - THEM", "Elderflame")
	if strings.Contains(frame, "pick a player") {
		t.Errorf("player details are off by default - nothing should offer them:\n%s", frame)
	}
	r.press(downKey, downKey, enterKey)
	settle()
	frame = r.screen("US - THEM")
	if strings.Contains(frame, "▸") || strings.Contains(frame, "esc back") {
		t.Errorf("with player details off the arrows and enter should do nothing:\n%s", frame)
	}

	r.writeSettings(detailsOn)
	r.screen("US - THEM", "↑↓ pick a player")
	r.press(downKey, downKey, enterKey)
	r.settled("esc back", "AllyOne#A1", "THIS ACT")

	r.writeSettings("player_details = false\n")
	frame = r.screen("US - THEM")
	if strings.Contains(frame, "▸") || strings.Contains(frame, "esc back") || strings.Contains(frame, "pick a player") {
		t.Errorf("switching player details off should close them and drop the pick:\n%s", frame)
	}
	r.writeSettings(detailsOn)
	frame = r.screen("US - THEM", "↑↓ pick a player")
	if strings.Contains(frame, "▸") {
		t.Errorf("switching them back on should start with nobody picked:\n%s", frame)
	}
}
