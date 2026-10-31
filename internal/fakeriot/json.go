package fakeriot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// response shapes follow techchrism's valorant-api-docs (MIT)

type obj = map[string]any

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func millis(t time.Time) int64 { return t.UnixMilli() }

func identity(puuid string, incognito bool) obj {
	return obj{
		"Subject":                puuid,
		"PlayerCardID":           "9fb348bc-41a0-91ad-8a3e-818035c4e561",
		"PlayerTitleID":          "d13e579c-435e-44d4-cec2-6eae5a3c5ed4",
		"AccountLevel":           120,
		"PreferredLevelBorderID": "",
		"Incognito":              incognito,
		"HideAccountLevel":       false,
	}
}

func badge() obj {
	return obj{"SeasonID": "", "NumberOfWins": 0, "WinsByTier": nil, "Rank": 0, "LeaderboardRank": 0}
}

func (w *World) incognito(puuid string) bool {
	if p, ok := w.Players[puuid]; ok {
		return p.Incognito
	}
	return false
}

func (w *World) privatePresence() []byte {
	match := obj{
		"sessionLoopState": w.Loop,
		"provisioningFlow": w.flow(),
		"matchMap":         "",
	}
	if w.QueueInPresence {
		match["queueId"] = w.Queue
	}
	party := obj{
		"partyId":                       w.PartyID,
		"isPartyOwner":                  true,
		"partyState":                    "DEFAULT",
		"partyAccessibility":            "CLOSED",
		"maxPartySize":                  5,
		"partyLFM":                      false,
		"partySize":                     len(w.PartyMembers),
		"partyOwnerSessionLoopState":    w.Loop,
		"partyOwnerMatchScoreAllyTeam":  w.AllyScore,
		"partyOwnerMatchScoreEnemyTeam": w.EnemyScore,
	}
	if w.PresenceVersion != "" {
		party["partyClientVersion"] = w.PresenceVersion
	}
	player := obj{
		"playerCardId":        "9fb348bc-41a0-91ad-8a3e-818035c4e561",
		"playerTitleId":       "d13e579c-435e-44d4-cec2-6eae5a3c5ed4",
		"accountLevel":        120,
		"competitiveTier":     0,
		"leaderboardPosition": 0,
	}

	out := obj{"isValid": true, "isIdle": false}
	switch w.Shape {
	case Flat:
		for _, part := range []obj{match, party, player} {
			for k, v := range part {
				out[k] = v
			}
		}
	case Nested:
		out["matchPresenceData"] = match
		out["partyPresenceData"] = party
		out["playerPresenceData"] = player
	case Hybrid:
		// session loop state nested - party fields at the top level
		out["matchPresenceData"] = match
		for k, v := range party {
			out[k] = v
		}
		out["playerPresenceData"] = player
	}
	return mustJSON(out)
}

func (w *World) presences() []byte {
	list := []obj{}
	friend := obj{
		"actor": nil, "basic": "", "details": nil, "game_name": "friend", "game_tag": "0001",
		"location": nil, "msg": nil, "name": "", "patchline": nil,
		"pid": "f81e0000-0000-4000-8000-000000000001@eu1.pvp.net", "platform": nil,
		"private": nil, "privateJwt": nil, "product": "league_of_legends",
		"puuid": "f81e0000-0000-4000-8000-000000000001", "region": "eu1", "resource": "",
		"state": "chat", "summary": "", "time": 1761400000000,
	}
	list = append(list, friend)
	if w.Valorant {
		self := w.Players[w.Self]
		list = append(list, obj{
			"actor": nil, "basic": "", "details": nil, "game_name": self.Name, "game_tag": self.Tag,
			"location": nil, "msg": nil, "name": "", "patchline": nil,
			"pid": w.Self + "@eu1.pvp.net", "platform": nil,
			"private":    base64.StdEncoding.EncodeToString(w.privatePresence()),
			"privateJwt": nil, "product": "valorant", "puuid": w.Self, "region": "eu1",
			"resource": "RC-1234", "state": "chat", "summary": "", "time": 1761400000000,
		})
	}
	return mustJSON(obj{"presences": list})
}

func (w *World) sessions() []byte {
	out := obj{
		"host_app": obj{
			"exitCode": 0, "exitReason": nil, "isInternal": true,
			"launchConfiguration": obj{
				"arguments": []string{}, "executable": "C:/Riot Games/Riot Client/RiotClientServices.exe",
				"locale": nil, "voiceLocale": nil, "workingDirectory": "C:/Riot Games/Riot Client",
			},
			"patchlineFullName": "riot_client", "patchlineId": "", "phase": "None",
			"productId": "riot_client", "version": "111.0.0.3261.5663",
		},
	}
	if w.Valorant {
		args := []string{
			"-ares-deployment=" + w.Region,
			"-config-endpoint=https://shared." + w.Region + ".a.pvp.net",
			"-launch-product=valorant",
			"-launch-patchline=live",
			"-riotclient-auth-token=8yVg0EMbWbvC9b4yUP5Bjw",
			"-riotclient-app-port=52113",
			"-subject=" + w.Self,
		}
		if w.Region == "" {
			args = args[1:]
		}
		out["5c3f2b8a-0000-4000-8000-00000000000a"] = obj{
			"exitCode": 0, "exitReason": nil, "isInternal": false,
			"launchConfiguration": obj{
				"arguments": args, "executable": "C:/Riot Games/VALORANT/live/VALORANT.exe",
				"locale": "en_US", "voiceLocale": nil, "workingDirectory": "C:/Riot Games/VALORANT/live",
			},
			"patchlineFullName": "VALORANT", "patchlineId": "live", "phase": "Gameplay",
			"productId": "valorant", "version": "13.8.0.5715960",
		}
	}
	return mustJSON(out)
}

func (w *World) mmr(puuid string) []byte {
	p := w.Player(puuid)
	skills := obj{}
	for queue, acts := range p.Ranks {
		seasons := obj{}
		for act, info := range acts {
			var wins any
			if info.WinsByTier != nil {
				wins = info.WinsByTier
			}
			seasons[act] = obj{
				"SeasonID": act, "NumberOfWins": info.Wins, "NumberOfWinsWithPlacements": info.WinsWithPlacements,
				"NumberOfGames": info.Games, "Rank": 0, "CapstoneWins": 0, "LeaderboardRank": 0,
				"CompetitiveTier": info.Tier, "RankedRating": info.RR, "WinsByTier": wins,
				"GamesNeededForRating": 0, "TotalWinsNeededForRank": 0,
			}
		}
		skills[queue] = obj{
			"TotalGamesNeededForRating": 0, "TotalGamesNeededForLeaderboard": 0,
			"CurrentSeasonGamesNeededForRating": 0, "SeasonalInfoBySeasonID": seasons,
		}
	}
	return mustJSON(obj{
		"Version": 1, "Subject": puuid, "NewPlayerExperienceFinished": true,
		"QueueSkills": skills,
		"LatestCompetitiveUpdate": obj{
			"MatchID": "", "MapID": "", "SeasonID": "", "MatchStartTime": 0,
			"TierAfterUpdate": 0, "TierBeforeUpdate": 0, "RankedRatingAfterUpdate": 0,
			"RankedRatingBeforeUpdate": 0, "RankedRatingEarned": 0, "RankedRatingPerformanceBonus": 0,
			"CompetitiveMovement": "MOVEMENT_UNKNOWN", "AFKPenalty": 0,
		},
		"IsLeaderboardAnonymized": false, "IsActRankBadgeHidden": false,
	})
}

func (w *World) competitiveUpdates(puuid string, start, end int) []byte {
	p := w.Player(puuid)
	list := []obj{}
	for i, u := range p.Updates {
		if i < start || i >= end {
			continue
		}
		list = append(list, obj{
			"MatchID": u.MatchID, "MapID": MapAscent, "SeasonID": ActCurrent,
			"MatchStartTime": millis(u.Start), "TierAfterUpdate": 0, "TierBeforeUpdate": 0,
			"RankedRatingAfterUpdate": 0, "RankedRatingBeforeUpdate": 0,
			"RankedRatingEarned": u.Earned, "RankedRatingPerformanceBonus": 0,
			"CompetitiveMovement": "MOVEMENT_UNKNOWN", "AFKPenalty": 0,
		})
	}
	return mustJSON(obj{"Version": 1, "Subject": puuid, "Matches": list})
}

func (w *World) history(puuid, queue string) []*Match {
	var out []*Match
	for _, m := range w.Matches {
		if queue != "" && m.Queue != queue {
			continue
		}
		for _, mp := range m.Players {
			if mp.PUUID == puuid {
				out = append(out, m)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Start.Equal(out[j].Start) {
			return out[i].Start.After(out[j].Start)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (w *World) matchHistory(puuid, queue string, start, end int) []byte {
	all := w.history(puuid, queue)
	list := []obj{}
	for i, m := range all {
		if i < start || i >= end {
			continue
		}
		list = append(list, obj{"MatchID": m.ID, "GameStartTime": millis(m.Start), "QueueID": m.Queue})
	}
	return mustJSON(obj{"Subject": puuid, "BeginIndex": start, "EndIndex": end, "Total": len(all), "History": list})
}

func location(x, y int) obj { return obj{"x": x, "y": y} }

func (w *World) matchDetails(m *Match) []byte {
	timeField := m.KillTimeField
	if timeField == "" {
		timeField = "roundTime"
	}
	players := []obj{}
	for _, mp := range m.Players {
		p := w.Player(mp.PUUID)
		name, tag := p.Name, p.Tag
		if mp.Name != "" {
			name, tag = mp.Name, mp.Tag
		}
		players = append(players, obj{
			"subject": mp.PUUID, "gameName": name, "tagLine": tag,
			"platformInfo": obj{"platformType": "PC", "platformOS": "Windows", "platformOSVersion": "10.0.19042.1.256.64bit", "platformChipset": "Unknown"},
			"teamId":       mp.Team, "partyId": "00000000-0000-0000-0000-000000000000", "characterId": mp.Agent,
			"stats": obj{
				"score": mp.Score, "roundsPlayed": mp.Rounds, "kills": mp.Kills, "deaths": mp.Deaths,
				"assists": 0, "playtimeMillis": 1800000,
				"abilityCasts": obj{"grenadeCasts": 0, "ability1Casts": 0, "ability2Casts": 0, "ultimateCasts": 0},
			},
			"roundDamage": nil, "competitiveTier": 0, "isObserver": false,
			"playerCard": "9fb348bc-41a0-91ad-8a3e-818035c4e561", "playerTitle": "d13e579c-435e-44d4-cec2-6eae5a3c5ed4",
			"preferredLevelBorder": "", "accountLevel": 120, "sessionPlaytimeMinutes": 60,
			"behaviorFactors": obj{"afkRounds": 0, "collisions": 0, "commsRatingRecovery": 0, "damageParticipationOutgoing": 0, "friendlyFireIncoming": 0, "friendlyFireOutgoing": 0, "mouseMovement": 0, "stayedInSpawnRounds": 0},
		})
	}
	teams := []obj{}
	for _, t := range m.Teams {
		teams = append(teams, obj{"teamId": t.ID, "won": t.Won, "roundsPlayed": len(m.Rounds), "roundsWon": t.RoundsWon, "numPoints": t.RoundsWon})
	}
	rounds := []obj{}
	allKills := []obj{}
	for i, r := range m.Rounds {
		stats := map[string]obj{}
		order := []string{}
		statFor := func(puuid string) obj {
			if s, ok := stats[puuid]; ok {
				return s
			}
			s := obj{
				"subject": puuid, "kills": []obj{}, "damage": []obj{}, "score": 0,
				"economy": obj{"loadoutValue": 3900, "weapon": "", "armor": "", "remaining": 100, "spent": 3900},
				"ability": obj{"grenadeEffects": nil, "ability1Effects": nil, "ability2Effects": nil, "ultimateEffects": nil},
				"wasAfk":  false, "wasPenalized": false, "stayedInSpawn": false,
			}
			stats[puuid] = s
			order = append(order, puuid)
			return s
		}
		for _, mp := range m.Players {
			statFor(mp.PUUID)
		}
		locations := []obj{}
		if m.Heavy {
			for j, mp := range m.Players {
				locations = append(locations, obj{"subject": mp.PUUID, "viewRadians": 1.5707963 + float64(j)/10, "location": location(1000+j*37, -2000+j*53)})
			}
		}
		for _, k := range r.Kills {
			kill := obj{
				"gameTime": i*100000 + k.At, timeField: k.At,
				"killer": k.Killer, "victim": k.Victim, "victimLocation": location(100, 200),
				"assistants": []string{}, "playerLocations": locations,
				"finishingDamage": obj{"damageType": "Weapon", "damageItem": "9c82e19d-4575-0200-1a81-3eacf00cf872", "isSecondaryFireMode": false},
			}
			s := statFor(k.Killer)
			s["kills"] = append(s["kills"].([]obj), kill)
			withRound := obj{"round": i}
			for key, v := range kill {
				withRound[key] = v
			}
			allKills = append(allKills, withRound)
		}
		for _, h := range r.Damage {
			s := statFor(h.From)
			s["damage"] = append(s["damage"].([]obj), obj{
				"receiver": h.To, "damage": h.Damage, "legshots": h.Leg, "bodyshots": h.Body, "headshots": h.Head,
			})
		}
		playerStats := []obj{}
		for _, puuid := range order {
			playerStats = append(playerStats, stats[puuid])
		}
		var economies, scores any
		if m.Heavy {
			var e, sc []obj
			for _, mp := range m.Players {
				e = append(e, obj{"subject": mp.PUUID, "loadoutValue": 3900, "weapon": "9c82e19d-4575-0200-1a81-3eacf00cf872", "armor": "822bcab2-40a2-324e-c137-e09195ad7692", "remaining": 1200, "spent": 3900})
				sc = append(sc, obj{"subject": mp.PUUID, "score": 210})
			}
			economies, scores = e, sc
		}
		rounds = append(rounds, obj{
			"roundNum": i, "roundResult": "Eliminated", "roundCeremony": "CeremonyDefault",
			"winningTeam": "Blue", "plantRoundTime": 0, "plantPlayerLocations": locations,
			"plantLocation": location(0, 0), "plantSite": "", "defuseRoundTime": 0,
			"defusePlayerLocations": nil, "defuseLocation": location(0, 0),
			"playerStats": playerStats, "roundResultCode": "Elimination",
			"playerEconomies": economies, "playerScores": scores,
		})
	}
	return mustJSON(obj{
		"matchInfo": obj{
			"matchId": m.ID, "mapId": m.Map, "gamePodId": "aresriot.aws-euc1-prod.eu-gp-frankfurt-1",
			"gameLoopZone": "frankfurt", "gameServerAddress": "", "gameVersion": DefaultClientVersion,
			"gameLengthMillis": 1800000, "gameStartMillis": millis(m.Start), "provisioningFlowID": "Matchmaking",
			"isCompleted": true, "customGameName": "", "forcePostProcessing": false, "queueID": m.Queue,
			"gameMode": m.Mode, "isRanked": m.Queue == "competitive", "isMatchSampled": false,
			"seasonId": ActCurrent, "completionState": "Completed", "platformType": "PC",
			"premierMatchInfo": obj{}, "partyRRPenalties": obj{}, "shouldMatchDisablePenalties": false,
		},
		"players": players, "bots": []any{}, "coaches": []any{}, "teams": teams,
		"roundResults": rounds, "kills": allKills,
	})
}

func (w *World) content() []byte {
	seasons := []obj{}
	for _, s := range w.Seasons {
		seasons = append(seasons, obj{
			"ID": s.ID, "Name": s.Name, "Type": s.Type,
			"StartTime": s.Start.UTC().Format(time.RFC3339), "EndTime": s.End.UTC().Format(time.RFC3339),
			"IsActive": s.Active,
		})
	}
	return mustJSON(obj{"DisabledIDs": []any{}, "Seasons": seasons, "Events": []any{}})
}

func (w *World) pregamePlayer(puuid string) ([]byte, bool) {
	if w.Pregame == nil || !w.inGame(w.Pregame, puuid) {
		return nil, false
	}
	return mustJSON(obj{"Subject": puuid, "MatchID": w.Pregame.ID, "Version": 1}), true
}

func (w *World) coreGamePlayer(puuid string) ([]byte, bool) {
	if w.CoreGame == nil || !w.inGame(w.CoreGame, puuid) {
		return nil, false
	}
	return mustJSON(obj{"Subject": puuid, "MatchID": w.CoreGame.ID, "Version": 1}), true
}

func (w *World) inGame(g *LiveGame, puuid string) bool {
	for _, p := range g.Players {
		if p.PUUID == puuid {
			return true
		}
	}
	return false
}

func (w *World) selfTeam(g *LiveGame) string {
	for _, p := range g.Players {
		if p.PUUID == w.Self {
			return p.Team
		}
	}
	return ""
}

func (w *World) pregameMatch(g *LiveGame) []byte {
	team := w.selfTeam(g)
	players := []obj{}
	for _, p := range g.Players {
		if p.Team != team {
			continue
		}
		players = append(players, obj{
			"Subject": p.PUUID, "CharacterID": p.Agent, "CharacterSelectionState": p.State,
			"PregamePlayerState": "joined", "CompetitiveTier": 0,
			"PlayerIdentity": identity(p.PUUID, w.incognito(p.PUUID)), "SeasonalBadgeInfo": badge(),
			"IsCaptain": false,
		})
	}
	ally := obj{"TeamID": team, "Players": players}
	return mustJSON(obj{
		"ID": g.ID, "Version": 1, "Teams": []obj{ally}, "AllyTeam": ally, "EnemyTeam": nil,
		"ObserverSubjects": []any{}, "MatchCoaches": []any{}, "EnemyTeamSize": 5, "EnemyTeamLockCount": 0,
		"PregameState": "character_select_active", "LastUpdated": "2026-10-25T18:00:00Z",
		"MapID": g.Map, "MapSelectPool": []any{}, "BannedMapIDs": []any{}, "CastedVotes": obj{},
		"MapSelectSteps": []any{}, "MapSelectStep": 0, "Team1": team,
		"GamePodID": "aresriot.aws-euc1-prod.eu-gp-frankfurt-1", "Mode": g.Mode, "VoiceSessionID": "",
		"MUCName": "", "TeamMatchToken": "", "QueueID": g.Queue, "ProvisioningFlowID": "Matchmaking",
		"IsRanked": g.Ranked, "PhaseTimeRemainingNS": 0, "StepTimeRemainingNS": 0,
		"altModesFlagADA": false, "TournamentMetadata": nil, "RosterMetadata": nil,
	})
}

func (w *World) coreGameMatch(g *LiveGame) []byte {
	players := []obj{}
	for _, p := range g.Players {
		players = append(players, obj{
			"Subject": p.PUUID, "TeamID": p.Team, "CharacterID": p.Agent,
			"PlayerIdentity": identity(p.PUUID, w.incognito(p.PUUID)), "SeasonalBadgeInfo": badge(),
			"IsCoach": false, "IsAssociated": true,
		})
	}
	var mm any
	if g.Queue != "" {
		mm = obj{"QueueID": g.Queue, "IsRanked": g.Ranked}
	}
	return mustJSON(obj{
		"MatchID": g.ID, "Version": 1, "State": "IN_PROGRESS", "MapID": g.Map, "ModeID": g.Mode,
		"ProvisioningFlow": "Matchmaking", "GamePodID": "aresriot.aws-euc1-prod.eu-gp-frankfurt-1",
		"AllMUCName": "", "TeamMUCName": "", "TeamVoiceID": "", "TeamMatchToken": "", "IsReconnectable": true,
		"ConnectionDetails": obj{
			"GameServerHosts": []string{"127.0.0.1"}, "GameServerHost": "127.0.0.1", "GameServerPort": 7000,
			"GameServerObfuscatedIP": 0, "GameClientHash": 0, "PlayerKey": "",
		},
		"PostGameDetails": nil, "Players": players, "MatchmakingData": mm,
	})
}

// loadout sockets - a gun's skin sits beside its chroma and the rest
const (
	skinSocket   = "bcef87d6-209b-46c6-8b19-fbe40bd95abc"
	chromaSocket = "3ad1b2b2-acdb-4524-852f-954a76ddae0a"
	someChroma   = "c4c40000-0000-4000-8000-000000000001"
)

func (w *World) coreGameLoadouts(g *LiveGame) []byte {
	loadouts := []obj{}
	for _, p := range g.Players {
		items := obj{}
		for gun, standard := range StandardSkins {
			skin := standard
			if s, ok := p.Skins[gun]; ok {
				skin = s
			}
			items[gun] = obj{"ID": gun, "Sockets": obj{
				skinSocket:   obj{"ID": skinSocket, "Item": obj{"ID": skin, "TypeID": skinSocket}},
				chromaSocket: obj{"ID": chromaSocket, "Item": obj{"ID": someChroma, "TypeID": chromaSocket}},
			}}
		}
		loadouts = append(loadouts, obj{"CharacterID": p.Agent, "Loadout": obj{
			"Subject": p.PUUID, "Sprays": obj{"SpraySelections": []any{}}, "Expressions": obj{"AESSelections": []any{}},
			"Items": items,
		}})
	}
	return mustJSON(obj{"Loadouts": loadouts})
}

func (w *World) flow() string {
	if w.Flow == "" {
		return "Matchmaking"
	}
	return w.Flow
}

func (w *World) partyPlayer(puuid string) []byte {
	return mustJSON(obj{
		"Subject": puuid, "Version": 1, "CurrentPartyID": w.PartyID, "Invites": nil, "Requests": []any{},
		"PlatformInfo": obj{"platformType": "PC", "platformOS": "Windows", "platformOSVersion": "10.0.19042.1.256.64bit", "platformChipset": "Unknown"},
	})
}

func (w *World) party() []byte {
	members := []obj{}
	for i, puuid := range w.PartyMembers {
		members = append(members, obj{
			"Subject": puuid, "CompetitiveTier": 0, "PlayerIdentity": identity(puuid, w.incognito(puuid)),
			"SeasonalBadgeInfo": nil, "IsOwner": i == 0, "QueueEligibleRemainingAccountLevels": 0,
			"Pings": []any{}, "IsReady": true, "IsModerator": false, "UseBroadcastHUD": false, "PlatformType": "PC",
		})
	}
	return mustJSON(obj{
		"ID": w.PartyID, "MUCName": "", "VoiceRoomID": "", "Version": 1, "ClientVersion": DefaultClientVersion,
		"Members": members, "State": "DEFAULT", "PreviousState": "DEFAULT", "StateTransitionReason": "",
		"Accessibility": "CLOSED", "MatchmakingData": obj{"QueueID": w.Queue, "PreferredGamePods": []any{}, "SkillDisparityRRPenalty": 0},
		"Invites": nil, "Requests": []any{}, "QueueEntryTime": "0001-01-01T00:00:00Z",
		"RestrictedSeconds": 0, "EligibleQueues": []string{}, "QueueIneligibilities": []string{},
		"XPBonuses": []any{}, "InviteCode": "",
	})
}

func (w *World) staticVersion() []byte {
	return mustJSON(obj{"status": 200, "data": obj{
		"manifestId": "7A64CDDDEC7CDB4C", "branch": "release-13.08", "version": "13.08.00.5715960",
		"buildVersion": "20", "engineVersion": "5.3.2.0", "riotClientVersion": w.StaticVersion,
		"riotClientBuild": "111.0.0.3261.5663", "buildDate": "2026-10-15T22:27:44Z",
	}})
}

func (w *World) staticAgents() []byte {
	list := []obj{}
	for id, name := range w.StaticAgents {
		list = append(list, obj{"uuid": id, "displayName": name, "isPlayableCharacter": true})
	}
	return mustJSON(obj{"status": 200, "data": list})
}

func (w *World) staticMaps() []byte {
	list := []obj{}
	for url, name := range w.StaticMaps {
		list = append(list, obj{"uuid": "00000000-0000-4000-8000-" + fmt.Sprintf("%012d", len(list)), "displayName": name, "mapUrl": url})
	}
	return mustJSON(obj{"status": 200, "data": list})
}

func (w *World) staticSkin(id string) (int, []byte) {
	s, ok := w.StaticSkins[id]
	if !ok {
		return http.StatusNotFound, mustJSON(obj{"status": 404, "error": "the requested uuid was not found"})
	}
	var edition any
	if s.Edition != "" {
		edition = s.Edition
	}
	return http.StatusOK, mustJSON(obj{"status": 200, "data": obj{
		"uuid": id, "displayName": s.Name, "themeUuid": "00000000-0000-4000-8000-000000000000",
		"contentTierUuid": edition, "chromas": []any{}, "levels": []any{},
	}})
}

func errorBody(status int, code, msg string) []byte {
	return mustJSON(obj{"httpStatus": status, "errorCode": code, "message": msg})
}

func splitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}
