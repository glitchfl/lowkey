# 02: Shapes of the new stats endpoints, own data only

**Why:** lowkey can only use a stats service once the Mac knows what it returns. The four services that matter most are lifetime stats (per agent and act), map rating, accolades and agent mastery. If lifetime stats can be read per player, one request could replace the 13 match downloads lowkey makes for each player today (`.scratch/data-discovery/findings.md`).

**Needs:** the Riot Client and Valorant in the menus; item 01 done

**Status:** open

## Steps

1. From `endpoints.md` and `local-api-help.json`, pick every endpoint about player stats, ratings, accolades or mastery that lowkey doesn't already call. lowkey already calls `mmr/v1/players`, `mmr/v1/players/.../competitiveupdates`, `match-history/v1/history`, `match-details/v1/matches`, `pregame`, `core-game`, the user's own two `parties` reads and `content-service/v3/content`.
2. Skip everything outside that, even if it's in the list:
   - name service, parties, invites
   - agent select, store, wallet, loadouts, contracts
   - restrictions, penalties, reports
   - leaderboards, which list other players' names
   - replay and `gdu` URLs
3. Send each remaining endpoint one `GET`, five or more seconds apart, at most 15 requests in all:
   - use the user's own PUUID wherever it takes a player, and one of the user's own match IDs from today wherever it takes a match
   - use the same host the game used, with these headers:
     - `Authorization: Bearer <accessToken>`
     - `X-Riot-Entitlements-JWT: <token>`
     - `X-Riot-ClientPlatform`: the base64 of `{"platformType":"PC","platformOS":"Windows","platformOSVersion":"10.0.19042.1.256.64bit","platformChipset":"Unknown"}`
     - `X-Riot-ClientVersion`: from doctor's client version line
   - if one needs anything else to answer, skip it and say what it asked for
4. For each endpoint, report:
   - status code
   - every key path with its value type, with lists collapsed to `[]`
   - list lengths
   - and, as values:
     - stat names
     - facet names, such as agent, act, map or queue, and how many of each
     - any enum-like strings, such as tiers, grades or ratings

   No other values.
5. For lifetime stats, also say whether it covers other modes than competitive, and how far back it goes.

## Send back

Per endpoint: the path shape, the status code and the shape, in the report.

## Results
