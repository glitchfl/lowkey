# lowkey test session brief · data discovery

You're the Windows test operator for **lowkey**, a tiny Windows console tracker for Valorant. It shows lobby stats using the Riot Client's local API and Riot's servers, and it never touches the game. Development happens on a Mac with no Riot software installed, so you're the only way to see the real thing.

This session is about finding data, not testing lowkey. The Mac found services in Riot's live events that lowkey has never called: lifetime stats per agent and act, a map rating, accolades and agent mastery. It also found Riot's own performance grade hidden in competitive match details. Your job:
- list every endpoint the game itself uses, from its log, without sending extra requests
- look at the new stats endpoints once each, with the user's own data only
- ask the user what the client shows about their performance after a match

This brief lives on the `data-discovery` branch, at `.scratch/data-discovery/windows-brief.md`, next to the Mac's `findings.md`. Read the findings only if a step leaves you unsure what to look for.

The numbers in parentheses, like (item 01), are this feature's Windows test items, in `windows-tests/` next to this brief. Use them in your report.

## Running the session

You run this session. The user plays Valorant and answers your questions. They haven't read this brief, so you tell them what to do at every point.

- Keep each message to the user short: one or two actions at a time, then wait until they say they're done.
- During a match, stay quiet. When they say they're back in the menus, ask your questions in one message.
- Plan around one Competitive match, about 45 minutes, plus 15 minutes in the menus. Competitive matters because only competitive matches carried the performance fields so far. If the user wants to keep playing, each extra match adds a census run and the post-match questions.

**Start here.** Note the time, then send the user one message that:
1. says what the session looks like: about 10 minutes clicking through a few client screens, one Competitive match (more if they like), then 5 minutes of wrap-up
2. tells them what to do now: open the Riot Client and Valorant, log in, and stay in the menus
3. mentions that you'll run a few commands during setup, and that approving them then keeps their games uninterrupted
4. asks them, after the match, to look through the end-of-match screens before leaving them, because you'll ask about them

## Rules

Riot is actively banning tools that reveal hidden players or automate agent select: more than 14,000 accounts in Act 5, in autumn 2026. Stay well clear of anything like that:

- **Requests.** Make only GET requests. Send one request at a time, five or more seconds apart, and at most 15 to Riot's servers in the whole session. No loops and no load testing.
- **Whose data.** On Riot's servers, look up only the user's own player ID. Fetch anything per match only with the user's own match IDs.
- **Endpoints you never use:**
  - the name service
  - anything that selects or locks an agent
  - party or invite endpoints
  - store, wallet, loadouts, contracts, restrictions, penalties and reports
  - leaderboards, which list other players' names
  - replay downloads and any `gdu` URL
- **The game process.** Never touch it: no reading its memory, no injecting into it, no overlays. Reading files Valorant writes to disk, like its log, is fine.
- **When in doubt.** If anything looks like it could get the account flagged, skip it and explain why.

## Privacy

- **Never send back:**
  - tokens
  - the lockfile password
  - Authorization headers
  - player IDs (PUUIDs)
  - Riot IDs (names and tags), whether the user's or anyone else's
  - URL signatures and query values (`Signature=`, `Key-Pair-Id=`, `Expires=`, any `token`)
- **Placeholders.** Replace them with placeholders like `<self>`, `<puuid-1>` and `<name-1>`.
- **Shapes, not payloads.** Report the shape of each response (keys and value types), not the full payload, unless a step asks for specific values.
- **lowkey's recordings** go back whole, after the one scrub in phase 3: lowkey's recorder keeps the presigned replay URLs that Riot pushes after each competitive match, and those must come out.

## How to work

- The commands below are PowerShell, run from `%USERPROFILE%\lowkey`. If your shell doesn't keep the directory between commands, start each one with `Set-Location "$env:USERPROFILE\lowkey"`. If your shell is Git Bash, save each snippet as a `.ps1` file and run it with `powershell.exe -NoProfile -ExecutionPolicy Bypass -File <file>`.
- Every local request goes to `https://127.0.0.1:<port>` with Basic auth: username `riot`, password from the lockfile at `%LOCALAPPDATA%\Riot Games\Riot Client\Config\lockfile` (`name:pid:port:password:protocol`). Certificate checks can be skipped for 127.0.0.1 only. `curl.exe -k` ships with Windows.
- Recordings and the game log can be large. Search them instead of reading them whole.
- Write your report as you go, so nothing is lost if this session gets interrupted.

## Your report and the hand-over

Everything goes back through the repo: one folder, `sessions/<YYYY-MM-DD>-discovery/` on the `data-discovery` branch, using the session's date. Nothing from this session goes on `main`. Build it in `%USERPROFILE%\lowkey\for-the-mac` first. By the end it holds exactly:
- `report.md`, kept up to date as you go
- `endpoints.md` (item 01)
- `local-api-help.json` (item 01)
- `recordings.zip`, made in phase 3

In `report.md`, give each item a section with its number and title, a status (done, partial or skipped, with the reason), your findings as short bullets, and any values or shapes the step asks for.

## The session at a glance

| When | What | Items |
|---|---|---|
| 0:00–0:15 | Setup, local `/help`, a tour of client screens, the first census, own-data probes | 01, 02 |
| 0:15–1:00 | One Competitive match, then the post-match questions and a second census | 03, 01, 02 |
| 1:00–1:05 | Wrap-up: scrub, zip, push | |

## Phase 1 · Setup and the census (about 15 minutes)

**Needs:** the Riot Client and Valorant open, logged in and sitting in the menus.

### 1. Start lowkey recording

1. `.\lowkey.exe --version` must print `lowkey v0.1.2`. If it doesn't, download it with `gh release download v0.1.2 --repo glitchfl/lowkey --pattern lowkey.exe --dir "$env:USERPROFILE\lowkey" --clobber`.
2. Start it in its own window:
   ```powershell
   Start-Process -FilePath "$env:USERPROFILE\lowkey\lowkey.exe" -ArgumentList "--record" -WorkingDirectory "$env:USERPROFILE\lowkey"
   ```
3. Tell the user a lowkey window opened, and to leave it open on the second monitor for the whole session. Its recording saves every response lowkey gets, including the match details the Mac needs for item 03.

### 2. The local API's own list (item 01)

`GET https://127.0.0.1:<port>/help`. Save the response as `for-the-mac\local-api-help.json`, after checking it holds no IDs, names or tokens.

### 3. A tour of the client (item 01)

Ask the user to go through these screens in order, staying about 5 s on each, then tell you they're done:
1. Career → Match History, then open one recent competitive match and click through every tab it has
2. any Career page that shows stats per agent, per map or for the whole act
3. Agents → their most-played agent → any mastery or progression page
4. Premier, without joining or changing anything
5. Leaderboards, without searching for anyone

Note the time they start and finish.

### 4. The census (item 01)

The game logs its own HTTP requests in `%LOCALAPPDATA%\VALORANT\Saved\Logs\ShooterGame.log`. Its bracket timestamps are UTC.

1. Find the lines that record requests. Try `Platform HTTP Query`, `QueryName`, `URL [` and `https://` first. Report the line format with IDs, names and query values removed.
2. Build `for-the-mac\endpoints.md`: one row per endpoint shape, with these columns:
   - query name, if the line has one
   - method
   - host
   - path, with every UUID as `{id}` and every numeric ID as `{n}`
   - query keys only, with no values
   - count, response codes, and the first time it was seen
   - which tour step first triggered it, from the times

   This is a starting point, so adapt it to the real line format:
   ```powershell
   $uuid = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
   $logs = Get-ChildItem "$env:LOCALAPPDATA\VALORANT\Saved\Logs" -Filter "ShooterGame*.log" | Where-Object LastWriteTime -gt (Get-Date).Date
   $hits = foreach ($f in $logs) {
     Select-String -Path $f.FullName -Pattern '(?:(GET|POST|PUT|PATCH|DELETE)\s+)?(https://[^\s\]\)"''<>]+)' -AllMatches | ForEach-Object {
       $line = $_.Line
       foreach ($m in $_.Matches) {
         $path, $query = $m.Groups[2].Value -split '\?', 2
         $shape = $path -replace $uuid, '{id}' -replace '/\d{3,}(?=/|$)', '/{n}'
         if ($query) { $shape += '?' + (($query -split '&' | ForEach-Object { ($_ -split '=', 2)[0] }) -join '&') }
         [pscustomobject]@{
           Shape  = $shape
           Method = $m.Groups[1].Value
           Name   = if ($line -match 'QueryName: \[([^\]]+)\]') { $Matches[1] } else { '' }
           Code   = if ($line -match 'Response Code: \[(\d+)\]') { $Matches[1] } else { '' }
           Time   = if ($line -match '^\[([\d.\-:]+)\]') { $Matches[1] } else { '' }
         }
       }
     }
   }
   $hits | Group-Object Shape | Sort-Object Name | ForEach-Object {
     $g = $_.Group
     [pscustomobject]@{
       Shape = $_.Name; Count = $_.Count
       Names = ($g.Name | Where-Object { $_ } | Sort-Object -Unique) -join ' '
       Methods = ($g.Method | Where-Object { $_ } | Sort-Object -Unique) -join ' '
       Codes = ($g.Code | Where-Object { $_ } | Sort-Object -Unique) -join ' '
       First = ($g.Time | Where-Object { $_ } | Select-Object -First 1)
     }
   } | Format-Table -AutoSize | Out-String -Width 400
   ```
3. Before saving, read the table for anything private that slipped through: Riot IDs in a path, tokens, signatures or long random strings. Replace each with a placeholder.
4. At the top of your report, call out the shapes that look like lifetime stats, map rating (the service is called `map-rating`, under `region-index`), accolades or mastery (`bloomline`).

### 5. Own-data probes (item 02)

1. From `endpoints.md` and `local-api-help.json`, pick every endpoint about player stats, ratings, accolades or mastery that lowkey doesn't already call. lowkey already calls `mmr/v1/players`, `mmr/v1/players/.../competitiveupdates`, `match-history/v1/history`, `match-details/v1/matches`, `pregame`, `core-game`, the user's own two `parties` reads and `content-service/v3/content`. Anything on the "never use" list stays out, even if the game calls it.
2. Get what you need from the local API:
   - `GET /entitlements/v1/token`: `accessToken`, `token`, and `subject` (the user's PUUID)
   - `GET /product-session/v1/external-sessions`: the region from `-ares-deployment`. Last session it was `eu`, with shard `eu`
3. Send each endpoint one `GET`:
   - use the user's own PUUID wherever it takes a player, and one of the user's own match IDs from Match History wherever it takes a match
   - use the same host the game used, with these headers:
     - `Authorization: Bearer <accessToken>`
     - `X-Riot-Entitlements-JWT: <token>`
     - `X-Riot-ClientPlatform`: the base64 of `{"platformType":"PC","platformOS":"Windows","platformOSVersion":"10.0.19042.1.256.64bit","platformChipset":"Unknown"}`
     - `X-Riot-ClientVersion`: from `.\lowkey.exe doctor`'s client version line
   - if one needs anything else to answer, skip it and say what it asked for
4. For each endpoint, report:
   - path shape and status code
   - every key path with its value type, with lists collapsed to `[]`
   - list lengths
   - and, as values:
     - stat names
     - facet names, such as agent, act, map or queue, and how many of each
     - any enum-like strings, such as tiers, grades or ratings

   No other values. For lifetime stats, also say which modes it covers and how far back it goes.

**Phase 1 is done when** `local-api-help.json` and `endpoints.md` are saved, the probes are in your report, and the user is queueing for Competitive.

## Phase 2 · The match (item 03, then 01 and 02 again)

1. During agent select and the match, stay quiet.
2. When the user is back in the menus, ask in one message:
   - any grade or rating word the end-of-match screens showed for them, such as Pass, Merit, Distinction or anything like it, and on which screen
   - any up or down arrows, or ratings, for damage, kill impact, death impact, trades, assists, utility, plants or defuses: which ones and which way
   - the RR change, and whether the client split it into parts, such as a performance bonus
   - the mastery level or progress shown for the agent they played, and whether agent select showed mastery levels for anyone
   - anything about a map rating or a per-map score
   - whether the client showed grades for other players too
3. Write their answers into the report word for word where they quote the client, with the match's end time, the mode and the agent they played. lowkey downloads that match's details within seconds of it ending, so the Mac can match it up.
4. Run the census again and add any new rows to `endpoints.md`, marked with the match. The end-of-match screens are where accolades, mastery and map rating are most likely to show up.
5. Probe any new stats endpoint from that run, as in phase 1 step 5, within the 15-request limit.
6. Tell the user they can play another match if they want. Repeat this phase after each one, or move on to the wrap-up.

## Phase 3 · Wrap-up (about 5 minutes)

1. Tell the user to close the lowkey window.
2. Scrub the presigned replay URLs out of the recordings, then check that none are left. The last line must print nothing:
   ```powershell
   Set-Location "$env:USERPROFILE\lowkey"
   $dirs = Get-ChildItem -Directory "lowkey-record-*" | Where-Object LastWriteTime -gt (Get-Date).Date | ForEach-Object FullName
   $files = Get-ChildItem -Path $dirs -Recurse -Filter record.jsonl
   foreach ($f in $files) {
     $t = [IO.File]::ReadAllText($f.FullName)
     $t = $t -replace '(\\"preSignedUrl\\":\\").*?(?=\\")', '$1<scrubbed>'
     [IO.File]::WriteAllText($f.FullName, $t)
   }
   $files | Select-String -Pattern 'Signature=|Key-Pair-Id=' -List
   ```
3. Zip only this session's recordings:
   ```powershell
   $paths = @(Get-ChildItem -Directory "lowkey-record-*" | Where-Object LastWriteTime -gt (Get-Date).Date | ForEach-Object FullName)
   Compress-Archive -Path $paths -DestinationPath "for-the-mac\recordings.zip" -Force
   ```
4. End your report with a short summary:
   - which new endpoints exist
   - which of them answered for the user's own data, and what they hold
   - what the client showed after each match
   - anything that surprised you
5. Check `for-the-mac` for anything private: search `report.md`, `endpoints.md` and `local-api-help.json` for UUIDs, for the user's Riot ID, and for `Signature=` and `token`. None should appear.
6. Hand it over through the repo:
   1. In your clone of `glitchfl/lowkey`, run `git switch data-discovery`, then `git pull --rebase`.
   2. Copy the four files into `sessions/<YYYY-MM-DD>-discovery/`.
   3. Commit on `data-discovery` with a one-line, lowercase, past-tense message and no prefix, like `added the data discovery session report and recordings`. Add no `Co-Authored-By` trailer and no other attribution.
   4. Push the branch.
   5. Tell the user the session is done and the report is on `data-discovery`.
