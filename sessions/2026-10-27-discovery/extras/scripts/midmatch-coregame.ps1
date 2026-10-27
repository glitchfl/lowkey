param([string]$After, [int]$AfterRound = 2, [int]$MaxMinutes = 55)
# Local log watch, then ONE request: waits for a new core-game match in the game log after $After (log time, UTC),
# waits until that match's round $AfterRound has ended, then fetches core-game for it once through probe.ps1.
$dir = Split-Path $MyInvocation.MyCommand.Path
$log = "$env:LOCALAPPDATA\VALORANT\Saved\Logs\ShooterGame.log"
$stop = (Get-Date).AddMinutes($MaxMinutes)
$match = ''; $matchAt = ''
while ((Get-Date) -lt $stop) {
  $fs = [IO.File]::Open($log, 'Open', 'Read', 'ReadWrite'); $sr = New-Object IO.StreamReader($fs); $t = $sr.ReadToEnd(); $sr.Close()
  if (-not $match) {
    $c = [regex]::Matches($t, '(?m)^\[(\d{4}\.\d\d\.\d\d-[\d.]+):\d+\]\[\s*\d+\]LogPlatformCommon: Platform HTTP Query End\. QueryName: \[CoreGame_FetchMatch\], URL \[GET https://glz-eu-1\.eu\.a\.pvp\.net/core-game/v1/matches/([0-9a-f-]{36})\]') | Where-Object { $_.Groups[1].Value -gt $After } | Select-Object -First 1
    if ($c) { $match = $c.Groups[2].Value; $matchAt = $c.Groups[1].Value; "$((Get-Date).ToUniversalTime().ToString('HH:mm:ss')) match found in log at $matchAt" }
  }
  if ($match) {
    $r = [regex]::Matches($t, "(?m)^\[(\d{4}\.\d\d\.\d\d-[\d.]+):\d+\].*OnRoundEnded for round '$AfterRound'") | Where-Object { $_.Groups[1].Value -gt $matchAt } | Select-Object -First 1
    if ($r) {
      "$((Get-Date).ToUniversalTime().ToString('HH:mm:ss')) round $AfterRound ended at $($r.Groups[1].Value), fetching core-game once"
      & "$dir\probe.ps1" -Url "https://glz-eu-1.eu.a.pvp.net/core-game/v1/matches/$match" -Save 'p13-midmatch-core-game'
      $match | Set-Content "$dir\match3-id-private.txt"
      return
    }
  }
  Start-Sleep -Seconds 10
}
'timed out without a request'
