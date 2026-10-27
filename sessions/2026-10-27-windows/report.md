# lowkey test session report · 2026-10-27

- operator: Claude Code (Opus 5.5) on the Windows PC, working from the 2026-10-25 brief
- session start: 12:33:27 local (UTC+02:00), so every time below is local unless it says UTC
- planned end: about 13:33. In practice the user kept playing until 17:04 (four competitive games on the patched build), and the wrap-up below was written at 18:10

## 01 · Environment basics
- status: done
- Windows 11 Pro for Workstations (edition ProfessionalWorkstation), version 26H2, build 26300.9711, 64-bit
- CPU: Intel Core i7-8700K @ 3.70 GHz, 6 cores / 12 threads, no hybrid: every CPU set reports efficiency class 0, so there are no separate performance and efficiency cores
- default terminal: "Let Windows decide" (`HKCU\Console\%%Startup` DelegationConsole and DelegationTerminal are both `{00000000-…}`). Windows Terminal 1.24.12741.0 is installed, and a console started with `Start-Process` opened in Windows Terminal, so Windows Terminal is in effect the default
- new terminal window at default size: 120 columns × 30 rows, measured in a fresh window. Windows Terminal's settings.json sets no `initialCols`, `initialRows` or `launchMode`, and conhost's default is also 120 × 30
- git 2.50.0.windows.1
- GitHub CLI 2.103.2 (2026-10-23), logged in as the repo owner's account (keyring, scopes `gist`, `read:org`, `repo`). The repo is private and this account is ADMIN on it
- Go: not installed (not on PATH, no `C:\Program Files\Go`)
- Valorant: installed in the default location, `C:\Riot Games\VALORANT\live\VALORANT.exe`, with the Riot Client at `C:\Riot Games\Riot Client\RiotClientServices.exe`. `RiotClientInstalls.json` and the product settings agree
- side note: the Desktop is redirected to OneDrive (`C:\Users\itayf\OneDrive\Desktop`), so `%USERPROFILE%\Desktop` is the wrong place for a hand-over zip. The user asked for the hand-over to go through the repo instead, so everything is in `sessions/2026-10-27-windows/` on `main`

## 08 · Get lowkey, doctor and the first recording
- status: done
- download: `gh release download v0.1.0` worked. The release was published 2026-10-25T15:50:17Z, and its only asset is `lowkey.exe` (7,621,120 bytes) with GitHub digest `sha256:8732759b…4b6f`
- SHA256: `8732759B6A8B86B001EA3DAA944CD6DEB4B15B4989691F2D6CE688C48D6F4B6F`, which matches the brief
- `lowkey.exe --version` printed `lowkey v0.1.0`, exit 0
- no SmartScreen prompt: the gh download carries no mark of the web (`Zone.Identifier`), and Defender didn't block or remove it
- extra, real-world case: a stale lockfile. At 12:38, with no Riot Client running (only Vanguard's `vgtray`), the lockfile from the client's last run on 2026-10-22 13:52 was still on disk, because the Riot Client doesn't always remove it on exit. A plain `lowkey.exe doctor` (no `--record`) printed:
  - `PASS  lockfile        …\Riot Client\Config\lockfile (port 15018)`
  - `FAIL  local API       can't reach https://127.0.0.1:15018: … connectex: No connection could be made because the target machine actively refused it.`
  - then `not checked - needs the local API` for the other five, and PASS for the offline checks
  - suggestion: the lockfile carries the client's PID, so doctor could say "left over from an earlier Riot Client - is it running?" when that PID is gone. Right now the "is the Riot Client running?" hint only appears when the file is missing. Worth checking that live mode also picks up the new lockfile once the client starts
- nit in that run: `static data  snapshot from 2026-10-25 (1 days old)`. "1 days" should be "1 day", and it's the 27th, so by the calendar it's 2 days. The count truncates hours
- `doctor --record` at 12:45:17, about 3 minutes after the Riot Client wrote its new lockfile: **all checks passed**, saved as `doctor.txt`. Recording: `lowkey-record-20261027-124517`
  - lockfile port 49884, local API answered with the lockfile password, tokens expire in 57m
  - phase `Menus (hybrid presence)`, session `region eu · shard eu`
  - client version `release-13.08-shipping-22-5760300 (from presence)`
  - rank data came back from pd (shard eu)
  - priority below normal, efficiency on (EcoQoS) on this non-hybrid CPU
  - console `width 120 · ANSI off - plain output` and settings `no lowkey.toml yet`, the two expected lines

## 09 · The user's own row
- status: partial. Both rows are saved. `me-after.txt` was skipped: by wrap-up (18:08) Valorant had been closed since 17:04 and the Riot Client was logged out, so `lowkey me` had nothing to connect to. The user didn't give their real W-L and RR for the SESSION check
- SESSION line over the day (live frames, v0.1.2): `0W 0L +0` → after Bind `0W 1L -18` → after Sunset `1W 1L +2` → after Corrode `2W 1L +20` → after Summit **`3W 1L +38 RR since 06:00`** (17:04:19). Each step matches one result in order (L −18, W +20, W +18, W +18), but none of it is checked against the client
- `me --record` at 12:45:56, finished in about 3 s, exit 0. Saved as `me-competitive.txt`, recording `lowkey-record-20261027-124556`:
  - `RANK Asc1 44 · PEAK Asc3 · LAST Dia3 · WR 58% · W-L 11-8 · K/D 1.16 · HS% 30% · ACS 236 · ADR 160 · FB% 6% · N 13 · MAINS Vyse Clov · TREND +23`
  - `SESSION · 0W 0L +0 RR since 06:00`
  - requests, all HTTP 200 with no rate limiting: entitlements ×2, presences, external-sessions, content, mmr, competitiveupdates, match-history, match-details ×13
- `me --mode unrated --record` at 12:47:27, exit 0. Saved as `me-unrated.txt`, recording `lowkey-record-20261027-124727`: same RANK/PEAK/LAST/TREND, every sample column `-` and N 0, because there are no recent unrated matches. Same SESSION line
- **finding: Riot's match details no longer carry names.** In the 13 match-details responses, all 130 `players[]` entries have `gameName: ""` and `tagLine: ""`. The scrubber passes empty strings through unchanged, so these came from Riot this way. lowkey takes every name from match details (`stats.SampleOf` → `Sample.Name`), so:
  - in `me-competitive.txt` the PLAYER cell is **blank**. Nothing needed replacing with `<self>`, and the file is left exactly as lowkey printed it
  - in `me-unrated.txt` the PLAYER cell is `-`, since there's no sample at all. Blank with a sample and `-` without one is inconsistent
  - expect the same in the lobby: every non-hidden row with a blank PLAYER cell. fakeriot's match details still include names, so the tests don't catch this
  - the only name source left that doesn't use the name service is the user's own presence (`game_name`/`game_tag`), and that only covers the user and their friends
- `players[]` keys seen in real match details (union): `accountLevel, behaviorFactors, bonsaiBloomline, characterId, competitiveTier, gameName, isObserver, newPlayerExperienceDetails, participationPeriods, partyId, platformInfo, playerCard, playerTitle, preferredLevelBorder, premierPrestige, roundDamage, scores, sessionPlaytimeMinutes, stats, subject, tagLine, teamId, teamNumber, xpModifications`. Some entries lack `preferredLevelBorder`, `sessionPlaytimeMinutes` or `xpModifications`

## 07 · Display and comfort
- status: partial. The first two answers are in. The comfort questions (window visible and readable, alt-tabbing in agent select) and the FPS/stutter/input question were never answered, because the user went straight from match to match
- monitors: 2, both 2560×1440 side by side. The primary is `\\.\DISPLAY2` at (0,0), running 164 Hz per the GPU (RTX 4060 Ti). `\\.\DISPLAY1` sits to its right at (2560,0). Device Manager names them Odyssey G5 and GM2711S. The user asked me to look these up rather than report them
- Valorant display mode: Windowed Fullscreen
- the user plays **true stretched**, and for that keeps both monitor devices disabled in Device Manager. Both show as `Generic Monitor (…)`, status Error, `CM_PROB_DISABLED`, still present, so Windows reports no monitor info through WMI. With Valorant closed, the desktop is at the native 2560×1440

## 10 · Games
- status: done. 11 lobbies over 4 h 16 m: the Range ×2, Deathmatch ×3, TDM, Glitch and Competitive ×4. Two Deathmatches were missed on v0.1.0 by the ghost bug, which v0.1.2 fixes, and every match after the swap was caught
- lowkey started at 12:48:11 with `Start-Process … --record`. It opened in a Windows Terminal window titled `lowkey` and created `lowkey.toml`. Recording: `lowkey-record-20261027-124811`
- first frame at 12:48:12: `waiting for the next match`, then `SESSION · 0W 0L +0 RR since 06:00`
- the user's true-stretch launch moved things: once Valorant was up, `\\.\DISPLAY1` became the primary at **1280×880** (the stretched resolution) and `\\.\DISPLAY2` (2560×1440) became the second monitor at x=1280. lowkey is meant to sit on DISPLAY2
- first reading (12:48:16): CPU 0.16 s · 21 MB (peak 21 MB) · BelowNormal
- modes, in the user's order: the Range, Deathmatch, Team Deathmatch, then Swiftplay if it fits. The user also floated a competitive game, but at 35–45 min it doesn't fit in the hour

### Warm-up · The Range (not a match)
- frames: MATCH LOADING 12:50:28, header `MATCH LOADING · The Range · Unrated`, then IN MATCH 12:51:08, exactly 40 s later. One ALLIES row: the user on Phoenix, name `-`, the competitive rank columns filled, unrated sample N 0, TREND +23, plus an AVERAGE row
- **finding: lowkey treats the Range as an Unrated match.** The core-game match has `ProvisioningFlow: "ShootingRange"`, `ModeID: "/Game/GameModes/ShootingRange/ShootingRangeGameMode.ShootingRangeGameMode_C"`, `MapID: "/Game/Maps/PovegliaV2/RangeV2"`, `MatchmakingData: null` (no queue at all), `State: "IN_PROGRESS"`, `IsReconnectable: false` and 1 player. lowkey still ran the match-load lookups: core-game match, the user's party, content, mmr and match-history with `queue=unrated`. Suggestion: treat `ProvisioningFlow == "ShootingRange"` (or no `MatchmakingData`) as untracked, `not tracking The Range - waiting for the next match`
- **finding: lowkey reads the user's own party, but the brief says it never calls party endpoints.** At match load it sent `GET glz /parties/v1/players/<self>` and then `GET glz /parties/v1/parties/<partyId>`. Both returned 200, both GET, and the ID is the user's own PUUID, the same one as in its mmr calls. This is deliberate: `remote.Guard.ownPartyRead` allows exactly these two shapes. It's read-only and the user's own data, the same calls the Riot Client makes, so I judged it no account risk and carried on. But the brief's description of lowkey ("never … party endpoints") is out of date, so one or the other should change
  - guard gap: `ownPartyRead` accepts `/parties/v1/parties/<any id>`, not just the party ID from the self lookup. Real match details carry a `partyId` for every player, so a future change could look up someone else's party and the guard wouldn't stop it. Worth pinning it to the self party's ID
- core-game `Players[]` keys (passively from lowkey's recording): `CharacterID, IsAssociated, IsCoach, PlatformType, PlayerIdentity, PremierPrestige, SeasonalBadgeInfo, Subject, TeamID, TeamNumber`. Match keys: `AllMUCName, ConnectionDetails, GamePodID, IsReconnectable, MapID, MatchID, MatchmakingData, ModeID, Players, PostGameDetails, ProvisioningFlow, State, TeamMUCName, TeamMatchToken, TeamVoiceID, Version`
- encoding: `screens.txt` is valid UTF-8, with `·` stored as `C2 B7`
- reading at 12:51:04, in the Range: CPU 0.25 s · 24 MB (peak 24 MB) · BelowNormal
- back in the menus at 12:55:03: `waiting for the next match` over `LAST MATCH · The Range · Unrated`, so the Range now holds the last-match slot. A session refresh followed at 12:55:18 (`competitiveupdates`, 200)
- reading at 12:55:30: CPU 0.27 s · 23 MB (peak 24 MB) · BelowNormal

### Match 1 · Deathmatch (Lotus): missed by a ghost match load
- frames: MATCH LOADING 12:55:37, header `MATCH LOADING · The Range · Deathmatch` with no rows, only the SESSION line. IN MATCH 12:56:17, again `The Range · Deathmatch`. No frame for the real match
- **bug: a late core-game message about the finished Range match started a ghost match load, and lowkey then ignored the real Deathmatch.** Timeline from the recording (`lowkey-record-20261027-124811`, user's presence plus RMS messages):
  - 12:55:00.1–12:55:02.1: RMS `ares-core-game/core-game/v1/matches/<range>` ×4, `…/players/<self>` and `ares-session/v1/sessions/<self>`, all `eventType: Create`. This is the Range ending. lowkey was in its in-round phase and correctly did nothing
  - 12:55:03: presence `MENUS`, so lowkey went back to menus (LAST MATCH = the Range)
  - 12:55:25: presence `MENUS`, queue `deathmatch`. 12:55:32: `partyState: MATCHMAKING`, the queue starting
  - **12:55:37.675: one more RMS `ares-core-game/core-game/v1/matches/<range>` (Create)**, 37 s after the Range ended, for the same finished match. `live.onCoreGame` → `startMatchLoad` fired because the phase was menus. lowkey fetched `GET glz /core-game/v1/matches/<range>`, which came back **`State: "CLOSED"`, `Players: []`**, with `PostGameDetails` set. It labelled that with the queued mode, giving the bogus `The Range · Deathmatch`, and started its 40 s cutoff
  - 12:55:56.2–12:55:56.7: RMS for the real match: `ares-core-game/core-game/v1/matches/<dm>` ×4, `…/players/<self>`, `ares-session/v1/sessions/<self>`. **Ignored**, because `onCoreGame` only takes a match ID while `phaseMatchLoad` has `lobby.matchID == ""`
  - 12:55:57: presence `INGAME`, queue `deathmatch`. 12:56:06: `provisioningFlow: Matchmaking`, `matchMap: /Game/Maps/Jam/Jam` (Lotus). `applyPresence` does nothing for INGAME while in `phaseMatchLoad`
  - 12:56:17: the cutoff passed, so `IN MATCH · The Range · Deathmatch`. No request ever touched `<dm>`
- suggested fixes, any one of which would have caught this:
  - ignore core-game messages for match IDs lowkey has already finished
  - drop a fetched core-game match whose `State` is `CLOSED` or whose `Players` is empty
  - only start a match load from RMS when presence isn't `MENUS`, or wait for presence `INGAME`
  - let a core-game message for a *different* match ID replace the one being loaded
- the dedupe buffer (`recentEvents`) didn't help: the late message came 37 s and many events after the first ones
- workaround for the rest of the session: wait about a minute in the menus after each match before queueing. Any MENUS presence update (picking a mode, starting the queue) calls `toMenus()` and clears a ghost
- table against the game: none, lowkey never saw this match
- status lines: none
- dodge: no (Deathmatch has no agent select)
- end of match: the RMS burst for `<dm>` came at 13:02:06.9–13:02:08.9 (core-game `matches/<dm>` ×12, `players/<self>`). There was also a lone `matches/<dm>` at 13:01:04 mid-match, which lowkey ignored in-round. Presence `MENUS` at 13:02:09.6. LAST MATCH frame at 13:02:09: `waiting for the next match` over **`LAST MATCH · The Range · Deathmatch`**, so the ghost became the last match
- **the ghost reproduced after a real match.** A late RMS `matches/<dm>` (Create) arrived at 13:02:38.5, 31.6 s after the end burst and 29 s after MENUS. lowkey fetched `<dm>`: `State: "CLOSED"`, **1 player** (not the user: a streamer-mode player on Fade, `Incognito: true`), `ProvisioningFlow: Matchmaking`, map `Jam`, queue `deathmatch`. It drew `MATCH LOADING · Lotus · Deathmatch` with one row `hidden  Fade` and `AVERAGE 0/1`, plus the party reads again. So after every match, about 30–37 s after the end, lowkey starts a ghost match load. If the next match loads before a MENUS presence update clears it, that match is missed
- the user has **streamer mode on**: their own `PlayerIdentity` in the Range had `Incognito: true` and `HideAccountLevel: true`. lowkey still showed the user's own row and looked them up, which is correct (`Guard.Hide` and `lobby.go` both exempt self)
- reading at 13:02:59: CPU 0.31 s · 23 MB (peak 24 MB) · BelowNormal
- the user was told to queue Team Deathmatch at about 13:04, after the ghost, so the queue's MENUS presence update would clear it

### Match 2 · Team Deathmatch (Piazza)
- the workaround held: presence `MENUS` with queue `hurm` at 13:04:06 called `toMenus()`, so `waiting for the next match` over `LAST MATCH · Lotus · Deathmatch` (the ghost, not a real table). `partyState: MATCHMAKING` at 13:04:09
- agent select: RMS `ares-pregame/pregame/v1/players/<self>` and `…/matches/<pregame>` at 13:04:22.7. First frame `AGENT SELECT · Piazza · TDM` at 13:04:22, before the presence even said `PREGAME` (13:04:23.05)
  - `GET /pregame/v1/matches/<pregame>`: `PregameState: character_select_active`, 5 allies, `QueueID: hurm`
  - lookups, all 200, two in flight, done by 13:04:28.9: own party ×2, mmr ×4 (self plus 3, none for the hidden player), competitiveupdates ×4, match-history ×4, match-details ×34
  - after that, `GET /pregame/v1/matches/<pregame>` again roughly every 3 s (13:04:35, :38, :41, :44, …), each set off by a burst of RMS pregame messages. These roster refreshes are lowkey's only steady traffic during agent select
- agent select table at 13:04:41: five rows, every PLAYER cell blank or `-` (no names, see item 09):
  - `Jett   Asc1 44  Asc3  Dia3  21%  7-26  1.41  43%  -  -  -  13  Jett Reyn  +23`, the user (it matches `me`)
  - `Reyna  Plat2 66  Plat2  Plat1  21%  3-11  1.08  28%  …  13  Reyn Jett  +98`
  - `-  Iso  Brnz2 74  Brnz3  -  0%  0-12  …  0  -  +24`. The name cell is `-` with N 0, but W-L says 0-12
  - `hidden  Neon` (streamer mode, not looked up)
  - `Raze   Plat2 26  Plat2  Plat1  31%  4-9  1.42  34%  …  8  Jett Reyn  +13`
  - `AVERAGE 4/5  Plat1  Plat2  Plat3  18%  1.30  35%  +40`
  - ACS, ADR and FB% are `-` in TDM, as the mode rules say
- frames: AGENT SELECT 13:04:22 · MATCH LOADING 13:05:30 · IN MATCH 13:06:10 · LAST MATCH 13:12:27
- gameplay start: 13:06:06.4 (`Gameplay started` in ShooterGame.log, see Loading times below), **36 s after MATCH LOADING, so combat began 4 s before lowkey's 40 s cutoff**. TDM has no buy phase. The game state went `TransitionToInGame` 13:05:30.6 → `InGame` 13:05:46.7, then about 20 s of pre-round
- requests: agent select made 60 requests (13:04:22.0–13:05:27.3): pregame ×11, mmr ×9, match-history ×4, match-details ×34, own party ×2. Match load made 60 more (13:05:30.7–13:05:38.6): core-game ×1, mmr ×8, match-history ×4, match-details ×47. **All 200, no rate limiting.** The last request went out 28 s before gameplay started, and nothing during play
- core-game at match load: `State: IN_PROGRESS`, 10 players, map `HURM_Yard`, queue `hurm`. `PregameState` went `character_select_active` → `character_select_finished` at 13:05:27
- **the ghost wiped the real LAST MATCH table.** The match ended (RMS burst 13:12:24.0–13:12:26.7, presence MENUS 13:12:27.1) and the real `LAST MATCH · Piazza · TDM` appeared at 13:12:27. Then the late RMS `matches/<tdm>` arrived at 13:12:56.3 (32 s after the burst), lowkey fetched it (`State: CLOSED`, **0 players**), and drew a ghost `MATCH LOADING · Piazza · TDM`. When the user's queue started at 13:13:05, `toMenus()` filed that empty ghost as the last match, so from 13:13:05 the screen showed `LAST MATCH · Piazza · TDM` with **no rows at all**. The last-match view only lasts about 30 s after each match
- table against the game: not checked, the user never answered the after-match questions
- status lines: none
- dodge: no
- reading at 13:13:35: CPU 3.63 s · 27 MB (peak 31 MB) · BelowNormal. Agent select plus match load for TDM cost about 3.3 s of CPU

### Match 3 · Deathmatch (Lotus)
- the user queued at 13:13:06 (presence MATCHMAKING), after the TDM ghost, so it was caught
- frames: MATCH LOADING 13:13:42, `MATCH LOADING · Lotus · Deathmatch`, one list of 12: the user (Viper, `Asc1 44 · Asc3 · Dia3 · K/D 0.96 · N 13 · MAINS Brim Dlck · TREND +23`), 4 more looked up, **7 hidden** (streamer mode), and `AVERAGE 5/12  Plat3  Dia1  Plat3  0.98  -1`. Every PLAYER cell is blank, no names
- core-game: `State: IN_PROGRESS`, 12 players, map `Jam`, queue `deathmatch`
- requests: 75 (13:13:42.7–13:13:56.6): core-game ×1, own party ×2, mmr ×9, match-history ×5, match-details ×58, all 200
- game state: `TransitionToInGame` 13:13:48.2 (from ShooterGame.log)
- frames: MATCH LOADING 13:13:42 · IN MATCH 13:14:22 · LAST MATCH 13:18:22 (`waiting for the next match` over `LAST MATCH · Lotus · Deathmatch`)
- the user **left this match early**. At 13:18:19 came RMS `core-game/v1/players/<self>` plus one `matches/<dm2>`, then presence MENUS at 13:18:22.5, and lowkey correctly drew LAST MATCH. The match itself carried on: at 13:20:30, `<dm2>` was `POST_GAME` with **11** players, against 12 at load

### Match 4 · Deathmatch (Plummet): missed by a ghost, second trigger
- the user started another Deathmatch (MATCHMAKING 13:20:06), about 1 m 45 s after leaving match 3
- **13:20:30.7–13:20:35.3: `<dm2>` really ended, and its RMS burst (`matches/<dm2>` ×13) arrived while the user was in queue.** lowkey was in menus, so `onCoreGame` started a match load for `<dm2>`. It fetched `State: "POST_GAME"`, 11 players, and drew `MATCH LOADING · Lotus · Deathmatch` with match 3's roster minus the user (4 looked up, 7 hidden, `AVERAGE 4/11`). It also re-ran mmr ×4, competitiveupdates ×4, match-history ×4 and the party reads for those players
- 13:20:48.4: RMS for the real match `<dm3>` (`matches/<dm3>` ×4, `players/<self>`). Presence INGAME 13:20:48.7, map `/Game/Maps/Plummet/Plummet` at 13:20:56.7. **Ignored**, same as match 1
- 13:21:05.8: one more `matches/<dm2>`. 13:21:10: the ghost's cutoff, so `IN MATCH · Lotus · Deathmatch`, the wrong map and the wrong lobby. The user noticed during the match and said "it did not update"
- so the ghost has two triggers: (1) the late single message about 30 s after a match you played to the end, and (2) **the real end of a match you left early**, which can come minutes later, here 2 m 11 s. The "wait a minute before queueing" workaround only covers the first. The fixes listed under match 1 cover both, but "ignore match IDs already finished" has to count a match you left as finished. Checking `State` against `IN_PROGRESS`, or presence INGAME, is the most robust fix
- table against the game: none for the real match
- status lines: none
- dodge: no
- end: the RMS burst for `<dm3>` came at 13:27:05–13:27:16 and presence MENUS at 13:27:17.6, played to the end. lowkey's LAST MATCH at 13:27:17 was **`LAST MATCH · Lotus · Deathmatch`**, the ghost of match 3. Plummet never appeared on screen

### Match 5 · Glitch (`abilitydraftarena`, a new arcade mode)
- 13:27:39: presence MENUS, queue `abilitydraftarena`. 13:27:41: MATCHMAKING
- 13:27:54.2: the usual late `matches/<dm3>` (38 s after its end burst) started a ghost match load again, but this time lowkey went straight to **`not tracking abilitydraftarena - waiting for the next match`**, with no fetch or lookups. So the untracked check comes before any request, which is good. The ghost still ran, though, and the label only came out right because the queue it read from presence happened to be the untracked one
- the status line shows the raw queue ID `abilitydraftarena`, not a display name ("Glitch" in the client)
- presence went INGAME at 13:31:39 (`partyState: MATCHMADE_GAME_STARTING`), with map `/Game/Maps/AbilityDraft/AbilityDraft` at 13:31:49. As expected for an untracked mode, lowkey made no requests during the match
- back in MENUS at 13:43:46. The user picked Deathmatch (13:43:58), then Competitive (13:44:02), and waited in the menus for the patched build below
- **third ghost, and one the first fix wouldn't have caught:** a late RMS about the Glitch match at 13:44:32 made v0.1.0 fetch it, and it came back **`State: "IN_PROGRESS"` with 7 players, not including the user**. The match was still running without them. The roster's queue `abilitydraftarena` then put v0.1.0 into `not tracking abilitydraftarena - waiting for the next match` while the user was queued for Competitive. That would also have blocked the Competitive agent select, since `onPregame` ignores `phaseMatchLoad`
- last v0.1.0 reading at 13:57:26 (stopped then): CPU 10.02 s · 29 MB (**peak 51 MB**) · BelowNormal. Peak working set over the whole run was about half the 100 MB budget

## Patched build for the competitive game (v0.1.1 → v0.1.2)
- the user asked for the ghost bug to be fixed before a competitive game, and for a release. Go 1.26.10 was fetched as a portable zip into a temp folder (checksum verified against go.dev), with nothing installed system-wide
- code, on branch `fix/ghost-match-load`. The user squash-merged it as PR #1 into `main` (`07604e8`) at 17:22:
  - `7e789b9` fixed late events about a finished match starting a ghost match load that hid the next match:
    - `riot.Roster.Over`: core-game `State` is `CLOSED` or `POST_GAME`
    - `live.matchID`: the match ID from lifecycle events, taken only while presence is PREGAME or INGAME, so a late event can't claim it. `toMenus` moves it to `live.finished`, the last 8 matches
    - `event()` drops lifecycle events for finished match IDs, and `findMatchID` ignores a finished ID
    - `fetchRoster` sends an over match to `matchOver()`, which drops the load, stops the cutoff, returns to menus and re-applies the latest presence. So agent select or a real match that's already underway is picked up again via `findMatchID`
  - `1c95117` dropped match loads for a match you're no longer in, like one you left that's still going: `riot.Roster.Has`. A core-game roster without the user also goes to `matchOver()`. This covers the 13:44:32 case (`IN_PROGRESS`, 7 players, no user)
  - tests (`tests/app/ghost_test.go`): a late event about the last match; an event about an ended match during agent select; an event about a match you left (`IN_PROGRESS`, without you); an event about an unknown ended match in the menus. **All four fail on v0.1.0 and pass on the fix**
- test suite on this PC: everything passes except `TestGoldenScreens`, `TestGoldenStatusLines` and `TestTheFullDefaultColumnSetFitsIn120Columns`. Those three fail identically on unmodified v0.1.0 because the Windows checkout gives the golden files CRLF endings (`git ls-files --eol`: `i/lf w/crlf`). `TestLowkeyWaitsForTheTokenToBeReady` failed once in a full run, then passed 3 of 3 alone, so it's a timing flake
- tried and reverted: ignoring core-game events for any other match ID during agent select. The real game reuses the pregame match ID for core-game (TDM: same UUID), but fakeriot uses different IDs (`coreGame` versus the pregame ID), so 20 tests broke. Worth aligning fakeriot with the real game, and then this rule becomes the cheapest guard
- releases: v0.1.1 and v0.1.2 were first published at 13:57:49 and 14:00:51 from commits `f61a878` and `d5c0f48`. At 15:3x the user asked for the `Co-Authored-By: Claude` trailer to come off those two commits. They were rewritten as `7e789b9` and `1c95117` (messages only, identical trees), the branch was force-pushed, both tags moved, and both releases deleted so CI could republish them from the rewritten commits. CI republished v0.1.2 at 15:35:29 (`lowkey.exe` 7,624,704 bytes, `sha256:14edab963d07022239d948d5dccaf48f4971fe8c312f4bc2c8675712a3f2d467`) and v0.1.1 at 15:35:42 (`sha256:bf4ff758d17de287b51b32e414f093ab8a62a317a7853fe78fc84b88eb474e05`). Because v0.1.1 finished last, GitHub marked it "Latest", and that was put back on v0.1.2 at wrap-up. **v0.1.1 lacks the roster rule, so v0.1.2 supersedes it**
- the local build used for the game: `lowkey v0.1.2`, SHA256 `46444E3C6EFD78DAF4431ED0B6C0A2CB6ECBE23179EEE01F41836F9C9EBC64F3`, built here with Go 1.26.10 and `-trimpath -ldflags "-s -w -X main.version=v0.1.2"`. The CI release exe may differ byte-for-byte (different Go patch version)
- swap at 13:57:26–28: v0.1.0 stopped, renamed to `lowkey-v0.1.0.exe`, the new build copied in as `lowkey.exe`, started with `--record` (recording `lowkey-record-20261027-135729`). First frame at 13:57:29: `waiting for the next match`, `SESSION · 0W 0L +0 RR since 06:00`. Reading: CPU 0.14 s · 21 MB (peak 21 MB) · BelowNormal

### Warm-up · The Range again (v0.1.2)
- MATCH LOADING 14:04:51 as `The Range · Competitive` (the Range is still treated as a match, since the fix doesn't cover it), IN MATCH 14:05:31, back to menus 14:06:36

### Match 6 · Competitive (Bind, v0.1.2)
- frames: AGENT SELECT 14:07:59 · MATCH LOADING 14:09:11 · IN MATCH 14:09:51 · LAST MATCH 14:40:53
- core-game at load: `IN_PROGRESS`, 10 players, map `Duality`, queue `competitive`
- loss: SESSION went from `0W 0L +0 RR` to **`0W 1L -18 RR since 06:00`** in the next agent select
- **no ghost:** after the match the next agent select (14:42:02) was picked up 69 s after LAST MATCH

### Match 7 · Competitive (Sunset, v0.1.2)
- frames: AGENT SELECT 14:42:03 · MATCH LOADING 14:43:29 · IN MATCH 14:44:09 · LAST MATCH 15:12:27
- core-game at load: `IN_PROGRESS`, 10 players, map `Juliett`, queue `competitive`
- win: the final frame (15:12:42) shows `LAST MATCH · Sunset · Competitive`, two `AVERAGE` rows (3/5 and 4/5, both `Asc1 · Asc3 · Asc1 · 51%`), a `US - THEM` row, and **`SESSION · 1W 1L +2 RR since 06:00`**
- **no ghost:** a core-game message for the finished match at 15:12:28 started nothing. The only requests afterwards were the session refresh (`competitiveupdates`, plus `match-details` for that match). No frames after 15:12:42

### Match 8 · Competitive (Corrode, v0.1.2)
- frames: AGENT SELECT 15:36:42 · MATCH LOADING 15:38:18 · IN MATCH 15:38:58 · LAST MATCH 16:20:51
- win: `SESSION · 2W 1L +20 RR` at the next agent select
- **no ghost:** the next agent select was picked up at 16:24:22

### Match 9 · Competitive (Summit, v0.1.2)
- frames: AGENT SELECT 16:24:22 · MATCH LOADING 16:26:04 · IN MATCH 16:26:44 · LAST MATCH 17:04:04, then `SESSION · 3W 1L +38 RR since 06:00` after the refresh at 17:04:19
- then Valorant closed: `waiting for Valorant` 17:04:43, `waiting for the Riot Client` 17:04:49, `waiting for the Riot Client to log in` 17:04:51

### Competitive on v0.1.2, all four games
| game | agent select requests (of which pregame roster refreshes) | match-load requests | last match-load request after MATCH LOADING |
|---|---|---|---|
| Bind | 53 (16) | 57 | +10.1 s |
| Sunset | 45 (21) | 52 | +8.8 s |
| Corrode | 42 (18) | 65 | +11.7 s |
| Summit | 50 (13) | 17 | +3.2 s |
- all **410** remote requests in the v0.1.2 run returned 200: no 429, no 5xx, no rate-limit or unreachable status line all day
- no ghost match loads after any of the four games, so v0.1.2 fixes the bug in real play
- the user's after-match answers (dodges, table against the game, markers) were never given

### Loading times (ShooterGame.log)
- the log's bracket timestamps are **UTC**: lowkey's 13:05:30 MATCH LOADING matches `[2026.10.27-11.05.30:634]` `Broadcasting state changed to TransitionToInGame`, and the local zone is UTC+02:00
- no `BuyPhase` or `Round` lines. The marker is this line, which never holds names or IDs:
  - `[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogShooterGameState: Warning: Gameplay started at local time <seconds> (server time <seconds>)`
  - nearby, `LogGameFlowStateManager: [PlatformTime: <s>] Broadcasting state changed to <Pregame|TransitionToInGame|InGame|TransitionToMainMenu|MainMenu>`. `TransitionToInGame` lines up with lowkey's MATCH LOADING to the second
- how to read it, from the competitive games: each round logs it twice. Once at **server time 0** (that round's buy phase starts), and once at **server time 30**, or **45** in the pistol rounds 1 and 13 (combat starts). Round 1 has no server-time-0 line, so **round 1's buy phase starts at the combat line's time minus its server time**. The `PreRoundShopContainer` screen opening and closing confirms the buy phases
- per match, from lowkey's first MATCH LOADING frame:

| match | round 1 buy phase (or pre-round) starts | combat starts | lowkey's 40 s cutoff |
|---|---|---|---|
| the Range | – | +15 s | after combat |
| Deathmatch 1 (from the real match's first RMS, 12:55:56.7) | +11 s | +21 s | after combat |
| TDM | +16 s | **+36 s** | 4 s after combat |
| Deathmatch 2 and the missed Plummet Deathmatch | no `Gameplay started` line, probably joined in progress. `InGame` at +18 s | – | – |
| Competitive · Bind | +17 s | +63 s | in the buy phase, 23 s before combat |
| Competitive · Sunset | +17 s | +63 s | same |
| Competitive · Corrode | +16 s | +62 s | same |
| Competitive · Summit | +17 s | +63 s | same |
- verdict on the 40 s cutoff: **right for round-based modes**. In competitive it lands inside the 45 s pistol-round buy phase, about 23 s before combat, and the lookups had finished by +3 to +12 s anyway. **Too late for Deathmatch and TDM**, where combat starts at about +21 s and +36 s. No requests actually went out during play (the last match-load request in TDM was at +8 s, in Deathmatch 2 at +14 s), but the guard window was still open. Options: per-mode cutoffs (about 15 s for Deathmatch, 30 s for TDM), or close the window as soon as the match-load lookups finish
- the "local time" in the line is seconds since the client entered the game state, not a wall-clock time

## Appendix probes

### 02 · Lockfile and local API certificate (probes A and B)
- status: done (run during match 5, local only, no request to Riot's servers)
- A: the lockfile is at `%LOCALAPPDATA%\Riot Games\Riot Client\Config\lockfile` with **5** colon-separated fields: name `Riot Client`, port `49884`, protocol `https`. The password isn't reported
- B: a TLS connection to `127.0.0.1:49884` serves **1 certificate** (the leaf only):
  - subject `CN=rclient`
  - issuer `C=US, ST=California, L=Santa Monica, O=Riot Games, OU=LoL Game Engineering, CN=LoL Game Engineering Certificate Authority, emailAddress=gametechnologies@riotgames.com`
  - signature **sha256WithRSAEncryption**
  - SAN `DNS:localhost, IP Address:127.0.0.1`
  - valid 2026-01-04 22:51:53 GMT to 2125-12-11 22:51:53 GMT (100 years)
- the chain leads to **LoL Game Engineering Certificate Authority**. `openssl s_client -CAfile internal/riot/riotgames.pem` gives `Verify return code: 0 (ok)`, depth 0 `CN=rclient`, depth 1 the Riot CA. That root (lowkey's bundled copy) is signed **sha1WithRSAEncryption**, valid 2013-12-04 to 2043-11-27. Only the root's self-signature is SHA-1, so Go's normal verification works and the SHA-1 fallback isn't used on this client. This matches doctor's PASS

### 04 · Game version sources (probe D)
- status: done
- ShooterGame.log `CI server version:` → `release-13.08-shipping-22-5760300`
- presence `partyClientVersion` (doctor's client version line, "from presence") → `release-13.08-shipping-22-5760300`, the same as the log
- valorant-api.com `/v1/version` `riotClientVersion` → **`release-13.08-shipping-20-5715960`** (`version 13.08.00.5715960`, `buildDate 2026-10-15T22:27:44Z`). **This is behind the live client** by two shipping builds, so a hotfix isn't there yet. lowkey only falls back to it when presence and the log both fail, and then it would send a stale `X-Riot-ClientVersion`
- all three match `release-NN.NN-shipping-N-N`

### 05 · Riot servers (probe E, step 1)
- status: partial. Answered from lowkey's own recording of the same request (`GET pd /mmr/v1/players/<self>` in `lowkey-record-20261027-124517`, HTTP 200) rather than a second request to Riot's servers. The recording keeps bodies, not headers, so **rate-limit header names aren't known**. No 429 appeared all session
- top-level keys: `DerankProtectedGamesRemaining, DerankProtectedStatus, IsActRankBadgeHidden, IsAtDerankProtectedTier, IsLeaderboardAnonymized, LatestCompetitiveUpdate, LifetimePrestige, NewPlayerExperienceFinished, OnboardingFlowV2Enabled, OnboardingStatus, QueueSkills, Subject, Version`
- `QueueSkills` queues: `abilitydraftarena, competitive, deathmatch, dodgeball, fortcollins, ggteam, hurm, newmap, onefa, premier, seeding, skirmish2v2, skirmishascension1v1, snowball, spikerush, swiftplay, unrated, valaram`
- `QueueSkills.competitive` keys: `CurrentSeasonGamesNeededForRating, SeasonalInfoBySeasonID, TotalGamesNeededForLeaderboard, TotalGamesNeededForRating`. 30 seasons are listed
- a `SeasonalInfoBySeasonID` entry's keys: `CapstoneWins, CompetitiveTier, GamesNeededForRating, LeaderboardRank, NumberOfGames, NumberOfWins, NumberOfWinsWithPlacements, Prestige, Rank, RankedRating, SeasonID, TotalWinsNeededForRank, WinsByTier`
- `WinsByTier` is present, not null, in 30 of 30 entries
- `LatestCompetitiveUpdate` keys: `AFKPenalty, CompetitiveMovement, IsPlacementMatch, MapID, MatchID, MatchLength, MatchStartTime, NewMapIncentiveRRForgiven, QueueID, RRPenalty, RankedRatingAfterUpdate, RankedRatingBeforeUpdate, RankedRatingEarned, RankedRatingPerformanceBonus, RankedRatingRefundApplied, SeasonID, TierAfterUpdate, TierBeforeUpdate, WasDerankProtected, WasDerankProtectionReplenished`
- step 2 (content) wasn't needed, since `me` worked. The content response is in the `me` recordings anyway

### 03 and 06 · local API shapes and live events (probes C and F)
- status: skipped. No doctor line failed, and lowkey reacted to agent select and match load when no ghost was in the way. lowkey's recordings already hold the presences, external-sessions, pregame, core-game and RMS traffic these probes would have produced
- one observation from the recordings for 06: all 207 RMS events in the live recording (up to 13:29) came under `/riot-messaging-service/v1/message/…`, and none under `/messages/`, so there were no `/message/`+`/messages/` pairs

## Wrap-up (18:08–18:15)
- last reading of the v0.1.2 run, at 18:08:35 after 4 h 11 m: **CPU 24 s · 35 MB (peak 67 MB) · BelowNormal**. That's about 0.16% of one core averaged over the run, mostly in agent select and match load, and the peak is two-thirds of the 100 MB budget. The v0.1.0 run peaked at 51 MB. lowkey was still open, showing `waiting for the Riot Client to log in`
- `lowkey.log` and `lowkey.log.old`: neither exists. No request failed all day, so there was nothing to scrub
- `recordings.zip` (19.2 MB, from 208.5 MB raw) holds all five recordings: doctor `124517`, me `124556` and `124727`, live v0.1.0 `124811` and live v0.1.2 `135729`. The v0.1.2 one was still being written, so it's a snapshot from 18:11 cut at the last complete line
- `me-after.txt` was skipped, and the FPS/stutter/input question wasn't answered (see items 07 and 09)
- privacy check before the push: no UUIDs, Riot ID, PUUID, lockfile password or JWTs in the report or the `.txt` files, and no real Riot ID, PUUID, password or tokens in any recording. The user's Riot name matches their GitHub account name, so the account name is left out of item 01
- hand-over: by the user's request, everything for the Mac is committed to `sessions/2026-10-27-windows/` on `main` instead of a Desktop zip

## Summary
**Worked**
- `doctor` passed every check against the real Riot Client, and `me` worked for competitive and unrated
- once the ghost was fixed, live mode followed every phase: agent select (your team plus the roster refresh every 3 s), match load (both teams, averages, US - THEM), the cutoff, LAST MATCH, and the SESSION refresh. That held for 4 competitive games, TDM and Deathmatch
- streamer-mode players showed as `hidden` and were never looked up. The user is in streamer mode themselves and was still shown and looked up correctly
- 667 requests to Riot's servers over the day (doctor 1, me 17 + 4, live v0.1.0 235, live v0.1.2 410), every one a 200, with no rate limiting at all. None went out more than 45 s after the client went in-game. In competitive everything finished before combat. A Deathmatch joined in progress, like Deathmatch 2, may overlap its lookups (+0 to +14 s) with live play
- light on the machine: below-normal priority, EcoQoS on, 24 s of CPU in 4 h, peak 67 MB

**Failed or needs work** (most important first)
1. **ghost match loads** after a match ended, or after leaving one early, hid the next match: 2 Deathmatches missed and LAST MATCH wiped. **Fixed in v0.1.2** (PR #1, merged), with 4 regression tests and confirmed in 4 real competitive games
2. **Riot's match details now come with empty `gameName`/`tagLine`**, so every PLAYER cell is blank (or `-`), which makes the rows hard to tie to people. fakeriot still sends names, so the tests miss it
3. **the 40 s cutoff runs past combat start in Deathmatch (+21 s) and TDM (+36 s)**. It's fine for competitive (+63 s)
4. **the Range is treated as a match** (`ProvisioningFlow: ShootingRange`, no `MatchmakingData`) and runs lookups
5. the untracked status shows the raw queue ID (`not tracking abilitydraftarena`) rather than "Glitch"
6. the guard's `ownPartyRead` allows any party ID, and the brief's "never calls party endpoints" doesn't match lowkey's two own-party reads
7. a stale lockfile gives a `connectex … refused` FAIL with no hint. `static data … (1 days old)` grammar
8. on a Windows checkout three golden tests fail on CRLF line endings (a `.gitattributes` with `testdata/** -text` would fix it), and `TestLowkeyWaitsForTheTokenToBeReady` is timing-flaky
9. valorant-api.com's version lagged the live client by two shipping builds. It's only a fallback, but it would send a stale client version

**Surprises**
- Riot sends a lone late core-game message 30–38 s after every match, and for a match you left early, the real end arrives minutes later while you're queueing for the next one
- the real game reuses the pregame match ID as the core-game match ID. fakeriot doesn't, which blocked the simplest guard
- the user plays true stretched at 1280×880 with both monitors disabled in Device Manager. Windows still showed two screens and lowkey's window stayed usable on the second one
