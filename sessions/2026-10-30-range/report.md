# lowkey session report · 2026-10-30 · queueing from the Range

- operator: Claude Code (Opus 5.5) on the Windows PC. Times are local (UTC+02:00) unless they say UTC
- what changed in Valorant: you can now queue from inside the Range, and the match pops while you're still in it. You never pass the menus
- what was run: lowkey v0.1.2 (`bin/lowkey.exe`, built 2026-10-27) with `--record` from 00:16:55, plus a raw capture of every local Riot Client event (`OnJsonApiEvent`) from 00:17:33. The raw capture is unscrubbed and stays off the repo
- what was played: the Range, then **Team Deathmatch** (`hurm`, Piazza) queued from inside the Range, then back to the menus. A second Range load at 00:31:52 had no queue. Deathmatch can't be queued from the Range, so the only path out of the Range into a match goes through agent select
- `recordings.zip`: `lowkey-record-20261030-001655` (171 records plus `screens.txt`). lowkey scrubbed it itself, except that every presence's `packedData` was blanked to `<scrubbed>` by hand before zipping (see "Scrubber gap" below)

## What lowkey v0.1.2 showed

| time | screen |
|---|---|
| 00:16:55 | `waiting for the next match` |
| 00:18:01 | `MATCH LOADING · The Range · Unrated`, one row (you, Phoenix), lookups for core-game, party ×2, content, mmr, `match-history?queue=unrated` |
| 00:18:41 | `IN MATCH · The Range · Unrated` (the 40 s cutoff) |
| 00:27:41 | `waiting for the next match` over `LAST MATCH · The Range · Unrated` |

**The TDM was missed completely.** There was no agent select and no match load, and no request ever touched its match. From lowkey's point of view, the Range lasted 9½ minutes.

## The event timeline (raw capture, your own presence and lifecycle messages)

```
00:18:01.044  RMS core-game matches/<range> ×5, players/<self>, session     presence still MENUS
00:18:02.360  presence INGAME · provisioningFlow ShootingRange · RangeV2
00:18:41.207  presence INGAME · ShootingRange · queueId hurm · partyState MATCHMAKING   <- queueing from the Range
00:19:01.856  RMS pregame players/<self>, core-game matches/<range>, session
00:19:01.871  ShooterGame.log: "Loopstate changed from INGAME to PREGAME"
00:19:01.897  RMS pregame matches/<tdm>                                        presence still INGAME · ShootingRange
00:19:02.276  presence PREGAME · provisioningFlow Invalid · partyState DEFAULT          <- no MENUS in between
00:19:07.377  ShooterGame.log: InGame -> TransitionToPregame (you sat in the Range 5.5 s into agent select)
00:19:08.213  RMS core-game matches/<range> + core-game/v1/matchterminated (new resource, ends the Range)
00:19:09.830  presence provisioningFlow Matchmaking · matchMap HURM_Yard (Piazza)
00:19:48.225  RMS core-game matches/<range>, 46 s after the switch
00:20:29.150  RMS core-game matches/<tdm> burst · log PREGAME -> INGAME
00:20:31.794  presence INGAME
00:27:34-39   end burst (match-details, mmr, xp, contracts, core-game ×10)
00:27:41.869  presence MENUS
00:28:10.018  RMS core-game matches/<tdm>, 28 s after MENUS (the known late event)
```

Compared with the old flow (2026-10-27: Range → menus → Deathmatch), there are three differences:

1. **Presence goes `INGAME → PREGAME` with no `MENUS` in between.** The queue (`queueId`, `partyState MATCHMAKING`) shows up while the presence still says `INGAME` with `provisioningFlow ShootingRange`
2. **Lifecycle events for the next match arrive while the presence still says `INGAME`.** The pregame event for the TDM came 0.4 s before the `PREGAME` presence, so "an event while your presence is in a match names your match" no longer holds in the Range
3. **The Range keeps sending events after the next match has started**: 6.3 s later with `matchterminated`, and again 46 s later

The Range is easy to recognise in two places: the presence's `provisioningFlow` is `ShootingRange`, and so is its core-game match's `ProvisioningFlow` (along with `MatchmakingData: null`, one player and `IsReconnectable: false`). The core-game answer came back 100 ms after the first event, well before the presence caught up: 1.3 s later on the first load and 3.8 s on the second.

## Why lowkey missed it

The 00:18:01 events started a match load for the Range from the menus. 40 s later the cutoff moved lowkey to the in-round phase. Nothing gets lowkey out of the in-round phase except a `MENUS` presence:

- `onPregame` does nothing while the phase is in-round
- `applyPresence` only acts on `PREGAME` from the menus or while waiting
- `onCoreGame` does nothing while the phase is in-round

HEAD (the ghost fix, 07604e8) behaves the same, and it would also have recorded the TDM's pregame event as "your match" while you were still in the Range.

## The fix

**lowkey now treats the Range as the menus**, since it's where you queue from now:

- `riot.Private` reads `provisioningFlow` (falling back to `partyOwnerProvisioningFlow`), and `InRange()` is `INGAME` plus `ShootingRange`
- `riot.DecodeCoreGame` sets `Roster.Range` from `ProvisioningFlow`. A match load whose roster turns out to be the Range is dropped like an ended match (`matchOver`). That finishes the Range's match ID, so every later event about it is ignored
- a presence that changes *into* the Range sends lowkey to the menus. Once lowkey has moved on to the next match, a presence that *still* says Range is ignored as late. That matters because the presence lags the events by up to 4 s
- `presenceInMatch()` is false in the Range, so events about your next match can't be mistaken for the Range's
- `doctor` says `In the Range` rather than `In match`

What you'll see: in the Range, `waiting for the next match` with your last real match and the session line. There's a brief `MATCH LOADING` while lowkey checks the Range's match, the same as for a late ghost event. The Range no longer takes the LAST MATCH slot, and no rank, history or match-details lookups run for it. The only requests at Range load are one core-game fetch plus your own party reads, where before there were also content, mmr, match history and match details.

Replaying this session's order against the fix: the Range's first events start a match load, the roster comes back as the Range, and lowkey drops it and finishes its ID. The Range presence changes nothing. The queue presence changes nothing. The Range's event at 00:19:01.865 is dropped as finished. The TDM's pregame event starts agent select from the menus. The later Range events are dropped, the TDM's core-game events start match load, and `MENUS` brings up `LAST MATCH · Piazza · TDM`.

Tests (`tests/app/range_test.go`): the Range counts as the menus; a match queued from the Range is tracked (this session's exact event order); opening lowkey in the Range waits for the next match; `doctor` names the Range.

- all four fail on HEAD without the fix, and pass with it (3 runs in a row)
- `go vet ./...` is clean, and the full suite passes (`tests/app` and `tests/e2e`)
- the run used go1.27.3 from a portable zip, since Go isn't installed on this PC
- **Windows checkout gotcha**: with `core.autocrlf=true` and no `.gitattributes`, the golden files check out with CRLF, and 17 golden tests then fail on a clean HEAD too. The suite passes once they're LF. A `.gitattributes` with `tests/app/testdata/golden/* -text` (or `* text eol=lf`) would fix it for good
- the Range's match load is dropped through `matchOver`, which cancels outstanding lookups. That includes a menus session refresh still in flight, so a Range loaded within ~2 s of lowkey starting leaves the SESSION line empty until the next match ends. Ghost events already behave this way

## Scrubber gap

Presences now carry `packedData`, base64 JSON with each friend's party ID (`rprn:party:valorant:<uuid>`), an invite action (`rprn:requestinvite:ags:<code>`) and their current map and mode. The scrubber decodes `private` but not `packedData`, so these went into `record.jsonl` unchanged. Your own party ID appeared 13 times. lowkey doesn't read `packedData`, so it was blanked in this session's zip. The scrubber should either decode it like `private` or drop it.

## Not covered

- a live run of the fixed build. It's checked against fakeriot replaying this session's event order, not yet against the real client
- dodging an agent select you queued for from the Range. Riot may send you back to the Range or to the menus: lowkey handles both, but neither was recorded
- party play: `partyOwnerProvisioningFlow` is only a fallback here. A party member in the Range while the owner isn't wasn't seen
