# Every Riot request Valorant itself made, from its own logs: method, host, path (IDs masked),
# the game's name for the query, the phases it ran in and the status codes. Sends nothing.
# usage: python -I game_requests.py <out.json> "%LOCALAPPDATA%\VALORANT\Saved\Logs"
import collections, glob, json, os, re, sys

out, logs = sys.argv[1], sys.argv[2]
files = sorted(glob.glob(os.path.join(logs, "ShooterGame*.log")), key=os.path.getmtime)
QUERY = re.compile(r"QueryName: \[([^\]]*)\], URL \[(\w+) https://([^/\]]+)([^\]?]*)(\?[^\]]*)?\].*?Response Code: \[(\d+)\], Seconds Since Query \[([0-9.]+)\]")


def host(h):
    for k in ("glz", "pd", "shared"):
        if h.startswith(k + "-") or h.startswith(k + "."):
            return k
    return re.sub(r"\d+", "N", h)


def template(p):
    p = re.sub(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}", "{id}", p)
    p = re.sub(r"[0-9a-fA-F]{20,}", "{id}", p)  # IDs written as 32 hex digits without dashes
    return re.sub(r"/[0-9]{3,}", "/{n}", p)


eps = {}
for f in files:
    phase = "MENUS"
    for line in open(f, encoding="utf-8", errors="replace"):
        if "Loopstate changed" in line:
            m = re.search(r"to (\w+)", line.split("Loopstate changed", 1)[1])
            phase = m.group(1) if m else phase
        if "Platform HTTP Query End" not in line:
            continue
        m = QUERY.search(line)
        if not m:
            continue
        name, method, h, path, query, code, secs = m.groups()
        key = (method, host(h), template(path))
        e = eps.setdefault(key, {"method": method, "host": host(h), "path": template(path), "names": collections.Counter(),
                                 "phases": collections.Counter(), "codes": collections.Counter(), "query_keys": set(), "slowest": 0.0})
        e["names"][name] += 1
        e["phases"][phase] += 1
        e["codes"][code] += 1
        e["slowest"] = max(e["slowest"], float(secs))
        if query:
            e["query_keys"].update(k.split("=")[0] for k in query[1:].split("&"))
res = sorted(({**e, "names": dict(e["names"]), "phases": dict(e["phases"]), "codes": dict(e["codes"]),
               "query_keys": sorted(e["query_keys"])} for e in eps.values()), key=lambda e: (e["host"], e["path"]))
json.dump(res, open(out, "w", encoding="utf-8"), indent=1)
print(f"{len(files)} logs, {sum(sum(e['names'].values()) for e in res)} requests, {len(res)} endpoints -> {out}")
