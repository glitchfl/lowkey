param([string[]]$Files = @("$env:LOCALAPPDATA\VALORANT\Saved\Logs\ShooterGame.log"), [string]$Since = '', [string]$Csv = '')
# Parses the request lines in Valorant's game log into endpoint shapes.
# Two line formats:
#   LogPlatformCommon: Platform HTTP Query <Phase>. QueryName: [X], URL [METHOD https://...], ... Response Code: [N]
#   LogReplayData: Replay Data Service: [ptr] Created METHOD query NAME to url <https://...
$uuid = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
function Shape([string]$url) {
  $path, $query = $url -split '\?', 2
  $s = $path -replace $uuid, '{id}' -replace '(?<=/)[0-9a-fA-F]{32}(?=/|$|\.)', '{hex}' -replace '/\d{3,}(?=/|$|\.)', '/{n}'
  if ($query) {
    $keys = $query -split '&' | ForEach-Object { ($_ -split '=', 2)[0] }
    # Party_FetchPlayer passes one key per game server pod; collapse them
    if (($keys | Where-Object { $_ -like 'aresriot.*' }).Count -gt 1) { $keys = @($keys | Where-Object { $_ -notlike 'aresriot.*' }) + '<one key per server pod>' }
    $s += '?' + (($keys | Group-Object | ForEach-Object { if ($_.Count -gt 1) { "$($_.Name) (x$($_.Count))" } else { $_.Name } }) -join '&')
  }
  $s
}
$hits = foreach ($f in $Files) {
  Select-String -Path $f -Pattern 'Platform HTTP Query|Replay Data Service: \[[0-9A-F]+\] Created' | ForEach-Object {
    $l = $_.Line
    $t = if ($l -match '^\[(\d{4}\.\d\d\.\d\d-\d\d\.\d\d\.\d\d):\d+\]') { $Matches[1] } else { '' }
    if ($Since -and $t -lt $Since) { return }
    if ($l -match 'Platform HTTP Query (\w+)\. QueryName: \[([^\]]*)\], URL \[(\w+) (https://[^\]\s]+)\]') {
      [pscustomobject]@{ Time = $t; Phase = $Matches[1]; Name = $Matches[2]; Method = $Matches[3]; Url = Shape $Matches[4]
        Code = if ($l -match 'Response Code: \[(\d+)\]') { $Matches[1] } else { '' } }
    } elseif ($l -match 'Created (\w+) query (\w+) to url <(https://[^>\s]+)') {
      [pscustomobject]@{ Time = $t; Phase = 'Created'; Name = $Matches[2]; Method = $Matches[1]; Url = Shape $Matches[3]; Code = '' }
    }
  }
}
Write-Host "lines parsed: $(@($hits).Count); phases: $(($hits | Group-Object Phase | ForEach-Object { "$($_.Name)=$($_.Count)" }) -join ' ')"
# one row per method + host + path; query key sets are merged
$rows = $hits | Group-Object { "$($_.Method) $(($_.Url -split '\?', 2)[0])" } | ForEach-Object {
  $g = @($_.Group | Sort-Object Time)
  $hostPath = ($g[0].Url -split '\?', 2)[0] -replace '^https://', ''
  $i = $hostPath.IndexOf('/')
  [pscustomobject]@{
    First  = $g[0].Time
    Names  = ($g.Name | Where-Object { $_ } | Sort-Object -Unique) -join ', '
    Method = $g[0].Method
    Host   = $hostPath.Substring(0, $i)
    Path   = $hostPath.Substring($i)
    Query  = ($g | ForEach-Object { if ($_.Url -match '\?(.*)') { $Matches[1] } } | Sort-Object -Unique) -join ' / '
    Count  = $g.Count
    Codes  = ($g.Code | Where-Object { $_ } | Group-Object | Sort-Object Name | ForEach-Object { "$($_.Name)x$($_.Count)" }) -join ' '
  }
} | Sort-Object First
if ($Csv) { $rows | Export-Csv -NoTypeInformation -Encoding UTF8 $Csv }
$rows
