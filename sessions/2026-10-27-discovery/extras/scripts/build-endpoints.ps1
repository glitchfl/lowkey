param([string]$Out = "$env:USERPROFILE\lowkey\for-the-mac\endpoints.md", [string]$Header = '')
# Builds endpoints.md from the current game log (tour and matches) and today's earlier log.
$s = Split-Path $MyInvocation.MyCommand.Path
$d = "$env:LOCALAPPDATA\VALORANT\Saved\Logs"
$earlierLog = "$d\ShooterGame-backup-2026.10.27-15.04.45.log"
$uuid = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
# trigger windows in UTC, from the client's own UI navigation log lines; later windows come from matches.csv
$windows = @(
  @{ From = '00.00.00'; Label = 'startup, before the tour' }
  @{ From = '16.50.20'; Label = 'tour asked for, no screen yet' }
  @{ From = '16.51.07'; Label = 'tour 1: Career, Match History' }
  @{ From = '16.51.29'; Label = 'tour 2: Career, competitive hub overview' }
  @{ From = '16.51.59'; Label = 'tour 2: competitive hub prestige, map rating, accolades' }
  @{ From = '16.52.28'; Label = 'tour 3: Agents, mastery' }
  @{ From = '16.52.53'; Label = 'Esports hub (opened instead of Premier, Leaderboards)' }
  @{ From = '16.52.55'; Label = 'after the tour: lobby, Career, Collections' }
)
if (Test-Path "$s\matches.csv") { Import-Csv "$s\matches.csv" | ForEach-Object { $windows += @{ From = $_.From; Label = $_.Label } } }
function Trigger([string]$t) { $l = ''; foreach ($w in $windows) { if ($t -ge $w.From) { $l = $w.Label } }; $l }

$cur = & "$s\census.ps1"
$old = & "$s\census.ps1" -Files $earlierLog
$curKeys = $cur | ForEach-Object { "$($_.Method) $($_.Host)$($_.Path)" }
$esc = { param($x) ($x -replace '\|', '\|') }
$md = New-Object System.Collections.Generic.List[string]
$md.Add('# Endpoint census · 2026-10-27 data discovery session')
$md.Add('')
$md.Add('Built from Valorant''s own log, `%LOCALAPPDATA%\VALORANT\Saved\Logs\ShooterGame.log`, without sending any request. Times are the log''s, in UTC. Every UUID is `{id}`, every 32-character hex ID (a UUID without dashes, used for agents, matches and sometimes players) is `{hex}`, every numeric ID is `{n}`. Query strings keep their keys only.')
if ($Header) { $md.Add(''); $md.Add($Header) }
$md.Add('')
$md.Add('## Log line formats')
$md.Add('')
$md.Add('Requests the game makes through its platform layer, one line per finished request:')
$md.Add('```')
$md.Add('[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogPlatformCommon: Platform HTTP Query End. QueryName: [<name>], URL [<METHOD> https://<host>/<path>?<query>], @trace_id:<32 hex> Response Code: [<code>], Seconds Since Query [<seconds>]')
$md.Add('```')
$md.Add('Requests from the replay data service, logged when created, with no response line:')
$md.Add('```')
$md.Add('[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogReplayData: Replay Data Service: [<pointer>] Created <METHOD> query <NAME> to url <https://<host>/<path>?<query>')
$md.Add('```')
$md.Add('Pushes from the Riot messaging service (not requests, listed in the last table):')
$md.Add('```')
$md.Add('[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogRMSService: Received RMS update. URI: /riot-messaging-service/v1/messages/<service>/<path>')
$md.Add('```')
$md.Add('The screen the client shows, which dates the tour steps:')
$md.Add('```')
$md.Add('[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogUINavigationModel: Warning: Current Url: main/hub/<screen>/<subscreen>')
$md.Add('```')
$md.Add('')
$md.Add('## This session (current log, game started 16:48 UTC)')
$md.Add('')
$md.Add('| Query name | Method | Host | Path | Query keys | Count | Codes | First seen | First triggered by |')
$md.Add('|---|---|---|---|---|---|---|---|---|')
foreach ($r in $cur) {
  $t = $r.First.Substring(11)
  $md.Add("| $($r.Names) | $($r.Method) | $($r.Host) | ``$(& $esc $r.Path)`` | $(if ($r.Query) { '`' + (& $esc $r.Query) + '`' }) | $($r.Count) | $($r.Codes) | $($t -replace '\.', ':') | $(Trigger $t) |")
}
$md.Add('')
$md.Add('## Seen only in today''s earlier log (first session, 10:43–15:04 UTC)')
$md.Add('')
$md.Add('These come from `ShooterGame-backup-2026.10.27-15.04.45.log`, the morning''s session with several matches. They aren''t in this session''s log yet.')
$md.Add('')
$md.Add('| Query name | Method | Host | Path | Query keys | Count | Codes | First seen |')
$md.Add('|---|---|---|---|---|---|---|---|')
foreach ($r in ($old | Where-Object { $curKeys -notcontains "$($_.Method) $($_.Host)$($_.Path)" })) {
  $md.Add("| $($r.Names) | $($r.Method) | $($r.Host) | ``$(& $esc $r.Path)`` | $(if ($r.Query) { '`' + (& $esc $r.Query) + '`' }) | $($r.Count) | $($r.Codes) | $($r.First.Substring(11) -replace '\.', ':') |")
}
$md.Add('')
$md.Add('## Messaging service pushes')
$md.Add('')
$md.Add('| Service path | Earlier log count | This session count | First seen this session |')
$md.Add('|---|---|---|---|')
$rms = foreach ($pair in @(@('earlier', $earlierLog), @('current', "$d\ShooterGame.log"))) {
  Select-String -Path $pair[1] -Pattern 'Received RMS update\. URI: (\S+)' | ForEach-Object {
    [pscustomobject]@{ Src = $pair[0]; T = if ($_.Line -match '^\[\d{4}\.\d\d\.\d\d-([\d.]+):') { $Matches[1] } else { '' }
      U = $_.Matches[0].Groups[1].Value -replace '^/riot-messaging-service/v1/messages/', '' -replace $uuid, '{id}' -replace '(?<![0-9a-fA-F])[0-9a-fA-F]{32}(?![0-9a-fA-F])', '{hex}' -replace '(?<=activities/v1/)[A-Za-z0-9]{8,}(?=/)', '{code}' -replace '\?.*', '' }
  }
}
foreach ($g in ($rms | Group-Object U | Sort-Object Name)) {
  $c = @($g.Group | Where-Object Src -eq 'current')
  $md.Add("| ``$($g.Name)`` | $(@($g.Group | Where-Object Src -eq 'earlier').Count) | $($c.Count) | $(if ($c) { $c[0].T -replace '\.', ':' }) |")
}
[IO.File]::WriteAllLines($Out, $md)
"rows: current $(@($cur).Count), earlier-only $(@($old | Where-Object { $curKeys -notcontains "$($_.Method) $($_.Host)$($_.Path)" }).Count), pushes $(@($rms | Group-Object U).Count)"
