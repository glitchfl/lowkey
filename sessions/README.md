# Test sessions

One folder per session on the Windows PC, named by date and what it was for. Each has a `report.md` to start from and a `recordings.zip` of lowkey's recordings.

- `2026-10-27-windows`: the first test against the real game, with `lowkey doctor` and `lowkey me` output
- `2026-10-27-discovery`: probing Riot's other endpoints to see whether performance scores can be had during a match. `prep/` holds the findings, brief, test items and tools that went into it, and `extras/` holds the raw responses, live timelines, scripts and screenshots that came out
- `2026-10-30-range`: queueing a match from inside the Range
- `2026-11-02-agent-dot`: the agent dot (`agent-dot` branch) placed in a custom game, then one Swiftplay match. It covers how long the match-found screen lasts before agent select can be clicked, and the game's present mode while the dot is up. `extras/` holds per-second PresentMon summaries and the scripts that made them

This repo is public, so real player IDs and names stay out. Recordings come out of lowkey's scrubber already. Anything captured another way needs the same scrubbing first, and game logs or screenshots that show names don't go in.
