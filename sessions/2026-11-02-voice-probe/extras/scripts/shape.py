# Print a redacted shape of a JSON file: keys, types, list lengths.
# Strings are never printed raw: UUIDs -> <uuid-N>, long tokens -> <token>.
# A masked string is shown only for allow-listed enum-like keys, or when it contains a placeholder
# and the leftover literal text is short (e.g. "<uuid-1>-tm1"). Otherwise: str(len=N).
# usage: python -I shape.py <file> <mapfile>
import json, os, re, sys

UUID = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
TOKEN = re.compile(r"[A-Za-z0-9+/=_\-\.]{16,}")
ENUM_KEYS = {"status", "state", "type", "product", "productname", "productid", "mode", "kind",
             "connectionstate", "sessiontype", "channeltype", "locale", "region", "statuscode",
             "errorcode", "message", "httpstatus", "implementationdetails"}
NAMEY = {"name", "gamename", "tagline", "displayname", "alias", "summonername", "gametag", "nickname"}

src, mapfile = sys.argv[1], sys.argv[2]
labels = json.load(open(mapfile)) if os.path.exists(mapfile) else {}

def label(u):
    u = u.lower()
    if u not in labels:
        labels[u] = f"<uuid-{len(labels) + 1}>"
    return labels[u]

def mask(s):
    m = UUID.sub(lambda x: label(x.group(0)), s)
    m = TOKEN.sub(lambda x: x.group(0) if x.group(0).startswith("<uuid") else "<token>", m)
    return m

def maskkey(k):
    # keys are schema names unless they are ids (e.g. maps keyed by puuid)
    if UUID.search(k):
        return UUID.sub(lambda x: label(x.group(0)), k)
    return k if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]{0,40}", k) else "<key>"

def show_str(key, s):
    if s == "":
        return 'str ""'
    k = (key or "").lower()
    m = mask(s)
    residual = re.sub(r"<uuid-\d+>|<token>", "", m)
    if k in ENUM_KEYS and len(s) <= 48 and m == s:
        return f"str {json.dumps(s)}"
    if m != s and len(residual) <= 16 and re.fullmatch(r"[A-Za-z0-9\-_@\.:/]*", residual):
        return f"str {json.dumps(m)}"
    if k in NAMEY:
        return f"str <name> (len={len(s)})"
    if re.fullmatch(r"-?\d+(\.\d+)?", s):
        return f"str numeric (len={len(s)})"
    if s.lower() in ("true", "false"):
        return f"str {s.lower()}"
    return f"str (len={len(s)})"

def merge_keys(items):
    keys = []
    for it in items:
        for k in it:
            if k not in keys:
                keys.append(k)
    return keys

def render(v, key=None, ind=0):
    pad = "  " * ind
    if isinstance(v, dict):
        if not v:
            return "{}"
        lines = ["{"]
        for k, sub in v.items():
            lines.append(f"{pad}  {json.dumps(maskkey(k))}: {render(sub, k, ind + 1)}")
        lines.append(pad + "}")
        return "\n".join(lines)
    if isinstance(v, list):
        if not v:
            return "list[0]"
        if all(isinstance(x, dict) for x in v):
            keys = merge_keys(v)
            out = [f"list[{len(v)}] of {{"]
            for k in keys:
                present = [x[k] for x in v if k in x]
                opt = "" if len(present) == len(v) else f" (present in {len(present)}/{len(v)})"
                vals = [render(p, k, ind + 1) for p in present]
                uniq = list(dict.fromkeys(vals))
                body = uniq[0] if len(uniq) == 1 else " | ".join(uniq[:6]) + (" | ..." if len(uniq) > 6 else "")
                out.append(f"{pad}  {json.dumps(maskkey(k))}: {body}{opt}")
            out.append(pad + "}")
            return "\n".join(out)
        vals = list(dict.fromkeys(render(x, key, ind + 1) for x in v))
        return f"list[{len(v)}] of " + (" | ".join(vals[:6]) + (" | ..." if len(vals) > 6 else ""))
    if isinstance(v, bool):
        return f"bool {str(v).lower()}"
    if isinstance(v, (int, float)):
        return f"{type(v).__name__} {v}"
    if v is None:
        return "null"
    return show_str(key, v)

raw = open(src, "rb").read()
try:
    data = json.loads(raw)
except Exception:
    print(f"(not JSON, {len(raw)} bytes)")
    sys.exit(0)
print(render(data))
json.dump(labels, open(mapfile, "w"))
