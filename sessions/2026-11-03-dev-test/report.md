# lowkey session report · 2026-11-03 · testing dev before it goes to main

- operator: Claude Code (Opus 5.5) on the Windows PC. Times are local (UTC+02:00) unless they say UTC
- what was run: `lowkey v0.1.10-13-g6e973ae`, built on the PC from the tip of `dev` (`6e973ae`), with `--record` from 14:23:02, from its own folder (`%USERPROFILE%\lowkey-dev`) with a copy of the user's everyday `lowkey.toml`
- played: a custom game (placing the dots), 3 TDM (one ended in agent select), a visit to the Range, 3 Competitive. The session line ended at `2W 1L +20 RR`
- **nothing in the app was changed on the PC.** The fixes below are for the Mac
- `recordings.zip`: `lowkey-record-20261103-142302` (`record.jsonl` 201 MB + `screens.txt`), scrubbed by lowkey. Checked before committing:
  - none of the 269 real player and match IDs in Valorant's logs from today appear in it (the one hit is a `CharacterID`, an agent missing from the 2026-10-25 static snapshot)
  - none of 33 real names (game log `player=` lines, chat participants, friends' presences) appear in it
- `extras/catalog/`: what the Riot Client, Riot's servers and the game log expose, phase by phase, as field names and types only (see "What exists where")
- `extras/agent-select-timeline.log`: the game log's lines for every match found, agent select loaded, pick, lock and voice room join, IDs masked
- `extras/_tools/`: the probes and scripts that made all of it. The leading `_` keeps them out of `go vet ./...`

## Verdict

**Not ready for main as it is.** One blocker: the voice column has no data source (below), so `voice_check` should come out. Everything else works in the real game. The rest of the fix list is a set of improvements the data supports.

## Setup

| | |
|---|---|
| doctor | **13/13 PASS** against the real game: eu/eu, client `release-13.09-shipping-18-5804228`, priority below normal, EcoQoS on, static snapshot 8 days old |
| settings | the user's flat v0.1.10 `lowkey.toml` was moved into sections on first start. Every value carried over and `lowkey.toml.bak` is byte-identical to the original |
| monitors | DISPLAY1 (primary) 1280x880 at (0,0) · DISPLAY2 2560x1440 at (1280,0) |
| Valorant | Windowed Fullscreen 1280x880 on DISPLAY1 |
| Overwolf | **not running** this session, so there's no GEP data |

## Automated

On the PC (Windows 11, Go 1.26.10, gcc for `-race`):

| | result |
|---|---|
| `go vet ./...` (windows) and `GOOS=linux go vet ./...` | clean |
| `go test -race ./...` in the working tree | golden tests fail: **CRLF, not code**. Git for Windows has `core.autocrlf=true` system-wide and the repo has no `.gitattributes`, so golden files are LF in git and CRLF on disk |
| `go test -race ./...` on an LF export (`git -c core.autocrlf=false archive dev`) | pass apart from two flaky tests (fix 7) |
| e2e against the built exe (`LOWKEY_EXE`, 13 tests) | **13/13 pass**. Idle CPU 0 s in 60 s, peak working set 70 MB after 598 match details. The dot windows come and go without taking focus |
| doctor against `fakeriot` | all PASS, including the move into sections |
| upgrade from v0.1.10 | every key main and v0.1.10 know, dev knows too |

A leftover `agent_dot_at` (from the pre-release agent-dot build in `%USERPROFILE%\lowkey-dot`) makes dev reject the whole file: `FAIL settings lowkey.toml: unknown setting "agent_dot_at" - using the last good settings`. Only pre-release test folders have that key. But removing `voice_check` (fix 1) will do the same to any file that has `voice_check`, so decide whether a removed key is ignored or rejected.

## Live

| found | what | notes |
|---|---|---|
| 14:23:53 | custom game | the 3 agent dots were placed here. lowkey: "not tracking custom games" |
| 14:26:36 | TDM · Glitch | agent select ended at 14:28:11 (back to the menus) |
| 14:28:24 | TDM · Glitch | played |
| 14:36:48 | TDM · District | played |
| 14:46:14 | the Range | ignored correctly, apart from a one-frame `MATCH LOADING` |
| 14:46:40 | Competitive · Bind | **found from inside the Range**. W, +19 |
| 15:22:13 | Competitive · Corrode | played |
| 15:52:45 | Competitive · Corrode | played. The user had switched `server_pod` off at 15:52:30 |

### Settings tab

The 5 groups showed with the right values. Every toggle saved at once (checked in `lowkey.toml` after each change). The user went through the agent dots list and dot time and reported no problems.

### Agent dots

- **Placing:** 3 agents. Jett was saved at **(2077, 488), which is on DISPLAY2**, because that's where the cursor was when the 5 s count ended. The user saw only 2 dots at the next match found. Jett was re-placed at (314, 448) **during a matchmade agent select** at 14:28:49, so placing works there too (fix 5)
- **Showing:** up at every one of the 6 match-founds, 0.05 s after the `PREGAME` event, including from the Range (52 ms). Never during a match load or a round

| found | agent select loaded | pick | dots gone | lowkey saw the pick | first ares-pregame event after the pick |
|---|---|---|---|---|---|
| 14:26:36 | +6.8 | +13.0 | +13.1 picked an agent | +13.1 | +13.0 |
| 14:28:24 | +6.8 | +12.9 | +13.0 picked an agent | +13.0 | +12.9 |
| 14:36:48 | +6.8 | +12.8 | +12.9 picked an agent | +12.9 | +12.8 |
| 14:46:40 (from the Range) | +7.6 | +12.8 | **+15.1 time limit** | **+15.3** | +12.8 |
| 15:22:13 | +6.9 | +33.6 | +15.1 time limit | +33.6 | +33.6 |
| 15:52:45 | +6.9 | +15.6 | **+15.1 time limit** | +15.6 | +15.6 |

- When the user instalocks, the pick lands at **12.8–13.0 s every time**. That leaves about 2 s of margin before the 15 s default.
- 14:46:40: a teammate's event at +12.2 used up the refetch (`rosterEvery`, 3 s). The pick at +12.8 then waited until +15.3 and the time limit took the dots first. Eight ares-pregame events arrived in between and were ignored (fix 2).
- In 3 of 6 match-founds the dots were gone before the pick or before lowkey saw it (fix 3).

### Server pod

`POD: Frankfurt 1` in the header at match load and in the match, for every match until the user switched it off. The pod ID was `aresriot.aws-euc1-prod.eu-gp-frankfurt-1` in every pregame and core-game response.

### Voice column

**Never had data.** `Voice()` asks the placeholder `/telemetry/v1/voice` (`internal/riot/local.go:225`). It got 404 `Invalid URI format` **74 times**, about every 3 s through each agent select. The column only ever showed `-`. See "Voice: what was tried" below.

### Parties

- Own party: `◆` on two partied allies in the Bind match.
- No enemy letters all session. In the Bind match two enemies shared a Bind W 13-7 from 33 minutes earlier, but one was hidden. Hidden players never give or get a letter, by design (`tests/app/party_test.go:225`).
- With no visible enemy pair, exact letters stay **untested live**.

### Player details, site scouting, weapon profiling

- Details work mid-match, for hidden players too (stats, never the name). Left/right and Esc work.
- **Site scouting on Bind:** the `TENDENCIES · BIND (N matches in sample)` block has `DEFENSE`, `ATTACK` (plants) and `OPENING` as A/B splits. The sample sizes match each player's Bind matches in the list below it.
- Weapon profiling, run with lowkey's own `LeaningsOf` over **all 383 matches recorded today, 42 lobby players** (`extras/_tools/leanprobe`):

| rule | flagged | what they actually do |
|---|---|---|
| Operator (`opKills*100 > 25*longKills \|\| opRounds >= 3`, `internal/stats/tendencies.go:202`) | **10 of 42** | Operator in **2–6% of their duelist rounds** and **≤15% of their rifle/sniper kills**. The 25% condition never fired, and none of them is an Operator player |
| Shotguns (`n*100 > 20*kills \|\| n >= 15`, `:205`) | 1 of 42 | 19 Judge/Bucky kills, 10% of their kills. The other 41 are at 0–4% |

### Requests

- Three 429s on `match-details`, one per match load (14:29:08, 14:37:30, 14:47:52). Older builds hit the same thing: Oct 31 had 1 in 2 loads, Nov 2 had 1 in 1. **Not from dev.** Dev adds no remote requests.
- After a match, lowkey made 3 requests: token, `competitiveupdates`, own match.

## Fix list for the Mac

1. **Remove `voice_check`. Blocker.** It touches the setting (`internal/settings/settings.go:142`, `:250`), the column insert (`internal/app/live.go:623`), `voiceCells`/`fetchVoice` (`internal/app/lobby.go:523`, `:534`), `Voice`/`DecodeVoice` (`internal/riot/local.go:225`), `render.ColumnVoice` and `tests/app/voice_test.go`. Decide what an existing `voice_check` line in a file does afterwards (see Automated).
2. **Notice the pick faster while dots are up.** `rosterEvery = 3 * time.Second` (`internal/app/lobby.go:53`) holds back the refetch an ares-pregame event asks for. While dots are up, about 0.5 s would have caught 14:46:40, for a few more pregame GETs per match found.
3. **Raise the dot default from 15 to 20 s** (the maximum). The dots go on the pick anyway, so the limit only matters when nobody picks. Three of six went before the pick or before lowkey saw it (+15.3, +15.6, +33.6).
4. **Weapon profiling:** count the Operator per round, e.g. Operator in at least 15% of duelist rounds or at least 25% of rifle/sniper kills, instead of `opRounds >= 3`. On today's 42 players that flags nobody, which is right. The shotgun rule is fine.
5. **Placing should check the monitor.** `tickPlacing` (`internal/app/dot.go:188`) saves wherever `Dot.Cursor()` is at zero. A notice when the spot isn't on the monitor Valorant's window is on (or is over lowkey's own window) would have caught Jett.
6. **Stale comment written into every `lowkey.toml`:** `internal/settings/settings.go:157` says "place dot on the settings tab sets where it goes". The row is "agent dots" now.
7. **Tests on Windows:**
   - `settingsFile()` (`tests/app/settings_test.go:22`) calls `t.Fatal` on a read error. Reading `lowkey.toml` while `settings.replace` renames over it gives `The process cannot access the file because it is being used by another process`. It failed **1 in 30 runs** of `TestPlacingTheSecondSlotKeepsTheFirst`, and **4 in 5 `-race` runs** of the dot/settings tests (`TestTheDotRowsChangeTheFile`, `TestTheAgentDotsListPicksAgentsWithLeftAndRight`). It should retry inside `eventually`.
   - Add a `.gitattributes` (e.g. `* text=auto eol=lf`) so golden files match on a Windows checkout.
   - `TestArrowsPickAPlayerAndEnterShowsEverythingKnownAboutThem` failed once under `-race` (0 in 30 without): `r.screen("enter for details")` can match the frame after the first Down. It's the same on main.
8. *Optional:* Left/Right on a placed slot clears its spot (`internal/app/settingstab.go:171`), so moving off and back to the same agent loses it.
9. *Optional:* the one-frame `MATCH LOADING` when the user enters the Range.

## Voice: what was tried

**Nothing lowkey can reach today says who is in team voice.** Tried live in agent select, in TDM and in a Competitive match:

- Riot Client local API, with the lockfile password:

| request | result |
|---|---|
| `voice-chat/v1/active-product` | 204 every time |
| `voice-chat/v2/sessions` | `[]` |
| `voice-chat/v3`/`v4/sessions/valorant`, and the same by room ID | 500 `We couldn't find any product for session '<id>'` |
| `voice-chat/v2/sessions/<room>` | 404 |
| `v5/.../participants/report` and `v4/.../transcriptions` | 500 |
| `v1/heartbeat-metrics` | 404 |

- Local websocket, subscribed to every event: 3 team voice room joins and 0 voice-chat events. The only voice-related events were the account's own `voice-permission`/`request/voice-chat` (`Allowed`).
- Presence: 53 fields, none about voice.
- Game log: `LogRNetVoiceManager` only has the user's own side: team and party room joins (`<matchId>-tm1`/`-tm2`, `<partyId>-pty`), mute requests the user sends, PTT sync and audio devices.
- **The one lead left:** the Riot Client log has `voice-chat: Binding a session <token> to product valorant patchline live` (122 times). Valorant starts with its own `-remoting-auth-token`. The 500s say the lockfile session has no product bound, so a request made as the game's session would probably see `voice-chat/v4/sessions/valorant`, and the match chat rooms too (`/chat/v6/conversations/ares-coregame` was empty for the lockfile session mid-match). **Not tried:** the PC's permission system blocked reading the game's token as credential exploration. Using it is a different trust level from what the README promises, so it's the user's call.

## What exists where

| question | answer | source |
|---|---|---|
| who is in team voice | no (above) | |
| live score in a match | **yes**: `partyOwnerMatchScoreAllyTeam`/`EnemyTeam` | own presence, pushed over the websocket |
| round number, round end, winner | **yes**: `OnRoundEnded for round N`, `Gameplay started`, `Match Ended … Winning Team (Attacker/Defender)` | game log `LogShooterGameState` |
| surrender / remake / timeout votes | **yes**, with Yes/No | game log `LogVoteControllerComponent` |
| per-round performance | **yes**, 83 numbers a round | game log, `End of Round Perf statistics` |
| live damage / HS / 10-player K/D/A | only Overwolf GEP (see the memory note from 2026-10-30) | not run today |
| queue, map, mode, party size, idle | yes | presence |
| post-match summary, stats delta | Riot pushes `gdu/…/infoType/SUMMARY` and `REPLAY`, `cap/stats/v1/DeltaEvent`, `ares-accolades`, `ares-mastery/bloomline` | websocket (RMS) |
| skins in agent select | `GET /pregame/v1/matches/{id}/loadouts` exists (the game calls it in PREGAME) | game log requests |
| avoid-as-teammate list | `GET /restrictions/v1/avoidList` | game log requests |
| a player's Premier team | `GET /premier/v3/players/{id}/playerRosterPublicInfo` | game log requests |
| agent mastery of other players | `GET /bloomline/v0/players/{id}` | game log requests |
| local replays | `%LOCALAPPDATA%\VALORANT\Saved\Demos\*.vrf` (7 files, 320 MB, from 2025), binary, header holds the match ID | disk |

The remote endpoints' fields weren't fetched. That needs the account tokens, so it's the same permission question as the voice lead.

### extras/catalog

| file | what | made by |
|---|---|---|
| `game-requests-from-logs.json` | **122 Riot endpoints Valorant itself calls** (glz 47, pd 65, shared 4, pp.sgp 6), from 18,930 requests in 11 logs: method, path with IDs masked, the game's query names, phases, status codes | `_tools/game_requests.py` |
| `local-api-routes.json` | 226 local API routes that exist, found from no-argument GETs in the 2026-11-02 help dump. Auth, tokens, payments, chat message text and console accounts were skipped | `_tools/catalog` |
| `local-api-by-phase.jsonl` | fields and enum values of each route, swept once per phase: INGAME, PREGAME, MENUS (cut short) and NOT_IN_VALORANT (after the game closed) | `_tools/catalog` |
| `websocket-events-by-phase.jsonl` | 300 event shapes over 92 URIs, with RMS payloads decoded | `_tools/catalog` |
| `websocket-other-event-types.jsonl` | the other 63 event types in the help. Only named copies of `OnJsonApiEvent` ever fired; LCDS and service-proxy events never did | `_tools/allevents` |
| `lowkey-endpoints-from-recording.jsonl` | fields of every response lowkey got today, plus 22 RMS push kinds | inline script over `record.jsonl` |
| `game-log-categories.json` | 210 game log categories with line counts per phase. Templates only for categories without player names | inline script over the game logs |
| `round-perf-2026-11-03.json` | 62 rounds × 83 fields | `_tools/round_perf.py` |

Values are kept only for enum-like fields. Account sign-in provider and age category, friends' chat resource IDs and activity IDs were masked.

## Round performance (Valorant's own numbers)

| match | rounds | frame time median (range) | CPU outside Valorant, median |
|---|---|---|---|
| Competitive · Bind | 22 | 4.73 ms (4.27–5.23) | 9.7% |
| Competitive · Corrode | 18 | 4.55 ms (3.95–5.01) | 11.1% |
| Competitive · Corrode | 20 | 4.49 ms (4.03–4.92) | 10.0% |

- dev lowkey ran through every round.
- `Valorant CPU Util` is per core and `System CPU Util` is the whole PC (12 logical processors). Divide the first by 12 before subtracting.
- The probing itself showed up once. In rounds 8–9 of the first Corrode match, a 230 MB log scan and Go builds pushed non-Valorant CPU to 14.3–14.8%. Frame time (4.74–4.97 ms) stayed inside that match's spread. After that, heavy work waited for the menus or ran at idle priority.
- This telemetry could measure lowkey's own cost round by round without PresentMon.

## Notes

- The live tail meant to collect the timeline on the PC (`tail -F` on `ShooterGame.log`) captured nothing. `agent-select-timeline.log` was rebuilt from the full log after Valorant closed.
- In the agent select at 14:23:53 (custom game) the pick came at +74 s. It's left out of the dot numbers.
