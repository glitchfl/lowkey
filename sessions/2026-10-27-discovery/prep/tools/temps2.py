import collections, json, math, sys
ms = {}
for f in sys.argv[1:]:
    for l in open(f, encoding="utf-8"):
        r = json.loads(l)
        if r.get("kind") == "remote" and r["path"].startswith("/match-details/") and isinstance(r.get("body"), dict) and r["body"].get("hasTempValues"):
            ms[r["body"]["matchInfo"]["matchId"]] = r["body"]

def corr(xs, ys):
    p = [(x, y) for x, y in zip(xs, ys) if x is not None and y is not None]
    xs, ys = zip(*p); mx, my = sum(xs)/len(xs), sum(ys)/len(ys)
    a = sum((x-mx)*(y-my) for x, y in p); b = math.sqrt(sum((x-mx)**2 for x in xs)*sum((y-my)**2 for y in ys))
    return a/b if b else 0

rows = []
for m in ms.values():
    team = {p["subject"]: p["teamId"] for p in m["players"]}
    tr = collections.Counter(); traded = collections.Counter(); kast = collections.Counter()
    for rr in m["roundResults"]:
        kills = sorted((k for ps in rr["playerStats"] for k in ps.get("kills") or []), key=lambda k: k["roundTime"])
        alive_contrib = set()
        for i, k in enumerate(kills):
            alive_contrib.add(k["killer"]); alive_contrib.update(k.get("assistants") or [])
            for later in kills[i+1:]:
                if later["roundTime"] - k["roundTime"] > 5000: break
                if later["victim"] == k["killer"] and team.get(later["killer"]) == team.get(k["victim"]):
                    tr[later["killer"]] += 1; traded[k["victim"]] += 1; alive_contrib.add(k["victim"]); break
        died = {k["victim"] for k in kills}
        for ps in rr["playerStats"]:
            s = ps["subject"]
            if s in alive_contrib or s not in died: kast[s] += 1
    for p in m["players"]:
        st, sc = p["stats"], p.get("scores") or {}
        n = max(st["roundsPlayed"], 1); ac = st.get("abilityCasts") or {}
        L = sc.get("TempValueL") or {}
        rows.append(dict(adr_raw=sum(d["damage"] for d in p.get("roundDamage") or [])/n,
            casts=sum(ac.values())/n, ult=ac.get("ultimateCasts",0)/n, trades=tr[p["subject"]]/n, traded=traded[p["subject"]]/n,
            trades_n=tr[p["subject"]], traded_n=traded[p["subject"]], kast=kast[p["subject"]]/n,
            **{"sc."+k: v for k, v in sc.items() if not isinstance(v, dict)}, **{"st."+k: v for k, v in st.items() if k.startswith("Temp")},
            M=L.get("TempValueM"), N=L.get("TempValueN"), P=L.get("TempValueP") or {}, Q=L.get("TempValueQ") or {}))
for t in ("sc.TempValueA","sc.TempValueB","sc.TempValueD","st.TempValueB","st.TempValueC"):
    v = [r.get(t) for r in rows]
    print(t, {k: round(corr(v, [r[k] for r in rows]), 3) for k in ("adr_raw","casts","ult","trades","traded","trades_n","traded_n","kast")})
lvl = {"double_down": -2, "down": -1, "neutral": 0, "up": 1, "double_up": 2}
for name, get in [("M", lambda r: r["M"]), ("N", lambda r: r["N"])] + [(f"P.{k}", (lambda k: lambda r: r["P"].get(k))(k)) for k in ("damage","deathImpact","killImpact","trades")] + [(f"Q.{k}", (lambda k: lambda r: r["Q"].get(k))(k)) for k in ("assists","defuses","plants","utilityUsage")]:
    v = [lvl.get(get(r)) for r in rows]
    cs = {t: round(corr(v, [r.get(t) for r in rows]), 3) for t in ("sc.TempValueA","sc.TempValueB","sc.TempValueC","sc.TempValueD","sc.TempValueE","sc.TempValueG","sc.TempValueH","sc.TempValueI","sc.TempValueJ","sc.TempValueK","sc.TempValueF")}
    top = sorted(cs.items(), key=lambda x: -abs(x[1]))[:3]
    print(f"{name:16}", top)
