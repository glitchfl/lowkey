import base64
import collections
import glob
import json
import re
import sys

UUID = re.compile(r'[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}')

stats = collections.defaultdict(lambda: collections.defaultdict(lambda: {"n": 0, "types": collections.Counter(), "vals": collections.Counter(), "empty": 0}))
docs = collections.Counter()


def b64json(s):
    try:
        return json.loads(base64.b64decode(s + "=" * (-len(s) % 4)))
    except Exception:
        return None


def walk(ep, path, v):
    if isinstance(v, dict):
        if v and all(UUID.fullmatch(k) for k in v):
            for sub in v.values():
                walk(ep, path + ".{id}", sub)
            return
        for k, sub in v.items():
            walk(ep, f"{path}.{k}" if path else k, sub)
        return
    if isinstance(v, list):
        s = stats[ep][path + "[]"]
        s["n"] += 1
        s["types"]["list"] += 1
        if not v:
            s["empty"] += 1
        for sub in v:
            walk(ep, path + "[]", sub)
        return
    s = stats[ep][path]
    s["n"] += 1
    s["types"][type(v).__name__] += 1
    if v in (None, "", 0, False):
        s["empty"] += 1
    sv = UUID.sub("{id}", str(v))[:60]
    if len(s["vals"]) < 400 or sv in s["vals"]:
        s["vals"][sv] += 1


def presence(ep, p):
    walk(ep, "", p)
    if p.get("private"):
        d = b64json(p["private"])
        if isinstance(d, dict):
            docs[ep + " private"] += 1
            walk(ep + " private", "", d)
    if p.get("packedData"):
        d = b64json(p["packedData"])
        if isinstance(d, dict):
            docs[ep + " packedData"] += 1
            walk(ep + " packedData", "", d)


for f in sys.argv[1:]:
    for line in open(f, encoding="utf-8"):
        try:
            r = json.loads(line)
        except ValueError:
            continue
        kind = r.get("kind")
        if kind in ("local", "remote"):
            ep = f"{r.get('service') or 'local'} {UUID.sub('{id}', r['path'])}"
            docs[ep] += 1
            body = r.get("body")
            if ep == "local /chat/v4/presences" and isinstance(body, dict):
                for p in body.get("presences", []):
                    presence("presence", p)
            else:
                walk(ep, "", body)
        elif kind == "event":
            b = r.get("body")
            if not isinstance(b, list) or len(b) < 3:
                continue
            payload = b[2]
            if r["event"].endswith("presences"):
                for p in (payload.get("data") or {}).get("presences", []):
                    presence("presence", p)
            else:
                uri = UUID.sub("{id}", payload.get("uri", ""))
                uri = re.sub(r"/v1/message/|/v1/messages/", "/msg/", uri)
                ep = "rms " + uri.split("/msg/")[-1]
                docs[ep] += 1
                walk(ep, "", payload.get("data"))

for ep in sorted(stats):
    print(f"\n## {ep}  ({docs[ep]} docs)")
    for path, s in sorted(stats[ep].items()):
        t = "/".join(k for k, _ in s["types"].most_common())
        distinct = len(s["vals"])
        ex = ", ".join(f"{v}" for v, _ in s["vals"].most_common(4))
        full = "" if s["empty"] == 0 else f" empty={s['empty']}"
        print(f"{path}  [{t}] n={s['n']}{full} distinct={distinct}{'+' if distinct >= 400 else ''}  e.g. {ex}")
