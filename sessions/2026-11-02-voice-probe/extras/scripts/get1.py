# One GET against the local Riot Client API. Saves raw body to a file, prints only status/size.
# usage: python -I get1.py <path> <outfile>
import os, sys, time
import requests, urllib3

urllib3.disable_warnings()
path, outfile = sys.argv[1], sys.argv[2]
assert path.startswith("/"), "path must be local"
lock = os.path.join(os.environ["LOCALAPPDATA"], "Riot Games", "Riot Client", "Config", "lockfile")
_name, _pid, port, pw, proto = open(lock, encoding="utf-8").read().strip().split(":")

time.sleep(2)  # keep requests a couple of seconds apart
try:
    r = requests.get(f"https://127.0.0.1:{port}{path}", auth=("riot", pw), verify=False, timeout=20)
except Exception as e:
    print(f"GET {path} -> EXCEPTION {type(e).__name__}")
    sys.exit(1)
with open(outfile, "wb") as f:
    f.write(r.content)
print(f"GET {path} -> {r.status_code} content-type={r.headers.get('content-type')} bytes={len(r.content)}")
