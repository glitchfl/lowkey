param([Parameter(Mandatory)][string]$File)
# Prints every key path with its value type, lists collapsed to [] with their lengths.
# Values are never printed.
$uuid = '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
$paths = [ordered]@{}
$lens = @{}
function Kind($v) {
  if ($null -eq $v) { 'null' }
  elseif ($v -is [bool]) { 'bool' }
  elseif ($v -is [int] -or $v -is [long] -or $v -is [decimal] -or $v -is [double]) { 'number' }
  elseif ($v -is [string]) { if ($v -match $uuid) { 'string(uuid)' } elseif ($v -match '^\d{4}-\d\d-\d\dT') { 'string(time)' } else { 'string' } }
  elseif ($v -is [array]) { 'list' }
  else { 'object' }
}
function Walk($v, [string]$p) {
  $k = Kind $v
  if (-not $paths.Contains($p)) { $paths[$p] = New-Object System.Collections.Generic.HashSet[string] }
  [void]$paths[$p].Add($k)
  if ($k -eq 'list') {
    if (-not $lens.ContainsKey($p)) { $lens[$p] = New-Object System.Collections.Generic.List[int] }
    $lens[$p].Add($v.Count)
    foreach ($e in $v) { Walk $e "$p[]" }
  } elseif ($k -eq 'object') {
    foreach ($prop in $v.PSObject.Properties) {
      # keys that are IDs become {id}, so per-agent or per-map maps collapse to one path
      $name = if ($prop.Name -match $uuid -or $prop.Name -match '^[0-9a-fA-F]{32}$') { '{id}' } else { $prop.Name }
      Walk $prop.Value "$p.$name"
    }
  }
}
$j = [IO.File]::ReadAllText($File) | ConvertFrom-Json
Walk $j '$'
foreach ($p in $paths.Keys) {
  $l = if ($lens.ContainsKey($p)) { $m = $lens[$p] | Measure-Object -Minimum -Maximum; "  len $($m.Minimum)-$($m.Maximum) over $($m.Count)" } else { '' }
  "{0}: {1}{2}" -f $p, (($paths[$p] | Sort-Object) -join '|'), $l
}
