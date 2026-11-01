$lf = (Get-Content "$env:LOCALAPPDATA\Riot Games\Riot Client\Config\lockfile" -Raw).Trim() -split ':'
$port = $lf[2]; $pass = $lf[3]
$auth = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("riot:$pass"))
$out = "$env:USERPROFILE\lowkey\for-the-mac\local-api-help.raw.json"
$code = curl.exe -k -s -o $out -w '%{http_code}' -H "Authorization: Basic $auth" "https://127.0.0.1:$port/help"
"status $code, bytes $((Get-Item $out).Length)"
