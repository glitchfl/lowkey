# Test sessions

One folder per session on the Windows PC, named by date and what it was for. Each has a `report.md` to start from and a `recordings.zip` of lowkey's recordings.

- `2026-10-27-windows`: the first test against the real game, with `lowkey doctor` and `lowkey me` output
- `2026-10-27-discovery`: probing Riot's other endpoints to see whether performance scores can be had during a match. `prep/` holds the findings, brief, test items and tools that went into it, and `extras/` holds the raw responses, live timelines, scripts and screenshots that came out
- `2026-10-30-range`: queueing a match from inside the Range
- `2026-11-02-voice-probe`: read-only probe of the Riot Client's local voice-chat API from the main menu. It found no Valorant session. Holds the full `/help?format=Full` dump and the voice type definitions, with no recordings
- `2026-11-03-dev-test`: `dev` tested end to end before main: automated runs, then TDM and Competitive with every new feature on. It has the fix list for the Mac (the voice column has no data source, so it blocks the merge) and the voice probe redone in a live match. `extras/catalog/` maps what the Riot Client, Riot's servers and the game log expose in each phase, as field names only, and `extras/_tools/` holds the probes and scripts that made it

This repo is public, so real player IDs and names stay out. Recordings come out of lowkey's scrubber already. Anything captured another way needs the same scrubbing first, and game logs or screenshots that show names don't go in.
