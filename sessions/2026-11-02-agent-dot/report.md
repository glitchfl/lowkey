# lowkey session report · 2026-11-02 · the agent dot

- operator: Claude Code (Opus 5.5) on the Windows PC. Times are local (UTC+02:00) unless they say UTC
- what was run: `lowkey v0.1.10-1-ge037b36` (the `lowkey-windows` artifact from CI run 41431555089, the tip of `agent-dot`) with `--record` from 13:14:14, from its own folder (`%USERPROFILE%\lowkey-dot`), plus PresentMon 2.6.0 on `VALORANT-Win64-Shipping.exe`
- **the session was cut to 20 minutes.** What ran: the dot was placed in a custom game, then **one Swiftplay match** with the dot on. Match 2 (wait 15 s before picking) and match 3 (Windowed) were not run
- the match was found at 13:17:49, before the match-1 instructions had reached the user, so they picked the way they normally do (an instalock)
- `recordings.zip`: `lowkey-record-20261102-131415` (`record.jsonl` + `screens.txt`), scrubbed by lowkey. Checked before committing:
  - none of the 61 real player and match IDs in Valorant's log appear in it
  - every name is `playerN`/`self`
- `extras/presentmon/`: per-second summaries of both captures. The raw CSVs (24 MB and 88 MB) stay on the PC in `%USERPROFILE%\lowkey-dot\for-the-mac`
- `extras/scripts/`: the scripts that made the summaries. PresentMon's `--date_time` stamps ran exactly 2 h ahead of local time here; the scripts shift them back

## Setup

| | |
|---|---|
| version | `lowkey v0.1.10-1-ge037b36` |
| doctor | **no FAILs**, 12/12 PASS. eu/eu, client `release-13.09-shipping-18-5804228`, priority below normal, EcoQoS on, console 120 wide with ANSI off |
| monitors | DISPLAY1 (primary) 1280x880 at (0,0) · DISPLAY2 2560x1440 at (1280,0) · **both 100%** scale · primary ~180 Hz (5.55 ms between display changes) |
| Valorant | Windowed Fullscreen (`FullscreenMode=1`), 1280x880, on the primary monitor |
| PresentMon | `PresentMon-2.6.0-x64.exe`, ran without elevation because the user is in Performance Log Users. Flags: `--process_name --output_file --stop_existing_session --no_console_stats --date_time --session_name lowkeydot`. Stopped with `--terminate_existing_session --session_name lowkeydot` |

## Placing (custom game)

```
13:14:49.244  dot shown  placed      (756, 532)   <- in the menus; the custom's agent select only started at 13:15:06
13:14:52.247  dot gone   time limit  (756, 532)
13:15:06      custom game agent select; screens.txt: "not tracking custom games - waiting for the next match"
13:15:21.641  dot shown  placed      (72, 373)    <- in the custom's agent select
13:15:24.642  dot gone   time limit  (72, 373)
```

- saved: `agent_dot_at = [72, 373]`, `agent_dot_seconds = 10`
- the user's answers on placing: "looks good, yea", then "worked well". They didn't say where on the icon the dot sat.
- **`agent_dot = false` after placing.** The user had been asked to turn it on before placing. The first placement happened in the menus, so the Enter that was meant for `agent dot` probably landed on `place dot`. Placing works with the dot off and says nothing about it. The user turned it on at 13:18.
- PresentMon (placing capture):
  - Composed: Flip for the whole 3 s of placement 2
  - back to Hardware: Independent Flip at 13:15:24.659, **17 ms after the dot went**
  - placement 1 can't be judged: the game was already composed for the whole minute before, while the user was working in lowkey's window

## Match 1 · Swiftplay · Windowed Fullscreen

```
13:17:49.059  RMS ares-pregame players/<self>                     (record.jsonl event; ShooterGame.log has it at the same ms)
13:17:49.065  log: Loopstate changed from MENUS to PREGAME
13:17:49.091  DOT SHOWN  match found  (72, 373)                    +0.03 s
13:17:49.097  PresentMon: Hardware: Independent Flip -> Composed: Flip   (6 ms after the dot)
13:17:49.200  first remote GET /pregame/v1/matches/<id>: character_select_active, PhaseTimeRemainingNS 89.87 s, nobody selected
13:17:54.661  log: MainMenu -> TransitionToPregame               +5.60 s   (end of the match-found screen)
13:17:55.662  log: Beginning travel to /Game/Maps/PregameV2/CharacterSelectPersistentLevel
13:17:55.841  log: TransitionToPregame -> Pregame                +6.78 s   ("Pregame Load Times ... Total load time: 1.18 seconds")
13:17:59.092  DOT GONE  time limit                               +10.03 s
13:17:59.592  PresentMon: Composed: Flip -> Hardware: Independent Flip   (0.50 s after the dot)
13:18:01.835  log: Pregame_SelectCharacter POST (the user's click)  +12.78 s   <- first pick by anyone on the team
13:18:01.843  RMS ares-pregame matches/<id>
13:18:01.921  remote GET pregame: ally slot 5 (self) "selected"    (lowkey saw the pick 86 ms after the click)
13:18:02.389  log: Pregame_LockCharacter POST                     +13.33 s
13:18:04-13   the other four allies select and lock
13:18:37.772  pregame: character_select_finished
13:18:40.738  log: Pregame -> TransitionToInGame
13:18:57.088  log: TransitionToInGame -> InGame
13:19:04.990  log: Opening WBP_Screen_PreRoundShopContainer_C
```

- **The dot went by its time limit 2.7 s before the user could click.** The user's feedback, which came in right after this match-found: "worked well, only thing is I would make the dot like longer (more like 15 seconds)".
- The "picked an agent" exit was **not exercised live**, because the dot was already gone. lowkey did see the pick 86 ms after the click (an RMS event, then a GET), so it would have had the information to hide the dot fast.
- The dot was never up during agent select's later part, match load or any round. There were 6 `dot` entries in total, and the only one at a match found is the one above.

### The number this session is for

| | seconds after the pregame event |
|---|---|
| match-found screen ends (`TransitionToPregame`) | 5.60 |
| agent select screen loaded (`Pregame`) | 6.78 |
| dot gone at the 10 s default | 10.03 |
| **first click on an agent by anyone on the team (the user, an instalocker)** | **12.78** |
| lock | 13.33 |

**Best single value: ~12.8 s** from the pregame event until agent select could be clicked. Treat it as an upper bound: the log shows no "clickable" line between 6.78 s (screen loaded) and 12.78 s (first click). Nothing happens in that window, and no ally picked before the user. It's one match, in Swiftplay.

**Suggested default for `agent_dot_seconds`: 15 s.** That covers 12.8 s plus a couple of seconds to react, and it matches what the user asked for. The pick exit hides the dot sooner anyway.

### PresentMon, match 1

| phase | present mode | median ms between presents | p99 | max |
|---|---|---|---|---|
| menus before the match found (13:16:28-13:17:49) | mostly Hardware: Independent Flip (57,076 frames), some Composed: Flip (15,507) | 1.05 | 2.26 | 26.4 |
| **dot up**, match-found screen (13:17:49.09-54.66) | **Composed: Flip** (4,722 of 4,723) | 1.11 | 2.51 | 10.2 |
| **dot up**, transition (54.66-55.84) | Composed: Flip | 1.02 | 1.78 | 10.0 |
| **dot up**, agent select (55.84-59.09) | Composed: Flip | 1.64 | 13.0 | 274.7 |
| agent select after the dot (13:17:59.09-13:18:40.74) | Independent Flip from 59.592, Composed again 13:18:04.24-13:18:34.49 | 1.98 | 3.59 | 640.8 |
| match load (13:18:40.74-57.09) | Hardware: Independent Flip only | 1.61 | 2.98 | 106.8 |
| first 90 s in game | mostly Independent Flip (21,635), Composed (3,375) | 3.51 | 5.57 | 378.7 |
| rest of the match | mostly Independent Flip (157,481), Composed (21,050) | 3.97 | 6.53 | 204.0 |

- **The dot switches the game to Composed: Flip.** The switch came 6 ms after the dot went up, and the game was back on Independent Flip 0.50 s after it went.
  - Frame pacing barely moved while the dot was up (median 1.05 → 1.11 ms).
  - Display latency did: `MsUntilDisplayed` was 4-10 ms while composed, against ~0.5 ms on Independent Flip.
  - That cost is confined to the match-found screen and the first seconds of agent select.
- **Two long frames fall inside or next to the dot. Neither is composition.** In both, the game was busy on the CPU (`MsCPUBusy` ≈ the gap) and spent only ~0.1 ms in `Present`:
  - 274.7 ms ending 13:17:55.939: the agent select map travel, which started at 13:17:55.662
  - 640.8 ms from 13:17:58.945 to 13:17:59.586: this began **146 ms before** the dot went at 13:17:59.092. It's probably agent select streaming in, but it's worth checking again in a second match.
- **During the match**, there were ten Composed: Flip spells from 0.7 s to 33 s long:
  - start times: 13:19:16.7, 13:20:14.7, 13:21:05.9, 13:21:51.6, 13:23:15.7, 13:23:31.4, 13:25:29.1, 13:28:19.3, 13:30:12.5, 13:31:42.9
  - **They're not lowkey.** No dot was up, and there's no lowkey activity of any kind within 1 s of any of them; lowkey made no requests at all from 13:19 to 13:34.
  - The cause is unknown: alt-tab, a notification or another overlay. The 13:18:04-13:18:34 spell in agent select has no lowkey activity at its start either.

## CPU and memory

| when | CPU time | working set | peak | priority |
|---|---|---|---|---|
| 13:32:34 (18 min up) | 4.47 s | 31 MB | 36 MB | BelowNormal |
| 13:39:23 (25 min up, just before it was closed) | 4.66 s | 31 MB | 36 MB | BelowNormal · 20 threads · 436 handles |

That's about 0.3% of one core on average over the session.

**`agent dot:` lines:** none. `lowkey.log` was never created, because it's only written on the first error.

## Anything that looked wrong

1. **The 10 s default is too short.** The dot went 2.7 s before the user's first click (see above).
2. **Placing works with the dot off, without saying so.** After the first pass, `agent_dot = false` with a spot saved, and the first placement landed in the menus. Placing could turn the dot on, or the `place dot` row could say the dot is off.
3. **Pregame polling stalled behind the stats burst.**
   - After the first pregame GET at 13:17:49.200, lowkey made no pregame GET until 13:18:01.921, a 12.7 s gap. It was busy with ~44 `match-details` GETs from 13:17:50 to 13:17:58.
   - The 3 s cadence (01.9, 04.9, 07.9 …) only started with the first pregame RMS event.
   - The dot's pick exit (`pickedAgent`, `internal/app/dot.go`) runs off the pregame lobby update. A pick made during such a gap would leave the dot up until the next pregame GET.
   - Once the burst was over, the RMS path was quick: the event came at 13:18:01.843 and the GET at 13:18:01.921.
   - Whether an RMS event that arrives during the burst gets its GET promptly wasn't tested, because nobody picked during it.
4. **Entitlements token fetched before every call.**
   - From 13:18:34 to 13:18:49, lowkey fetched `/entitlements/v1/token` before every one of its 72 remote calls. There were 2 fetches in the four minutes before that.
   - Doctor at ~13:13 said the token expired in 7 min, so it was 1-2 minutes from expiry. It looks like the refresh doesn't stick while the client keeps returning the near-expiry token.
   - Not dot-related.
5. **One 429.** `match-details` returned 429 at 13:18:48.923, during the core-game burst (~50 `match-details` GETs from 13:18:41 to 13:18:49). Not dot-related.
6. **Two long frames around the dot's exit.** These are the 274.7 ms and 640.8 ms frames above. They're game CPU work that started before the dot went, so they're probably not the dot, but they're worth re-checking.

## Not done (20-minute session)

- match 2: wait ~15 s before picking, to see the time-limit exit with the user watching it
- match 3: Windowed mode, with the dot re-placed
- the user's answers on where the dot sat on the icon, and on input feel during agent select. The user said only "looks good" / "worked well"
- the zip on the Desktop. The user asked for a commit and push instead
