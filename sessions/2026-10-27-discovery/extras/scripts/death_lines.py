"""Do any game log lines line up with the kills in match details?

argv: game log, match details json, log window start, log window end (log time like 2026.10.27-17.43.45).
For each log effect (and a few other line types), finds the clock offset that lines up most of its lines
with a kill time within 1 s, and prints how many line up. Prints no IDs or names.
"""
import json
import re
import sys
from datetime import datetime, timezone

log_path, md_path, start, end = sys.argv[1:5]
m = json.load(open(md_path, encoding='utf-8'))
t0 = m['matchInfo']['gameStartMillis'] / 1000
kills = sorted(t0 + k['gameTime'] / 1000 for rr in m['roundResults'] for ps in rr['playerStats'] for k in ps['kills'])
victims = {}
for rr in m['roundResults']:
    for ps in rr['playerStats']:
        for k in ps['kills']:
            victims[t0 + k['gameTime'] / 1000] = k['victim']
print(f'kills in match details: {len(kills)}')

line = re.compile(r'^\[(\d{4})\.(\d\d)\.(\d\d)-(\d\d)\.(\d\d)\.(\d\d):(\d{3})\]\[\s*\d+\](\w+): (.*)')
events = {}
with open(log_path, encoding='utf-8', errors='replace') as fh:
    for raw in fh:
        if not raw.startswith('['):
            continue
        stamp = raw[1:20]
        if stamp < start or stamp > end:
            continue
        g = line.match(raw)
        if not g:
            continue
        y, mo, d, h, mi, s, ms, cat, msg = g.groups()
        t = datetime(int(y), int(mo), int(d), int(h), int(mi), int(s), int(ms) * 1000, tzinfo=timezone.utc).timestamp()
        if cat == 'LogAbilitySystem':
            e = re.search(r'(\w+?)_C_\d+ (\w+)', msg)
            if e:
                events.setdefault('effect ' + e.group(2), []).append(t)
                if e.group(1).endswith('_PC'):
                    events.setdefault('effect on a player ' + e.group(2), []).append(t)
        else:
            key = cat + ': ' + re.sub(r'[0-9a-fA-F-]{20,}|\d+', 'N', msg)[:70]
            events.setdefault(key, []).append(t)


def hits(ts, off):
    n = 0
    for t in ts:
        lo, hi = 0, len(kills)
        while lo < hi:
            mid = (lo + hi) // 2
            if kills[mid] < t - off:
                lo = mid + 1
            else:
                hi = mid
        best = min((abs(kills[i] - (t - off)) for i in (lo - 1, lo) if 0 <= i < len(kills)), default=99)
        n += best <= 1.0
    return n


rows = []
for key, ts in events.items():
    if len(ts) < 5 or len(ts) > 400:
        continue
    best = max(((hits(ts, off / 2), off / 2) for off in range(-240, 241)), key=lambda x: x[0])
    rows.append((best[0] / len(ts), best[0], len(ts), best[1], key))
rows.sort(reverse=True)
print('share of lines within 1 s of a kill at the best clock offset (top 15):')
for share, n, total, off, key in rows[:15]:
    print(f'  {share:5.0%}  {n:>3}/{total:<3}  offset {off:+6.1f}s  {key}')
