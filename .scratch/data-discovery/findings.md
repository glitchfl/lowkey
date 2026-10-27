# What Riot's data holds that lowkey doesn't show yet

Status: needs-triage

Source: every response in the 2026-10-27 Windows session recordings (`sessions/2026-10-27-windows/recordings.zip`): 396 match details, 47 mmr, 58 competitive update pages, 79 pregame, 13 core-game, 12 own-party, 1,082 live events. Rerun it on new recordings with the scripts in `tools/`, each taking `record.jsonl` paths: `fields.py` maps every field of every endpoint and event, `temps.py` and `temps2.py` decode the hidden performance fields.

## Riot's hidden performance fields

Competitive match details carry `hasTempValues: true` and fields under made-up names, for every player. Correlated against stats computed from the same matches (2,560 player rows, 256 matches):

| field | meaning | evidence |
|---|---|---|
| `scores.TempValueF` | overall performance score, 0–500 | sets the grade exactly: every pass < 330 ≤ every merit < 420 ≤ every distinction |
| `scores.TempValueL.TempValueO` | grade: `pass`, `merit` or `distinction` | thresholds in `TempValueT.TempValueS` (0 / 330 / 420), scale `TempValueV`–`TempValueU` (0–500), `TempValueR` 250 |
| `scores.TempValueG` | combat score, 0–500 | r 0.96 with kills per round |
| `scores.TempValueH` | support score, 0–500 | r 0.69 with assists per round |
| `scores.TempValueL.TempValueM` / `N` | combat / support rating, `double_down`…`double_up` | r 0.96 with G, 0.95 with H |
| `scores.TempValueL.TempValueP.{damage,killImpact,deathImpact,trades}` | combat sub-ratings | each tracks A, I, J, B (r 0.89–0.95) |
| `scores.TempValueL.TempValueQ.{assists,plants,defuses,utilityUsage}` | support sub-ratings | each tracks C, E, K, D |
| `scores.TempValueA` | damage per round | r 0.97 with ADR |
| `scores.TempValueB` | trade involvement per round | r 0.59 trades + 0.53 traded |
| `scores.TempValueC` | assists per round | r 1.000 |
| `scores.TempValueD` | ability casts per round | r 0.999 |
| `scores.TempValueE` / `K` | plants / defuses per round | r 0.96 / 0.90 |
| `scores.TempValueI` / `J` | kill impact / death impact per round (J negative) | r 0.98 kills, −0.94 deaths per round |
| `stats.TempValueA` | total damage | r 0.98 |
| `stats.TempValueB` / `C` | trade kills / deaths that were traded | r 0.83 / 0.80 with a 5 s trade window |
| `stats.TempValueD` / `E` | plants / defuses | exact |

- distribution: pass 77%, merit 18%, distinction 5%. Distinction wins 84% of the time, pass 42%
- the grade didn't move RR in this sample: wins averaged +18.6 (pass), +19.6 (merit), +19.0 (distinction) over 123 games with both an update and details
- the names say Riot considers this temporary. Expect renames or removal, so anything built on it needs a fallback

## Already in what lowkey downloads (no new requests)

From the match details lowkey fetches for each player's sample:
- Riot's performance grade and score per match (above): an average PERF score, merit+ rate, and a fragger/support lean from G vs H
- trades and traded deaths (`stats.TempValueB/C`), KAST (computable from `kills[]`)
- clutches, aces and flawless rounds: `roundResults[].roundCeremony` (`CeremonyClutch`, `CeremonyAce`, `CeremonyFlawless`, `CeremonyCloser`, …) with `ceremonyPlayer`
- weapon habits: `roundResults[].playerEconomies[].weapon` per round and `kills[].finishingDamage.damageItem`, e.g. an Operator marker
- behaviour: `behaviorFactors.afkRounds`, `stayedInSpawnRounds`, `friendlyFireOutgoing`, `selfDamage`, per-round `wasAfk` / `wasPenalized`, update `AFKPenalty`
- `matchInfo.completionState`: `Surrendered` and `VoteDraw` alongside `Completed`
- `matchInfo.partyRRPenalties` per party (0.25 seen), so skill-disparity parties are visible
- `sessionPlaytimeMinutes` per player per match, meaning unconfirmed
- kill and plant positions with view angles (`playerLocations`, `plantLocation`, `plantSite`): site habits, heavy to use

From pregame and core-game, which lowkey already fetches:
- agent mastery level: `bonsaiBloomlineData.bloomlineShortLevel` (pregame) and `bonsaiBloomline.bloomlineShortLevel` (core-game), meaning to confirm in-game
- Premier team name and tag: `PremierPrestige.rosterName/rosterTag`
- act rank badge: `SeasonalBadgeInfo.Rank/NumberOfWins/WinsByTier`
- agent select: `CharacterSelectionState` (selected vs locked), `EnemyTeamLockCount`, `PhaseTimeRemainingNS`. `EnemyTeam` only arrives at `character_select_finished`, about 3 s before match load

From mmr:
- `LifetimePrestige.<TIER>.Count` and per-season `Prestige`, the 2026 prestige system
- `LeaderboardRank` per season, `NumberOfGames` per queue per season (how much someone plays DM, swiftplay, etc.)
- own derank protection: `DerankProtectedStatus`, `DerankProtectedGamesRemaining`, `IsAtDerankProtectedTier`

From your own party and presence:
- `MatchmakingData.SkillDisparityRRPenalty` for your party before you queue
- `Members[].Pings[]`: your ping to every game pod
- `QueueEntryTime`: time in queue
- friends' presence: `competitiveTier`, `leaderboardPosition`, map, mode and live score (`partyOwnerMatchScoreAllyTeam/EnemyTeam`, `packedData.activity.progress.value`)

## Off limits whatever the data says

- anything about an `Incognito` player beyond "hidden". Pregame and core-game carry their tier and level anyway
- a player's account level when `HideAccountLevel` is true, even though match details carry `accountLevel`
- replay downloads: `gdu/.../REPLAY` events carry presigned CloudFront URLs for each match's replay

## Not seen yet, needs the Windows probes

Live events show services lowkey has never called. Their endpoints are unknown:
- `cap/stats/v1/DeltaEvent` from `cap.lifetime-stats`: lifetime stat groups faceted by agent and act. If readable per player, one call could replace the 13 match downloads per player
- `ares-region-index/region-index/v1/players/<id>` from a service called `map-rating`, fired after each competitive game
- `ares-accolades/accolades/v1/players/<id>` and `ares-mastery/bloomline/v0/players/<id>/matches`

`windows-brief.md` covers these (Windows test items 11–13).

## Side findings

- lowkey's recorder doesn't scrub `preSignedUrl` in `gdu` events. The 2026-10-27 recordings in the repo hold expired replay URLs with signatures
- vRY also shows rank rating, so "the only tracker that shows current RR" doesn't hold
