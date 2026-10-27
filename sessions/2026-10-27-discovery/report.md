# lowkey data discovery session · 2026-10-27

- Session start: 18:49 local (16:49 UTC), Windows 11 Pro for Workstations 10.0.26300
- lowkey v0.1.2, recording to `lowkey-record-20261027-184914` (started 18:49:14 local)
- Riot Client and Valorant already open and logged in at start. Valorant was started at 16:48 UTC, so this session's game log starts then. Region `eu`, shard `eu`, client `release-13.08-shipping-22-5760300`.
- Requests to Riot's servers: 14 of 15 (8 in phase 1, 3 after match 1, 1 after match 2, 2 for match 3), all `GET`, one at a time, at least 5.5 s apart, all with the user's own PUUID or match ID. Local API requests (127.0.0.1) aren't counted.
- Played: 3 Swift matches, no Competitive. Midway through, the user said the main goal is **whether each player's performance score (and stats like trades) can be had live, during a match**, so the session followed that.
- Session end: about 18:08 UTC. lowkey was stopped at the end (it didn't respond to a window close). Its recording parses cleanly, 654 records.

## What's in this folder

The brief asked for exactly four files. Before the push, the user asked to "provide everything" for analysis with AI, so `extras/` was added:

- `report.md`: this file
- `endpoints.md` (item 01): every endpoint shape from the game log, by tour step and match, plus the messaging pushes
- `local-api-help.json` (item 01): the local API's `/help`, unchanged
- `recordings.zip`: lowkey's recording of this session (`lowkey-record-20261027-184914`, 654 records), with `preSignedUrl` values scrubbed. lowkey had already blanked the lockfile password and the entitlement tokens itself.
- `extras/raw/`: the 14 probe responses, as Riot sent them. **They contain PUUIDs and Riot IDs**: the user's own, and in the match details, gold stars and `core-game`, other players' too. `13-match3-core-game-mid-match.json` had `TeamMatchToken` and `ConnectionDetails.PlayerKey` replaced with `<scrubbed>`.
- `extras/screenshots/`: the 6 screenshots the user's messages saved as files:
  - match 1: the performance score, the breakdown and head-to-head
  - match 2: the end scoreboard
  - match 3: the Tab scoreboard at 3–3 and at 4–3
  - The rest (match 2's performance screens, the 2–2 Tab, match 3's scoreboard and breakdown) arrived inline only and are transcribed in 03. The screenshots show player names.
- `extras/live/`: the presence watcher's timelines for matches 2 and 3, and the match windows used to label `endpoints.md`
- `extras/game-logs.zip`: today's two `ShooterGame.log` files (session 1, 10:43–15:04 UTC; this session, from 16:48 UTC). `Signature`, `Key-Pair-Id`, `Expires` and token query values and JWT-looking strings were replaced with `<scrubbed>`. Player names in the log were left in.
- `extras/scripts/`: everything run here:
  - `probe.ps1`, one counted, rate-limited GET
  - `shape.ps1`, key paths and types
  - `census.ps1` and `build-endpoints.ps1`
  - `watch-live.ps1`, the local presence and round watcher
  - `midmatch-coregame.ps1`
  - `perf_fit.py`, the score fits
  - `check_live.py`, the match 3 live check
  - `death_lines*.py`, the log death markers

Every file was scanned for the lockfile password, JWTs, bearer values and URL signatures before the push. None were left.

## Callouts

New stats services the game itself calls, from `endpoints.md`:

- **Map rating:** `GET pd.eu.a.pvp.net/region-index/v1/players/{id}?seasonID` (`RegionIndex_GetPlayer`). The competitive hub overview fires it once per act for 5 acts, and acts with no data get 404 (`RESOURCE_NOT_FOUND`). The push `ares-region-index/region-index/v1/players/{id}` came 4 times in this morning's log, after competitive games. Probed: 200, see 02.
- **Agent mastery (`bloomline`):**
  - `GET /bloomline/v0/players/{id}`, also with `?longOnly` (`Bloomline_FetchPlayer`, at startup)
  - `GET /bloomline/v0/players/{id}/bonsai/{hex}`, with `?includeModules` on the agent page (`Bloomline_FetchPlayerForBonsai`). `{hex}` is the agent's UUID without dashes.
  - `GET /bloomline/v0/players/{id}/matches/{hex}` (`Bloomline_FetchProcessedMatch`), after a match, in this morning's log only. `{hex}` is the match ID without dashes.
  - `GET /bloomline/v0/config/valid-queues` (`Bloomline_FetchValidQueues`, at startup)

  The player ID appears both as a UUID and as uppercase hex without dashes. All four were probed: 200.
- **Accolades:** no request in either log, only the push `ares-accolades/accolades/v1/players/{id}` (4 times this morning, in the same second as the map rating pushes). Opening Career → accolades fired no request. That page is most likely served by **gold stars**, `GET /goldstars/v1/players/{id}` (`GoldStar_GetPlayer`) and `GET /goldstars/v1/goldstars` (`GoldStar_GetGoldStars`). The client loads both on the competitive hub overview. Probed: 200.
- **Lifetime stats:** no request in either log, only the push `cap/stats/v1/DeltaEvent/{id}` (9 times this morning, one per match, in the same second as the match details push). `cap` isn't on the `pd` host, so its request endpoint is still unknown. Not probed. lowkey's recording has the push's payload after match 1 (service `cap.lifetime-stats`). It's a notice with no values: `{"eventId", "ownerId", "updatedStatGroups": [{"statGroupId", "segments": [{"facets": {"platformgroup": "pc", "agent": {id}, "act": {id}}}]}]}`. So **lifetime stats count Swift** and are faceted by platform group, agent and act. Reading them needs the `cap` stats request, which neither the game's log nor the local `/help` shows.
- **Other pushes after match 1**, from lowkey's recording:
  - `ares-match-details/match-details/v1/matches`: the payload is the bare match ID
  - `ares-contracts/.../matches`: `{"MatchID"}`
  - `ares-mastery/bloomline/v0/players/{id}/matches` and `cap/progression/v1/notifications/cache/invalidate`: `{"version", "updatedGroups": [id]}`
  - `cap/v1/wallets`: `{currency id: {amount, subCurrencies: {GRANTED}}}`
  - `ares-session/v1/sessions/{id}`: `subject`, `cxnState`, `loopState`, `loopStateMetadata`, `playtimeMinutes`, `playtimeNotification`, `connectionTime`, `isRestricted`, `restrictionType`, `clientPlatformInfo` and more
  - the `gdu` REPLAY and SUMMARY payloads (presigned URLs) weren't opened
- **Other players:** this morning the game itself called `region-index` (1 URL), `goldstars/v1/players` (1) and `bloomline/v0/players` (2) with other players' IDs. The client fetches other players' map rating, gold stars and mastery, probably from a player card. I didn't probe any of them.
- **Riot computes the performance score for Swift too:** `bloomline/v0/config/valid-queues` lists `PerformanceScoreQueues`: competitive, unrated, premier, swiftplay and their console versions. Match 1 confirmed it: the Swift's match details have `hasTempValues: true` and the full `scores.TempValue*` set for all 10 players (see 03).
- **Map rating and gold stars count competitive only:** both were unchanged 3 minutes after the Swift. Map rating's body was byte-for-byte identical, and gold stars stayed at `version` 66 without the Swift among its last 20 matches. After the Swift, the game refetched gold stars but not map rating, and no map rating or accolades push came.

## Live performance score during a match: what's available

The user's main goal: can lowkey work out each player's current performance score **while the match is running**? Checked during match 1 (Swift, about 16:59–17:10 UTC) without sending a single request:

- **Riot's servers:** the game itself requests nothing about stats mid-match, only `session/v1/sessions/{id}/heartbeat` (POST, about every 40 s), `v1/config/eu` and one `latency/v1/ingest` POST per round. No pushes arrived from Riot's messaging service, apart from one product registry update. Map rating, gold stars, mastery and match details all arrive only after the match (this morning's log: in the same second as the match details push).
- **Local API events:** only `chat_v4_presences`, about once a minute. Presence carries a team's round score (`partyOwnerMatchScoreAllyTeam/EnemyTeam`), not anything per player. The 14 updates during the match were all friends' presences. lowkey's recording got no event for the user's own presence mid-match, but `GET /chat/v4/presences` on the local API returns it.
- **The user's own presence mid-match** (match 2, local `GET /chat/v4/presences`, decoded):
  - `private` (base64 JSON) has `matchPresenceData.{sessionLoopState, matchMap, queueId, provisioningFlow, gameScoreType: "Rounds"}`, `partyOwnerMatchScoreAllyTeam`, `partyOwnerMatchScoreEnemyTeam`, and `partyPresenceData.queueEntryTime`
  - `playerPresenceData.{competitiveTier, accountLevel, leaderboardPosition}`
  - `premierPresenceData.{rosterName, rosterTag, division, score, plating}`
  - `packedData`, a second base64 JSON, has `activity.progress.value` as a string like `"0 - 0"` (`type: CUSTOM`), `activity.mode` (`social_mode_swiftplay`), `activity.location` (`social_location_/Game/Maps/...`) and `player.state` (`INGAME`)
  - **The live round score is available locally.** A watcher polled it every 20 s during match 2 to measure how far it lags the log's round ends.
- **Match 2 live timeline** (the watcher, local only, every 20 s). The presence score changed 3–23 s after each `OnRoundEnded` line (the lag is bounded by the polling interval):

  | Log round end (UTC) | Presence first showed (UTC) | Score (ally – enemy) |
  |---|---|---|
  | round 0, 17:26:04 | 17:26:27 | 0 – 1 |
  | round 1, 17:27:23 | 17:27:27 | 0 – 2 |
  | round 2, 17:28:55 | 17:29:08 | 1 – 2 |
  | round 3, 17:30:17 | 17:30:28 | 2 – 2 |
  | round 4, 17:32:15 | 17:32:28 | 3 – 2 |
  | round 5, 17:33:17 | 17:33:28 | 3 – 3 |
  | round 6, 17:34:37 | 17:34:49 | 4 – 3 |
  | round 7, 17:35:39 | 17:35:49 | 4 – 4 |
  | final round, no log line | 17:37:29 straight to `MENUS`, score `0 – 0` | (the 5 – 4 win never showed in presence) |

  The final round shows in neither source. lowkey can take the end of the match from `sessionLoopState` leaving `INGAME`, or from the match details push.
- **The game log:**
  - `LogShooterGameState: AShooterGameState::OnRoundEnded for round 'N'` with a timestamp, once per round, so round boundaries are live.
  - `LogClientPerRoundTelemetryComponent: End of Round Perf statistics` is frame times only.
  - Nothing about kills, deaths, damage, assists, ability casts, plants or defuses. `LogAbilitySystem` lines name buffs on agent classes, which isn't a usable stat source.
  - `LogShooterPlayerState: [ReconnectSpectate]` lines carry players' in-game names. They're kept out of everything here.
- **Conclusion:** none of the score's inputs exist anywhere outside the game process during the match. Those inputs are damage, kill and death impact, trades, assists, ability casts, plants and defuses per round. The only other ways to get them are reading the game's memory or its network traffic. Both are what Vanguard bans, and both are off limits in this brief.
- **What is possible:**
  - Before and during the match: each player's **expected** score, from the `TempValueF` averages in the recent match details lowkey already downloads (13 per player). This works for Swift, unrated and Premier as well as competitive, since all of them carry the fields. Map rating's per-map `tempValueF` adds to that for competitive. No new requests.
  - After the match: the exact score and grade for all 10 players, as soon as match details are out. That came 9–10 s after the final round ended in matches 1 and 2. The log's flow line marks the end, see below.
  - During the match: only the round count and team score are live, so lowkey can show round progress next to the expected score. The log has no `OnRoundEnded` line for the final round, so the match's end has to come from the game flow or presence.
- Match details hold everything needed to replay the score round by round after the match: `roundResults[].playerStats[]` with `kills[]`, `damage[]` (per receiver, with head, body and leg shots), `score`, `economy` and `ability`, plus `roundResults[].playerScores[]`.

### What the game shows live, and how far that gets

- **The in-game Tab scoreboard** (the user's screenshot from match 3, at 2–2) has the columns `NAME`, `ULTIMATE`, `KDA`, `LOADOUT`, `CREDS` and `PING`, plus the team round score (`ATK 2`, `DEF 2`) with an icon per round. There's no performance score, damage, trades, first bloods, plants or defuses. **So the game itself doesn't show the score during a match.** It shows K/D/A for all 10 players.
- **The real end of a match is in the log:** `LogGameFlowStateManager: Reconcile called with state: InGame. Scheduling a reconcile in 4.000000 seconds.` appears when the final round ends, then `Transitioning from InGame to TransitionToMainMenu` 6–7 s later. The match details push follows 9–10 s after the first line (match 1: 17:12:50 → 17:12:59; match 2: 17:37:13 → 17:37:23). **`gameStartMillis + gameLengthMillis` doesn't mark the end**: for match 2 it gives 17:36:07, but the log shows the final round started at 17:36:24.
- Each round logs `Gameplay started` twice, once with the round's `OnRoundEnded` and again about 30–45 s later.
- **Fits against Riot's score**: 341 competitive match details from this morning's recordings, 2,556 players with a score, ordinary least squares. The script is `perf_fit.py`, not sent; it's described here.

  | Inputs | R² | Mean abs. error | Within ±25 | Within ±50 | Grade right |
  |---|---|---|---|---|---|
  | the 8 breakdown values `scores.TempValueA I J B C D E K` | 0.985 | 7.5 | 99% | 100% | 97% |
  | final kills, deaths and assists per round (all visible on Tab) | 0.907 | 21.2 | 67% | 94% | 91% |
  | K/D/A after the first 3 rounds, predicting the final score | −2.8 | 138 | 11% | 21% | 61% |
  | after 6 rounds | −0.74 | 92 | 19% | 34% | 68% |
  | after 9 rounds | 0.04 | 68 | 24% | 45% | 73% |
  | after 12 rounds | 0.41 | 54 | 30% | 54% | 76% |

  - The K/D/A fit is `F ≈ 51.4 + 310.8 × kills/round − 78.3 × deaths/round + 148.1 × assists/round`.
  - The breakdown fit is `F ≈ 23.2 + 1.13 A + 120.0 I + 132.6 J + 164.4 B + 94.6 C + 12.0 D + 16.1 E + 47.4 K`. It's close to linear, and the rest is probably the 0–500 clamps on the combat and support halves (`TempValueG` hit 500 in match 1).
  - Per-round K/D/A from `roundResults[].playerStats[].kills[]` matched the match totals for 3,206 players and differed for 278.
- **What this means:**
  - **Predicting the final score** mid-match is weak, because the rounds still to come decide most of it.
  - **A "current" score**, as if the match ended now, from the K/D/A so far, is about as good as the K/D/A fit: ±21 and 91% grade accuracy. Riot never computes a mid-match score, so this can't be checked against one.
  - **The real obstacle is the data source.** K/D/A is on the Tab scoreboard but in no API, push or log line. Getting it live would mean reading the screen. That's outside this brief (no game process access, no overlays) and is the user's call.
- **Match 3: the live test.** The user pasted 3 Tab screenshots during the match (2–2, 3–3, and 4–3 mid-round 8). Each was turned into live estimates with the K/D/A fit in chat, then checked against match details (request 14) and the end screens:
  - **The Tab K/D/A equals the per-round data:** after 4 and 6 rounds, the user's 4/3/2 and 5/5/3 match `roundResults[].playerStats[].kills[]` exactly, and so do all 10 players' K/D/A after 6 rounds.
  - **The user's live estimate trail** (per-round data, K/D/A fit):

    | After round | K/D/A | Estimate |
    |---|---|---|
    | 1 | 1/1/1 | 432 |
    | 2 | 4/1/2 | 782 |
    | 3 | 4/2/2 | 512 |
    | 4 | 4/3/2 | 377 |
    | 5 | 4/4/2 | 297 |
    | 6 | 5/5/3 | 319 |
    | 7 | 9/5/3 | 458 |
    | 8 (final) | 12/5/3 | 524 |

    Riot's final score was **429.5, distinction (Masterful)**. The estimate at round 7 (458) had the right grade. The chat answer during round 8 said 408, because it counted the round in progress, but the 4 kills were round 7's.
  - **Final K/D/A fit for all 10 players:** grade right 9/10, mean absolute error 36. Errors ranged from +95 (the user, 12/5/3) to −62. The one wrong grade was a merit 346 estimated at 327.
  - **What to fix before relying on it:**
    - Clamp to 0–500. Riot's combat half caps at 500, and the straight line overshoots strong frag games: +95 for 1.5 kills a round.
    - Pull early-match estimates toward the player's usual score (for example, weight `rounds / (rounds + k)`), since 2 rounds produced 782.
    - Count only completed rounds.
    - The fuller fits (damage, trades) would do better, but Tab doesn't show those.
- **Deaths in the game log, tested against match 3** (`death_lines2.py` and `death_lines3.py`, not sent):
  - The log-to-match clock offset was fixed from round starts. Each round's start (`kills[].gameTime − roundTime`) lines up with a `Gameplay started` line at +39.0 s for all 8 rounds.
  - At that offset, 55 kills were tested against every `LogAbilitySystem: InternalOnActiveGameplayEffectAdded <class> <effect>` line:

    | Effect | Lines | Lines within 1 s of a kill | Kills covered (ally / enemy victims) | What it is |
    |---|---|---|---|---|
    | `Debuff_Disabled_OwnerDead_C` | 37 | 100%, about +0.8 s after | 14 of 55 (3/25, 11/30) | lands on a dead player's **deployables** (Cypher tripwires, cage and camera; Chamber trap and teleporter; another agent's trap and teleporter), so it marks the death of a player with something placed and names the owner by the object's class |
    | `Debuff_GrievousWounds_C` | 67 | 60% | 33 of 55 (15/25, 18/30) | on agent pawns. 40% of lines aren't near a kill, and the pawn is often not the victim, so it looks like a damage or heal effect, not a death marker |
    | `Buff_Smonk_PostDeathAttributes_C` | 5 | 100% | 5 (Clove deaths only) | on `Smonk_PostDeath_PC`: Clove's post-death state |

  - **Not one of them names the killer.** A trade needs killer, victim and time for every kill, so **live trades can't be had from the log**. The log only offers partial death times. These are also debug warnings that a patch can silence.
  - Agent class names seen: `Vampire` Reyna, `Wushu` Jett, `Smonk` Clove, `Hunter` Sova, `Deadeye` Chamber, `Gumshoe` Cypher, `Phoenix`. `Iris` and `Pine` are two more agents in the lobby, not identified.
- How F relates to its combat and support halves is still open. In match 1, G 500 and H 248.8 gave F 450.5. In match 2, G 181.0 and H 181.9 gave F 183.8, which is above both, so F isn't a plain weighted average of G and H.

## 01 · Endpoint census

Status: partial. Premier and Leaderboards were never opened (Esports hub instead), so neither has rows of its own. The census ran again after all three matches and found no new stats endpoints.

- `GET /help` on the local API answered 200, 285 KB, saved as `local-api-help.json` unchanged. Checked: no UUIDs, no JWTs, no Riot ID, no signatures.
- `/help` lists only the Riot Client's own API (patching, chat, auth, voice, PlayStation and Xbox linking and so on). It holds **no Valorant stats, match or mastery endpoints**.
- `/help` has keys that differ only in case (`PlayerAccountVngV1Profile` and `PlayerAccountvngV1Profile`), so Windows PowerShell 5.1's `ConvertFrom-Json` refuses it. Python's `json` reads it fine.
- The log has four useful line types, all in `endpoints.md`:
  - `LogPlatformCommon: Platform HTTP Query End.`, one line per finished request, with query name, method, URL and response code
  - `LogReplayData: Replay Data Service: ... Created <METHOD> query <NAME>`, created only, with no response line
  - `LogRMSService: Received RMS update. URI:`, the messaging pushes
  - `LogUINavigationModel: Warning: Current Url: main/hub/...`, the screen on show. This line dates every tour step to the second.
- The brief's census filter (`ShooterGame*.log` written today) also catches this morning's log, `ShooterGame-backup-2026.10.27-15.04.45.log`, 10:43–15:04 UTC. `endpoints.md` keeps it separate: this session's 72 shapes with their tour step, then 18 shapes seen only this morning, then 23 push paths.
- Tour, from the UI log (UTC): asked at 16:50:20.
  - 16:51:07 Career profile with Match History
  - 16:51:29 competitive hub overview
  - 16:51:59 prestige
  - 16:52:06 map rating (`compethubmaprating`)
  - 16:52:24 accolades (`compethubaccolades`)
  - 16:52:28 Agents, then 16:52:40 agent details with `bloomlineplank`, `bloomlinetrack` and `bloomlineshortlevel`
  - 16:52:53 Esports hub
  - 16:52:55 lobby
  - 16:53:20 Collections
  - The user said they were done at 16:54:07.
- The map rating and accolades pages fired nothing of their own. Everything they show came from the overview's calls at 16:51:30–16:51:53: `goldstars`, `mmr?backfill`, and `region-index` for 5 acts.
- The game's own calls include several on the never-use list: `name-service` (POST v3, PUT v2), store, wallet, contracts, restrictions, party, loadout `PUT`, and agent select and lock (this morning). I didn't send any of them.
- Replay data service: `euc1.pp.sgp.pvp.net/match-history-query/v3/products/valorant/players/{id}/infoTypes/SUMMARY` and `/REPLAY`, with one `id` key per match (7 or 9 at a time), plus presigned CloudFront `summary.json` (GET) and `.vrf` (HEAD) files. These are replay territory, so I skipped them. Their query keys are `Expires&Signature&Key-Pair-Id`.

## 02 · New endpoint shapes

Status: done for map rating, mastery and gold stars. Skipped for lifetime stats and accolades, because the game never requested either, so there's no known endpoint to probe (only push paths).

All requests went to `pd.eu.a.pvp.net` with the user's own PUUID, using the headers from the brief.

### `GET /region-index/v1/players/{self}?seasonID={act}`: map rating

- Status: 404 `RESOURCE_NOT_FOUND` for the act the game also got 404 for (request 1), and 200, 12.9 KB, for the act the game got 200 for (request 8)
- Shape:
  ```
  subject: uuid
  platformGroup: string            "pc"
  seasonId: uuid
  regionData: {map id}: {          7 maps
    regionId: uuid
    values: {
      winRate, attackWinRate, defenseWinRate: number (fraction 0-1)
      tempValueF: number (fractional)
      regionIndex: number (fractional)
      totalGames: number (integer)
      computedFromValidSeasons: bool  (true for all 7)
    }
    deltaValues: same keys as values
    bestMatch: { matchId, agentId: uuid; actIndexBefore, actIndexAfter, tempValueF, attackWinRate, defenseWinRate: number; outcome: string }
    matchHistory: [] of the same keys as bestMatch   length 2-5 per map, 24 matches across the 7 maps
    topAgents: [] { agentId: uuid }                  length 1-3, 5 agents across all maps
    version: number
  }
  ```
- Stat names: `winRate`, `attackWinRate`, `defenseWinRate`, `tempValueF`, `regionIndex`, `totalGames`, `computedFromValidSeasons`
- Facets: map (7, keyed by map UUID, which the Mac can match to content), agent (`topAgents`, 5 in all) and act (`seasonID` in the query)
- Enums: `outcome` is `Win`, `Lose` or `Draw`, and `platformGroup` is `pc`
- `actIndexBefore` and `actIndexAfter` are fractional and chain from match to match within a map: one match's `Before` is the previous match's `After`. They look like a running per-map rating that each match moves. `tempValueF` shares its name with the hidden performance score in competitive match details (`scores.TempValueF`).
- Mode coverage is unknown so far. None of the 24 match IDs are in this morning's lowkey recordings, so I couldn't look up their queues locally. `computedFromValidSeasons` hints that it spans several acts. The pushes after the Swift and Comp matches will show which modes trigger an update.

### `GET /bloomline/v0/players/{self}`: mastery for every agent

- Status 200, 5.0 KB
- Shape:
  ```
  Subject: uuid
  Bonsais: []                       length 29 (one per agent)
    BonsaiID: uuid                  the agent's UUID
    Flourish: { ShortLevel, LongLevel, Points, PointsVersion, PreviousShortLevel: number; FlourishID: uuid }
    TrackProgressList: []           length 1 for every agent
      TrackID: uuid
      TrackLevel: number
  LongGroupVersion: number
  ```
- Stat names: `ShortLevel`, `LongLevel`, `PreviousShortLevel`, `Points`, `TrackLevel`
- Facets: agent (29)

### `GET /bloomline/v0/players/{selfhex}/bonsai/{agent hex}?includeModules=true`: one agent's mastery

- Status 200, 1.1 KB. This is the exact URL the agent page used in the tour.
- Shape: same as one `Bonsais[]` item above, plus:
  ```
  NumberModules: []                 length 9
    ModuleID: uuid
    NumberID: uuid
    Value: number
  ```
- `NumberModules` look like per-agent counters: 9 numbers, each named by a UUID that probably resolves through content. They're the closest thing to per-agent lifetime stats found so far.

### `GET /bloomline/v0/players/{self}/matches/{match hex}`: mastery earned in one match

- Status 200, 565 B. Used a match ID the game itself fetched this morning for the user, which is also in the map rating's match history.
- Shape:
  ```
  MatchID: uuid
  CreatedAt: string (ISO time)
  ShortID: uuid
  PlayedCharacter: uuid
  Delta: {
    BonsaiID: uuid
    PointsBefore, PointsAfter: number
    ShortLevelBefore, ShortLevelAfter, LongLevelBefore, LongLevelAfter: number
    FlourishIDBefore: uuid
    EarningsBreakdown: { SecondsPlayed, TimePlayedMP, WinBonusPercent, WinBonusMP, TotalMPGranted: number }
  }
  ```
- Mastery points (MP) come from time played plus a win bonus. Nothing here is a performance grade.

### `GET /bloomline/v0/config/valid-queues`

- Status 200, 315 B
- `ProcessedQueues` (11): spikerush, competitive, unrated, swiftplay, premier, hurm, console_spikerush, console_competitive, console_unrated, console_swiftplay, console_hurm
- `PerformanceScoreQueues` (7): competitive, unrated, premier, swiftplay, console_competitive, console_unrated, console_swiftplay

### `GET /goldstars/v1/goldstars`: the gold star definitions

- Status 200, 691 B
- Shape: `[]` of `{ id: uuid, tempT: number }`, length 12. `tempT` is fractional, with 9 distinct values, so it's probably a threshold.

### `GET /goldstars/v1/players/{self}`: the user's gold stars (most likely what the accolades page shows)

- Status 200, 56 KB
- Shape:
  ```
  puuid: uuid
  version: number
  tempS: {act id}: {                3 acts, one of them the current one
    tempS: uuid                     equals its key
    tempA: {gold star id}: { id: uuid, tempC: number, tempB: number }   11-12 per act, fractional
  }
  tempM: []                         length 20: the user's last 20 matches
    id: uuid                        match ID (19 of the 20 are in the map rating's match history)
    tempG: number                   epoch milliseconds, the match time
    tempP: {puuid}: {               10 per match: every player in the match
      tempA: {gold star id}: { id: uuid, tempV: number, tempB: bool }   0-8 per player
    }
  ```
- All 12 gold star IDs in the player data are the ones `/goldstars/v1/goldstars` defines.
- **Privacy:** this own-data response carries all 10 players' PUUIDs for each of the 20 matches (200 entries), with their gold star values.
- `tempB` isn't simply `tempV >= tempT` (that matched 101 of 418 entries), so what it means is still open.

### After match 1 (Swift)

- Request 9: `GET /match-details/v1/matches/{the Swift's match ID}`, 200, 243 KB. The ID was checked against this session's `core-game` match before sending. lowkey hadn't fetched it, see 03.
- Request 10: `region-index` for the same act, 200, byte-for-byte identical to request 8. Total games still 24, and the Swift isn't in any map's `matchHistory`.
- Request 11: `goldstars/v1/players/{self}`, 200, `version` still 66, and the Swift isn't in `tempM`, though the game itself refetched this endpoint 2 s after the match details push.

### Not probed

- Lifetime stats (`cap/stats`) and accolades (`ares-accolades`): only push paths are known. The game made no request to either, and guessing a host is outside the brief.
- `mmr/v1/players/{id}?backfill=true` (`MMR_FetchPlayer_WithPrestigeBackfill`): lowkey already calls `mmr/v1/players`, and this only adds a query flag.
- `account-xp`, `premier`, `daily-ticket`, `favorites`, `esports-service`: not player stats.
- `contracts/v1/players/{id}/matches/{id}`: contracts are on the never-use list.
- `match-history-query` on `pp.sgp.pvp.net` and the CloudFront files: replay data service.

## 03 · What the client shows

Status: partial. All three matches were Swift, not the Competitive the item asks for. The performance screens are fully covered from the user's screenshots, matched value by value against match details in all three matches. Still open:
- the client word for Merit, which never came up
- what the mastery progress screen shows
- whether agent select showed mastery levels for anyone
- anything about a map rating on the end screens. None was reported, and map rating doesn't count Swift.

### Match 1: Swift (`swiftplay`), ended about 17:12:50 UTC

- Agent played: Reyna, from `characterId`. Won 5–4 in 9 rounds, 14.2 minutes, `completionState` `Completed`.
- Timeline (UTC):
  - 16:57:54 agent select
  - 16:59 match start
  - `OnRoundEnded` for rounds 0–7 in the log, the last at 17:11:22. There was no line for the final round.
  - 17:12:08 the final round starts (`Gameplay started`). 17:12:09 is `gameStartMillis + gameLengthMillis`, which doesn't mark the end.
  - 17:12:50 the final round ends (`Reconcile called with state: InGame. Scheduling...`), then 17:12:56 `Transitioning from InGame to TransitionToMainMenu`
  - 17:12:58–59 the end-of-match screen opens. In the same second, the match details push and 16 other pushes arrive, and the game fetches match details, mastery, contracts and wallet.
  - 17:13:01 the game fetches match details again by hex ID, plus gold stars
  - 17:13:16 lowkey fetches `competitiveupdates`, and nothing else
- End-of-match screens the user went through, from the UI log:
  - 17:13:00 `eogmatchmvp`
  - 17:13:08 `eogmatchprogress`
  - 17:13:13 `eogmatchsummary`, `eogmatchscoreboard`
  - 17:13:32 `eogmatchperformance`, a performance screen of its own
- **lowkey didn't download the Swift's match details**: none appear in its recording after the match. Its post-match refresh asks `competitiveupdates` with `queue=competitive` (`internal/app/session.go:49`) and only downloads details for the matches listed there, so it never sees a Swift, unrated or Premier match. To show post-match grades for those queues, lowkey would need the match ID from elsewhere, such as `core-game` or `match-history/v1/history`, and then fetch its details.
- What the match details say, for the Mac to check against the client:
  - `hasTempValues: true`
  - `scores` has `TempValueA`–`L` and `TempValueT` for all 10 players
  - `stats` adds `TempValueA`, `B`, `D` and `E` next to `score`, `kills`, `deaths`, `assists`, `abilityCasts`, `roundsPlayed` and `playtimeMillis`
  - the user's grade (`TempValueL.TempValueO`) is `distinction`, and the other 9 got `pass`
  - the user's combat rating (`M`) is `double_up` and support (`N`) is `neutral`
  - `TempValueP`: damage `double_up`, killImpact `double_up`, deathImpact `double_up`, trades `double_down`
  - `TempValueQ`: assists `double_down`, plants `up`, defuses `double_up`, utilityUsage `neutral`
  - `matchMvp` (a top-level string holding a player ID) is the user, who also had the lobby's highest `TempValueF` and `stats.score`
  - the rating words seen so far: `double_down`, `neutral`, `up`, `double_up`
- **The client's performance screen** (`eogmatchperformance`), from the user's screenshots, quoted word for word. **Every number matches the match details**:

  | Client shows | Match details |
  |---|---|
  | header `PERFORMANCE SCORE`, big number `450` | `scores.TempValueF` 450.47 |
  | `MASTERFUL` under the number | `TempValueL.TempValueO` `distinction` |
  | `DISTRIBUTION`: a bell curve with axis marks `0`, `250` (dashed peak), `330`, `420`, `500`; the bands 330–420 and 420–500 are shaded, and a marker sits at `450` | `TempValueT`: `TempValueV` 0, `TempValueR` 250, `TempValueS` merit 330 / distinction 420, `TempValueU` 500 |
  | `BREAKDOWN`, columns `STATS`, `PER ROUND`, `IMPACT` | |
  | `DAMAGE` `296`, two green up arrows | `scores.TempValueA` 295.89, `TempValueP.damage` `double_up` |
  | `ADJUSTED KILLS` `1.9`, two green up | `scores.TempValueI` 1.895, `killImpact` `double_up` (raw kills per round would be 16/9 = 1.78) |
  | `ADJUSTED DEATHS` `0.51`, two green up | `scores.TempValueJ` −0.514, `deathImpact` `double_up` (raw deaths per round would be 4/9 = 0.44) |
  | `TRADES` `0.11`, two red down | `scores.TempValueB` 0.111, `trades` `double_down` |
  | `ASSISTS` `0.11`, two red down | `scores.TempValueC` 0.111, `TempValueQ.assists` `double_down` |
  | `UTIL. USAGE` `1.9`, a white dash | `scores.TempValueD` 1.889 (17 casts / 9 rounds), `utilityUsage` `neutral` |
  | `PLANTS` `0.11`, one green up | `scores.TempValueE` 0.111, `plants` `up` |
  | `DEFUSES` `0.11`, two green up | `scores.TempValueK` 0.111, `defuses` `double_up` |

- So the client's names for the hidden fields are:
  - `TempValueF` is the performance score
  - `TempValueO` is the grade, with `distinction` shown as **Masterful**. The client words for `merit` and `pass` are still unseen.
  - `TempValueA` damage, `I` adjusted kills, `J` adjusted deaths (shown without the minus sign), `B` trades, `C` assists, `D` util usage, `E` plants, `K` defuses
  - the impact arrows: `double_up` is two up, `up` is one up, `neutral` is a dash, `double_down` is two down
  - "Adjusted" kills and deaths are weighted, not raw counts per round
- The user's other score fields: `TempValueG` (combat) 500, the top of the scale; `TempValueH` (support) 248.8. Lobby scores were 450, 322, 290, 253, 214, 203, 194, 191, 138 and 109, so only the user cleared 330.
- A third screen (the user's screenshot, below the performance section) shows a table of `KILLS`, `DEATHS`, `ASSISTS` `vs.` each of the 5 opponents, with each opponent's portrait and name. The user's rows add up to 16 / 4 / 1, which matches `stats`. One row (6 kills, 1 death) has an orange star. Two rows where the user never died have white stars. What the stars mean is unknown, and the Swift isn't in the gold stars data. Two opponents appear under their agent's name rather than a player name. Their names aren't copied here.
- Not answered yet: whether any screen showed grades for other players, and what the mastery screen showed.

### Match 2: Swift (`swiftplay`) on Icebox (`/Game/Maps/Port/Port`), ended about 17:37:13 UTC

- Agent played: Reyna again. The user's team (Red) won 5–4 in 9 rounds. 5 kills, 8 deaths, 2 assists.
- Timeline (UTC):
  - 17:22:24 agent select
  - 17:23:27 match start
  - rounds 0–7 end in the log at 17:26:04 … 17:35:39, again with no line for the final round
  - 17:36:24 the final round starts. `gameStartMillis + gameLengthMillis` gives 17:36:07, before that.
  - 17:37:13 the final round ends (the flow line), then 17:37:20 the transition to the menu
  - 17:37:21 end-of-match screen, then 17:37:23 match details push, `cap/stats` push and the game's match details fetch
  - 17:37:24 the game fetches gold stars **and `region-index` for the user (200)**, which it didn't do after match 1. No `ares-region-index` push came.
  - end screens: `eogmatchmvp` 17:37:24, `eogmatchprogress` 17:37:31, `eogmatchsummary` and `eogmatchscoreboard` 17:37:36, `eogmatchperformance` 17:37:38
- Request 12: `GET /match-details/v1/matches/{match 2}`, 200, 247 KB, `hasTempValues: true`.
- **The client's performance screen** (user's screenshots) against the match details. **Every value matches again**:

  | Client shows | Match details |
  |---|---|
  | `PERFORMANCE SCORE`, big number `184` in a plain circle, **with no grade word** | `TempValueF` 183.81, grade `pass` |
  | the distribution marker at `184`, left of the `250` peak | |
  | `DAMAGE` `96.6`, red double down | `TempValueA` 96.6, `double_down` |
  | `ADJUSTED KILLS` `0.6`, red single down | `TempValueI` 0.60, `down` |
  | `ADJUSTED DEATHS` `0.89`, red double down | `−TempValueJ` 0.89, `double_down` |
  | `TRADES` `0.11`, red double down | `TempValueB` 0.11, `double_down` |
  | `ASSISTS` `0.22`, red single down | `TempValueC` 0.22, `down` |
  | `UTIL. USAGE` `1.1`, red double down | `TempValueD` 1.11, `double_down` |
  | `PLANTS` `0`, dash | `TempValueE` 0, `neutral` |
  | `DEFUSES` `0`, dash | `TempValueK` 0, `neutral` |

- **The end-of-match scoreboard shows every player's performance score** (the user's screenshot of the summary scoreboard, sorted `INDIVIDUALLY SORTED`). Its columns are `PERFORMANCE SCORE`, `KDA`, `TRADES`, `FIRST BLOODS`, `PLANTS` and `DEFUSES`, one row per player with portrait and name. Names aren't copied here. Some players appear under their agent's name. All 10 rows match the match details:
  - `PERFORMANCE SCORE` is `scores.TempValueF`, rounded (415 … 99)
  - `KDA` is `stats.kills / deaths / assists`
  - `TRADES` is `stats.TempValueB + stats.TempValueC` (trade kills plus deaths that got traded), which equals `scores.TempValueB` × rounds
  - `FIRST BLOODS` is the count of `roundResults[].firstBloodPlayer`, the same as each round's earliest kill
  - `PLANTS` is `stats.TempValueD` and `DEFUSES` is `stats.TempValueE`, with the key missing when it's 0
- Files written during match 2, besides the game log: `Saved\Config\{puuid}-eu\Windows\RiotUserSettings.ini` (settings), `Saved\Telemetry\Persistence.dat` (212 B at match start), and the CEF `webcache_*` folder when the end screen opened. None holds match stats.
- So a **Pass shows no grade word**. Its frame is a plain circle, while a Distinction gets a diamond frame and "MASTERFUL". The client word for Merit is still unseen. All the impact words have now been seen: `double_up`, `up`, `neutral`, `down`, `double_down`.
- Combat `TempValueG` 181.0 (`down`), support `TempValueH` 181.9 (`down`). Lobby scores were 415, 367, 366, 310, 295, 229, 184, 175, 163 and 99, which is 3 merit and 7 pass. The match MVP wasn't the user.
- **A prediction made before the match ended:** "about 280, Pass". It was based on the user's 24 competitive matches in map rating this act (weighted mean `tempValueF` 284, range 156–414, 17 pass and 7 merit). The user has no Icebox entry this act, so it used all maps. **The grade was right, and the number was 96 too high.** The user's 2 Swift scores so far are 450 and 184, so a history average forecasts the grade but not the number.

### Match 3: Swift on Sunset (`/Game/Maps/Juliett/Juliett`), won 5–3

- Agent select at about 17:43, `core-game` in the log at 17:43:45. The user's team (Blue) won 5–3 in 8 rounds. Riot gave the user 429.5, distinction, and match MVP, with K/D/A 12/5/3.
- Request 13, during the match: `GET glz-eu-1.eu.a.pvp.net/core-game/v1/matches/{match 3}`, sent at 17:48:18 right after round 3 ended. 200, 9.7 KB. Shape:
  - `MatchID`, `Version`, `State` (`IN_PROGRESS`), `MapID`, `ModeID` (`.../Swiftplay_EoRCredits_GameMode...`), `ProvisioningFlow`, `GamePodID`
  - chat and voice MUC names and `TeamMatchToken`
  - `ConnectionDetails` (`GameServerHosts[]`, `GameServerHost`, `GameServerPort`, `GameServerObfuscatedIP`, `GameClientHash`, `PlayerKey`)
  - **`PostGameDetails: null`**
  - `Players[10]` with `Subject`, `TeamID`, `TeamNumber`, `CharacterID`, `bonsaiBloomline {bloomlineFlourishID, bloomlineShortLevel, bloomlineHypeID}`, `PlayerIdentity {AccountLevel, Incognito, HideAccountLevel, ...}`, `SeasonalBadgeInfo {SeasonID, NumberOfWins, WinsByTier, Rank, LeaderboardRank}`, `IsCoach`, `IsAssociated`, `PlatformType` and `PremierPrestige`
  - `MatchmakingData {QueueID, IsRanked}`

  **There are no kills, damage or scores in it mid-match.** `PostGameDetails` might fill in at the end, but the game fetches `core-game` again only in the same second as the match details push, so it can't come meaningfully earlier. Not tested.
- Request 14: `GET /match-details/v1/matches/{match 3}`, 200, 217 KB.
- **The end screens** (user's screenshots):
  - header `5 VICTORY 3`, with `DAILIES 4/4`, `MISSIONS 0/0`, `NEXT BATTLEPASS REWARD TIER 30` and `SWIFTPLAY`, `MAP - SUNSET`
  - a `MATCH REPLAY` download button
  - tabs on the left: `SCOREBOARD`, `PERFORMANCE`, `HEAD TO HEAD`, `TIMELINE`
  - the scoreboard has the same columns as match 2, with all 10 scores matching the data
  - the performance breakdown matched the data again:

    | Client shows | Match details |
    |---|---|
    | `429`, in the Masterful band | `TempValueF` 429.5, `distinction` |
    | `DAMAGE` `284`, two up | `TempValueA` 283.6, `double_up` |
    | `ADJUSTED KILLS` `1.5`, two up | `TempValueI` 1.49, `double_up` |
    | `ADJUSTED DEATHS` `0.7`, dash | `−TempValueJ` 0.70, `neutral` |
    | `TRADES` `0.25`, one down | `TempValueB` 0.25, `down` |
    | `ASSISTS` `0.38`, dash | `TempValueC` 0.38, `neutral` |
    | `UTIL. USAGE` `2.1`, one up | `TempValueD` 2.12, `up` |
    | `PLANTS` `0`, dash | `TempValueE` 0, `neutral` |
    | `DEFUSES` `0`, dash | `TempValueK` 0, `neutral` |

- The presence watcher behaved as in match 2. The score changed anywhere from 1 s before the log's round end to about 20 s after it, within the 20 s polling, and the final round never showed.

## Summary

**New endpoints that exist** (all on `pd.eu.a.pvp.net`, all called by the game itself):
- map rating: `region-index/v1/players/{id}?seasonID`
- mastery: `bloomline/v0/players/{id}` (with `?longOnly`), `.../bonsai/{agent hex}` (with `?includeModules`), `.../matches/{match hex}` and `bloomline/v0/config/valid-queues`
- gold stars, most likely the accolades page: `goldstars/v1/players/{id}` and `goldstars/v1/goldstars`

Lifetime stats (`cap.lifetime-stats`) and accolades (`ares-accolades`) showed up only as pushes. The game never requested either, so their read endpoints are still unknown.

**What answered for the user's own data:** every one of them, 200.
- **Map rating:** per map (7), win rates, `regionIndex`, `tempValueF`, totals, deltas, best match, recent matches and top agents. Competitive only.
- **Mastery:** levels and points for 29 agents, 9 numeric modules per agent, and per-match MP earned from time played plus a win bonus.
- **Gold stars:** 12 definitions with thresholds, plus per act and per match (the last 20) values for all 10 players in each match. Competitive only.
- **The queue config:** `PerformanceScoreQueues` includes swiftplay, unrated and premier.

**What the client showed after each match:**
- **Match 1** (Swift, won 5–4): 450, Masterful.
- **Match 2** (won 5–4): 184, Pass, with no grade word.
- **Match 3** (won 5–3): 429, Masterful.

Every number on the performance screens and scoreboards matches `scores.TempValue*` and `stats.*` in match details. The client's names for the hidden fields:
- `F` performance score, `O` grade (`distinction` shows as Masterful)
- `A` damage, `I` adjusted kills, `J` adjusted deaths, `B` trades, `C` assists, `D` util usage, `E` plants, `K` defuses
- the scoreboard's `TRADES` is `stats.TempValueB + C`

**The live question:**
- **Riot's performance score doesn't exist anywhere during a match**, not even in the game. The in-game Tab doesn't show it. The game requests nothing stats-related mid-match, gets no pushes, and its `core-game` mid-match has `PostGameDetails: null` and no per-player stats. It fetches match details 9–10 s after the final round ends and builds the performance screen from them.
- **Live trades can't be had either.** The log has death markers for only some deaths (`Debuff_Disabled_OwnerDead` on a dead player's deployables), and no killers anywhere.
- **What is live:**
  - the round score, from the user's own presence on the local API, with no lag beyond the polling interval
  - round ends and the match's real end, from the log
  - map, mode and queue
  - before the match: everyone's agent, rank, account level and mastery level
- **The best a live number can do:** K/D/A for all 10 players is on the Tab scoreboard, and nowhere else. A fit on K/D/A per round gets 91% of grades right over 2,556 competitive players (mean error 21). In match 3 it got 9 of 10 grades right, and the user's estimate after round 7 (458) had Riot's grade (429, Masterful). Using it live means reading the screen, which is outside this brief and the user's call.
- **The earliest real score:** the match details push, about 10 s after the final round. **lowkey doesn't get it for Swift, unrated or Premier today**, because its post-match refresh only follows competitive updates (`internal/app/session.go:49`).

**What surprised me:**
- Swift carries the full hidden score, with `hasTempValues: true`.
- `gameStartMillis + gameLengthMillis` ends before the final round even starts.
- Gold stars returns every lobby player's PUUID inside the user's own data.
- The game fetched map rating after match 2 but not after match 1.
- lowkey's recording got no event for the user's own presence mid-match. Only `GET /chat/v4/presences` returned it.
- `/help` has keys that differ only in case, which breaks Windows PowerShell's JSON parser.
