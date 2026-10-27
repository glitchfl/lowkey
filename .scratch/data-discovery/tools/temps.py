import collections
import json
import math
import sys

matches = {}
for f in sys.argv[1:]:
    for line in open(f, encoding="utf-8"):
        r = json.loads(line)
        if r.get("kind") == "remote" and r["path"].startswith("/match-details/") and isinstance(r.get("body"), dict):
            m = r["body"]
            matches[m["matchInfo"]["matchId"]] = m


def derived(m):
    rounds = m.get("roundResults") or []
    out = collections.defaultdict(lambda: collections.Counter())
    for rr in rounds:
        killers = collections.Counter()
        for ps in rr.get("playerStats") or []:
            s = ps["subject"]
            for d in ps.get("damage") or []:
                out[s]["dmg"] += d["damage"]
                out[s]["head"] += d["headshots"]
                out[s]["body"] += d["bodyshots"]
                out[s]["leg"] += d["legshots"]
            killers[s] += len(ps.get("kills") or [])
        for s, k in killers.items():
            if k >= 2:
                out[s][f"mk{min(k, 5)}"] += 1
        if rr.get("firstBloodPlayer"):
            out[rr["firstBloodPlayer"]]["fb"] += 1
        if rr.get("ceremonyPlayer"):
            out[rr["ceremonyPlayer"]]["ceremony"] += 1
        if rr.get("bombPlanter"):
            out[rr["bombPlanter"]]["plants"] += 1
        if rr.get("bombDefuser"):
            out[rr["bombDefuser"]]["defuses"] += 1
    return out


rows = []
for mid, m in matches.items():
    if not m.get("hasTempValues"):
        continue
    d = derived(m)
    q = m["matchInfo"]["queueID"]
    won = {t["teamId"]: t.get("won") for t in m.get("teams") or []}
    for p in m["players"]:
        st, sc = p.get("stats") or {}, p.get("scores") or {}
        n = max(st.get("roundsPlayed", 0), 1)
        x = d[p["subject"]]
        hits = x["head"] + x["body"] + x["leg"]
        row = {
            "queue": q,
            "won": won.get(p["teamId"]),
            "rounds": st.get("roundsPlayed"),
            "kills": st.get("kills"), "deaths": st.get("deaths"), "assists": st.get("assists"),
            "acs": st.get("score", 0) / n, "adr": x["dmg"] / n, "dmg": x["dmg"],
            "hs": x["head"] / hits if hits else 0,
            "kpr": st.get("kills", 0) / n, "dpr": st.get("deaths", 0) / n, "apr": st.get("assists", 0) / n,
            "fb": x["fb"], "fbr": x["fb"] / n, "plants": x["plants"], "defuses": x["defuses"], "ceremony": x["ceremony"],
            "mk2": x["mk2"], "mk3": x["mk3"], "mk4": x["mk4"], "mk5": x["mk5"],
            "mvp": p["subject"] == m.get("matchMvp"),
        }
        for k, v in st.items():
            if k.startswith("TempValue"):
                row["st." + k] = v
        for k, v in sc.items():
            if k.startswith("TempValue") and not isinstance(v, dict):
                row["sc." + k] = v
        lvl = sc.get("TempValueL") or {}
        row["grade"] = lvl.get("TempValueO")
        row["M"], row["N"] = lvl.get("TempValueM"), lvl.get("TempValueN")
        rows.append(row)

print(f"{len(rows)} player rows with temp values, queues {collections.Counter(r['queue'] for r in rows)}")


def corr(a, b):
    pairs = [(x, y) for x, y in zip(a, b) if isinstance(x, (int, float)) and isinstance(y, (int, float))]
    if len(pairs) < 20:
        return None, len(pairs)
    xs, ys = zip(*pairs)
    mx, my = sum(xs) / len(xs), sum(ys) / len(ys)
    sxy = sum((x - mx) * (y - my) for x, y in pairs)
    sx = math.sqrt(sum((x - mx) ** 2 for x in xs))
    sy = math.sqrt(sum((y - my) ** 2 for y in ys))
    return (sxy / (sx * sy) if sx and sy else None), len(pairs)


known = ["rounds", "kills", "deaths", "assists", "acs", "adr", "dmg", "hs", "kpr", "dpr", "apr", "fb", "fbr", "plants", "defuses", "ceremony", "mk2", "mk3", "mk4", "mk5"]
temps = sorted({k for r in rows for k in r if k.startswith(("st.", "sc."))})
for q in ("competitive", "hurm", "deathmatch"):
    sub = [r for r in rows if r["queue"] == q]
    if not sub:
        continue
    print(f"\n### {q}: {len(sub)} rows")
    for t in temps:
        vals = [r.get(t) for r in sub]
        present = [v for v in vals if v is not None]
        if not present:
            continue
        best = []
        for k in known:
            c, n = corr(vals, [r[k] for r in sub])
            if c is not None:
                best.append((abs(c), c, k, n))
        best.sort(reverse=True)
        rng = f"{min(present):.3g}..{max(present):.3g}"
        print(f"{t:16} n={len(present):4} range {rng:18} best: " + ", ".join(f"{k} {c:+.3f}" for _, c, k, _ in best[:3]))
        exact = [k for k in known if all(r.get(t) is None or abs(r.get(t) - r[k]) < 1e-6 for r in sub)]
        if exact:
            print(f"{'':16} exactly equals {exact}")

comp = [r for r in rows if r["queue"] == "competitive" and r["grade"]]
print("\n### grade vs score fields (competitive)")
for g in ("pass", "merit", "distinction"):
    sub = [r for r in comp if r["grade"] == g]
    if not sub:
        continue
    desc = []
    for t in ("sc.TempValueF", "sc.TempValueG", "sc.TempValueH", "acs", "adr"):
        v = sorted(r.get(t) for r in sub if r.get(t) is not None)
        if v:
            desc.append(f"{t} {v[0]:.0f}-{v[-1]:.0f} (median {v[len(v) // 2]:.0f})")
    print(f"{g:12} n={len(sub):4} won={sum(1 for r in sub if r['won'])}  " + " | ".join(desc))
print("\nM x N:", collections.Counter((r["M"], r["N"]) for r in comp).most_common(12))
print("grade by won:", collections.Counter((r["grade"], r["won"]) for r in comp))
print("M by won:", collections.Counter((r["M"], r["won"]) for r in comp))
print("N by won:", collections.Counter((r["N"], r["won"]) for r in comp))
