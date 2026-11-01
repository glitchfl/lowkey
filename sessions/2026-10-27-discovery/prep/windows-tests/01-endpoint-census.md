# 01: Endpoint census from the game's own log

**Why:** Live events show Riot services lowkey has never called: lifetime stats (`cap.lifetime-stats`), map rating (`ares-region-index`), accolades and agent mastery (`.scratch/data-discovery/findings.md`). The game logs the requests it makes, so reading its log lists every endpoint it uses without sending a single extra request. The local API's `/help` lists every local endpoint and event the same way.

**Needs:** the Riot Client and Valorant in the menus

**Status:** open

## Steps

1. `GET https://127.0.0.1:<port>/help` with the local Basic auth. Save the response as `local-api-help.json` after checking that it holds no IDs, names or tokens.
2. Ask the user to go through these screens in order, staying about 5 s on each, then tell you they're done:
   1. Career → Match History, then open one recent competitive match and click through every tab it has
   2. any Career page that shows stats per agent, per map or for the whole act
   3. Agents → their most-played agent → any mastery or progression page
   4. Premier, without joining or changing anything
   5. Leaderboards, without searching for anyone
3. Find the lines in `%LOCALAPPDATA%\VALORANT\Saved\Logs\ShooterGame.log` that record the game's HTTP requests. Try `Platform HTTP Query`, `QueryName`, `URL [` and `https://` first. Report the line format with IDs, names and query values removed.
4. Build `endpoints.md`: one row per endpoint shape, with these columns:
   - query name, if the line has one
   - method
   - host
   - path, with every UUID as `{id}` and every numeric ID as `{n}`
   - query keys only, with no values
   - count, response codes, and the first time it was seen
   - which tour step from step 2 first triggered it

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
5. Before saving, read `endpoints.md` for anything private that slipped through: Riot IDs in a path, tokens, signatures (`Signature=`, `Key-Pair-Id=`), or long random strings. Replace each with a placeholder.
6. After each match later in the session, run step 4 again and add any new rows, marked with the match they came from.

## Send back

- `local-api-help.json`
- `endpoints.md`, with the log line format above the table
- in the report: the shapes that look like lifetime stats, map rating, accolades or mastery, called out at the top

## Results
