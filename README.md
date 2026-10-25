# lowkey

A small Windows console app that shows ranks and recent stats for everyone in your Valorant lobby, then goes quiet once the match starts.

```text
MATCH LOADING · Ascent · Competitive
PLAYER       AGENT    RANK      PEAK   LAST   WR    W-L    K/D  HS%  ACS  ADR  FB%  N  MAINS  TREND  MET
ALLIES
Yourself#ME  Sova     Dia3 12   Imm2   Dia2  57%   12-9   1.00    -  200    0   0%  6  Jett     +14
AllyOne#A1   Jett     Dia1 50   Dia1   -     50%  10-10   2.00  30%  200  150  10%  5  Jett      +5
AllyTwo#A2   Killjoy  Plat1 30  Plat1  -     60%    6-4   0.50  20%  150  100   5%  3  KJ       -10
hidden       Omen
-            Sage     -         -      -       -      -      -    -    -    -    -  0  -          -
AVERAGE 4/5           Dia1      Dia2   Dia2  56%          1.17  25%  183   83   5%               +3
ENEMIES
EnemyOne#E1  Raze     Plat2 10  Plat2  -     60%    6-4   2.50  50%  250  200  20%  2  Raze     +12
EnemyTwo#E2  Breach   Gold3 70  Gold3  -     50%    5-5   0.50  10%  100   80   0%  1  Brch      -6
hidden       Viper
-            Cypher   Plat3 0   Plat3  -     70%    7-3      -    -    -    -    -  0  -         +0
-            Skye     -         -      -       -      -      -    -    -    -    -  0  -          -
AVERAGE 4/5           Plat2     Plat2        60%          1.50  30%  175  140  10%               +2
US - THEM             ▲2        ▲3            ▼4         ▼0.33   ▼5   ▲8  ▼57   ▼5               ▲1
SESSION · 2W 1L +26 RR since 06:00
```

The names above are made up. On a terminal, ranks are colored by tier.

## Download and run

1. Download `lowkey.exe` from the [latest release](https://github.com/glitchfl/lowkey/releases/latest).
2. Put it in a folder of its own. lowkey keeps its settings file there.
3. Double-click it.

Windows may warn that the publisher is unknown, because the exe isn't signed. Choose **More info**, then **Run anyway**.

There's nothing to install and it doesn't need admin rights. Leave it open as long as you like: it waits at about 0% CPU for the Riot Client and Valorant, and follows the game from there. Close the window to quit.

## What it shows

- **In the menus:** "waiting for the next match", the table from your last match, and a session line with your competitive wins, losses and net RR since 6am.
- **At agent select:** your team, updating as people hover and lock, with a team average.
- **When the match loads:** allies and enemies, an average row for each team, and a US - THEM row with ▲ and ▼. Deathmatch shows one list.
- **At the cutoff, 40 seconds after the match loads:** lookups stop and the header changes to IN MATCH. Nothing more is sent until you're back in the menus.

| Column | What it shows |
|---|---|
| (no header) | ● for your party. ◆ ■ ◇ □ ★ mark likely parties: players on the same team in at least 2 of their recent matches. |
| PLAYER | Riot ID. "hidden" means streamer mode, and "-" means no recent match in this mode to take the name from. |
| AGENT | Their current agent, as they hover and lock. |
| RANK | Competitive rank and RR this act. |
| PEAK | Highest rank in any act, with old episodes mapped to today's ranks. |
| LAST | Final rank last act. |
| WR, W-L | Win rate and wins-losses this act, in the mode you're playing. |
| K/D, HS%, ACS, ADR, FB% | From their recent matches in this mode. FB% is the share of rounds where they got the first kill. |
| N | How many matches those numbers come from. |
| MAINS | Their two most-played agents in those matches. |
| TREND | Net RR over their last 5 competitive games. |
| MET | Whether you've played with or against them lately, for example "ally W 2h" or "enemy L 3d". |

Stats that don't apply to a mode show "-", like win rate in Deathmatch. If a window is too narrow for every column, columns drop from the end with a "+N hidden" note.

## Commands

Run these from PowerShell in the exe's folder, for example `.\lowkey.exe doctor`.

| Command | What it does |
|---|---|
| `lowkey` | Follows the game and shows lobby tables until you close the window. Same as double-clicking. |
| `lowkey me` | Prints your own row and session line once. Add `--mode unrated`, or any mode from the `modes` list under Settings, for another mode. |
| `lowkey doctor` | Checks this PC in a few seconds and prints PASS or FAIL for each item. Exits non-zero if anything failed. |
| `lowkey --record` | A normal run that also saves every Riot response in a folder next to the exe, with tokens removed and every player's ID and name replaced by a consistent fake. Works with `doctor` and `me` too. |
| `lowkey --version` | Prints the version. |

`doctor` checks the lockfile, the Riot Client's local API, your login tokens, the game phase, your region, the client version, one rank lookup for your own account, how old the built-in agent and map data is, the process priority, Efficiency mode, the console and the settings file.

## Settings

The first normal run creates `lowkey.toml` next to the exe, with a comment above every setting. Edits apply straight away without a restart. If something in the file is wrong, lowkey keeps using the last good settings and shows a warning saying what's wrong.

| Setting | Default | What it does |
|---|---|---|
| `columns` | all 17 | Which columns to show, left to right. Remove or reorder freely. |
| `matches` | `13` | How many recent matches per player are behind K/D through MAINS, from 1 to 25. |
| `modes` | competitive, premier, unrated, swiftplay, deathmatch, team_deathmatch | Modes to follow. In any other mode lowkey stays idle. |
| `session_reset_hour` | `6` | Hour of the day, in local time, when the session line starts over. |
| `cutoff_seconds` | `40` | Seconds after the match loads when every lookup stops. |
| `enemy_lookups` | `true` | `false` shows enemies by agent only and never looks them up. |
| `region` | `"auto"` | `auto` reads your region from the game. `na`, `latam`, `br`, `eu`, `ap`, `kr` or `pbe` overrides it. |

## What lowkey never does

- **Change anything.** It only reads. It never hovers or locks an agent, and never changes anything on your account.
- **Look up streamer-mode players.** They show as "hidden" and are left out of averages, party marks and MET.
- **Call Riot's name service.** Names come from match data lowkey downloads anyway.
- **Show starting sides.** Teams are only ever "allies" and "enemies".
- **Read anyone else's party.** Only your own party is looked up. Likely parties are guessed from match history.
- **Use the internet during rounds.** Everything stops at the cutoff. If you open lowkey mid-match, it waits for the next game.
- **Touch the game.** No injection, memory reading, screen capture or overlay. It runs at below-normal priority with Windows Efficiency mode on, and stays under 100 MB of RAM (about 30 MB in testing).

The only files it writes are next to the exe: `lowkey.toml`, `lowkey.log` after an error (with tokens and passwords removed), and recording folders when you use `--record`.

## Development

lowkey is written in Go and cross-compiled from macOS. Windows-only code (priority, Efficiency mode, console setup, Windows paths) sits behind build tags, so everything else builds and tests on any OS.

| Command | What it does |
|---|---|
| `make test` | Runs `go vet` and the test suite. |
| `make windows` | Builds `bin/lowkey.exe` and `bin/fakeriot.exe`. |
| `make e2e` | Runs the out-of-process scenarios against a local build. |
| `make doctor-e2e` | Runs `doctor` against the fakeriot program. |

Tests never talk to Riot. They run lowkey against **fakeriot**, a fake Riot Client (lockfile, local HTTPS API and WebSocket events) plus fake Riot servers that play scripted scenarios such as a full match, a dodge, rate limiting and a Riot Client restart. Every test checks fakeriot's request log against the rules above. Tests of the live screen also check that nothing went out while the screen said lowkey was waiting or the match had started.

CI runs the tests on Linux, then runs the real `lowkey.exe` against fakeriot on a Windows runner: `doctor`, the scenarios, and idle CPU and memory checks.

To release, push a tag like `v0.2.0`. CI builds `lowkey.exe` with that version, runs everything, and publishes a GitHub release with the exe attached.

| Path | What's in it |
|---|---|
| `cmd/lowkey` | The exe's entry point. |
| `cmd/fakeriot` | The fakeriot program for out-of-process runs. |
| `internal/app` | Live mode, `me` and `doctor`, plus the in-process tests. |
| `internal/riot` | The Riot Client's local API, its events and Riot's response formats. |
| `internal/remote` | Requests to Riot's servers, and the guard that refuses anything the rules forbid. |
| `internal/stats` | Ranks, sample stats and the session line. |
| `internal/render` | Turning a screen into terminal output. |
| `internal/settings` | `lowkey.toml`. |
| `internal/record` | `--record` and its scrubbing. |
| `internal/static` | Built-in agent, map and rank data. |
| `internal/platform` | Windows paths, priority, Efficiency mode and console setup. |
| `internal/fakeriot` | The fake Riot Client and servers. |
| `e2e` | The scenarios that run the real exe. |

---

lowkey isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone officially involved in producing or managing Riot Games properties. Riot Games and all associated properties are trademarks or registered trademarks of Riot Games, Inc.
