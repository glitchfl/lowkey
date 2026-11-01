param([Parameter(Mandatory)][string]$Url, [Parameter(Mandatory)][string]$Save)
# One GET to Riot's servers with the game's headers. {self} and {selfhex} become the user's own PUUID.
# Keeps a request counter and refuses a 16th request or one sent sooner than 5 s after the last.
$ErrorActionPreference = 'Stop'
$dir = Split-Path $MyInvocation.MyCommand.Path
$raw = Join-Path $dir 'raw'; New-Item -ItemType Directory -Force $raw | Out-Null
$counter = Join-Path $dir 'probe-count.txt'
$n = if (Test-Path $counter) { [int](Get-Content $counter) } else { 0 }
if ($n -ge 15) { throw "request budget used up ($n of 15)" }
$last = Join-Path $dir 'probe-last.txt'
if (Test-Path $last) {
  $wait = 5.5 - ((Get-Date) - [datetime]::Parse((Get-Content $last))).TotalSeconds
  if ($wait -gt 0) { Start-Sleep -Milliseconds ([int]($wait * 1000)) }
}

$lf = (Get-Content "$env:LOCALAPPDATA\Riot Games\Riot Client\Config\lockfile" -Raw).Trim() -split ':'
$basic = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("riot:$($lf[3])"))
$ent = curl.exe -k -s -H "Authorization: Basic $basic" "https://127.0.0.1:$($lf[2])/entitlements/v1/token" | ConvertFrom-Json
$self = $ent.subject
$platform = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('{"platformType":"PC","platformOS":"Windows","platformOSVersion":"10.0.19042.1.256.64bit","platformChipset":"Unknown"}'))
$version = (Get-Content (Join-Path $dir 'client-version.txt')).Trim()

$target = $Url.Replace('{self}', $self).Replace('{selfhex}', ($self -replace '-', '').ToUpper())
if ($target -notmatch '^https://[a-z0-9.-]+\.pvp\.net/') { throw 'only pvp.net hosts' }
$out = Join-Path $raw "$Save.json"
$code = curl.exe -s -o $out -w '%{http_code}' -X GET $target `
  -H "Authorization: Bearer $($ent.accessToken)" `
  -H "X-Riot-Entitlements-JWT: $($ent.token)" `
  -H "X-Riot-ClientPlatform: $platform" `
  -H "X-Riot-ClientVersion: $version"
(Get-Date).ToString('o') | Set-Content $last
$n + 1 | Set-Content $counter
"request $($n + 1) of 15 at $((Get-Date).ToUniversalTime().ToString('HH:mm:ss')) UTC: status $code, $((Get-Item $out).Length) bytes"
