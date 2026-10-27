param([string]$Out, [int]$EverySeconds = 20, [int]$MaxMinutes = 50)
# Local only: polls the user's own presence from the Riot Client and reads the game log's round ends.
# Writes one line per change. Stops when the user is back in the menus after a match, or after MaxMinutes.
$log = "$env:LOCALAPPDATA\VALORANT\Saved\Logs\ShooterGame.log"
$lf = (Get-Content "$env:LOCALAPPDATA\Riot Games\Riot Client\Config\lockfile" -Raw).Trim() -split ':'
$base = "https://127.0.0.1:$($lf[2])"
$basic = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("riot:$($lf[3])"))
$self = (curl.exe -k -s -H "Authorization: Basic $basic" "$base/entitlements/v1/token" | ConvertFrom-Json).subject
$pos = 0L; $rounds = New-Object System.Collections.Generic.List[string]; $seenInGame = $false; $last = ''
"utc`tloop`tally`tenemy`tprogress`tlogRoundsEnded`tlastRoundEndUtc" | Set-Content $Out -Encoding UTF8
$stop = (Get-Date).AddMinutes($MaxMinutes)
while ((Get-Date) -lt $stop) {
  try {
    $fs = [IO.File]::Open($log, 'Open', 'Read', 'ReadWrite')
    if ($fs.Length -lt $pos) { $pos = 0 }
    [void]$fs.Seek($pos, 'Begin'); $sr = New-Object IO.StreamReader($fs)
    while (($l = $sr.ReadLine()) -ne $null) {
      if ($l -match '^\[\d{4}\.\d\d\.\d\d-([\d.]+):\d+\].*OnRoundEnded for round ''(\d+)''') { $rounds.Add("$($Matches[2])@$($Matches[1])") }
    }
    $pos = $fs.Position; $sr.Close()
    $pres = (curl.exe -k -s -H "Authorization: Basic $basic" "$base/chat/v4/presences" | ConvertFrom-Json).presences
    $me = $pres | Where-Object { $_.puuid -eq $self -and $_.product -eq 'valorant' } | Select-Object -First 1
    $pj = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($me.private)) | ConvertFrom-Json
    $pk = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($me.packedData)) | ConvertFrom-Json
    $loop = $pj.matchPresenceData.sessionLoopState
    $lastRound = if ($rounds.Count) { $rounds[$rounds.Count - 1] } else { '' }
    $row = "$loop`t$($pj.partyOwnerMatchScoreAllyTeam)`t$($pj.partyOwnerMatchScoreEnemyTeam)`t$($pk.activity.progress.value)`t$($rounds.Count)`t$lastRound"
    if ($row -ne $last) { "$((Get-Date).ToUniversalTime().ToString('HH:mm:ss'))`t$row" | Add-Content $Out -Encoding UTF8; $last = $row }
    if ($loop -eq 'INGAME') { $seenInGame = $true }
    if ($seenInGame -and $loop -eq 'MENUS') { "$((Get-Date).ToUniversalTime().ToString('HH:mm:ss'))`tstopped: back in menus" | Add-Content $Out -Encoding UTF8; break }
  } catch { "$((Get-Date).ToUniversalTime().ToString('HH:mm:ss'))`terror: $($_.Exception.Message)" | Add-Content $Out -Encoding UTF8 }
  Start-Sleep -Seconds $EverySeconds
}
