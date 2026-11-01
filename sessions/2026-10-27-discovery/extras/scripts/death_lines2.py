"""Fixes the log-to-match clock offset from round starts, then tests effect lines against kill times.

argv: game log, match details json, log window start, log window end. Prints no IDs or names.
"""
import json
import re
import sys
from datetime import datetime, timezone

log_path, md_path, start, end = sys.argv[1:5]
m = json.load(open(md_path, encoding='utf-8'))
t0 = m['matchInfo']['gameStartMillis'] / 1000
rounds = sorted(m['roundResults'], key=lambda r: r['roundNum'])
kills = []  # (epoch, round, victim team, killer team, victim is user's team?)
team = {p['subject']: p['teamId'] for p in m['players']}
starts = {}
for rr in rounds:
    for ps in rr['playerStats']:
        for k in ps['kills']:
            kills.append((t0 + k['gameTime'] / 1000, rr['roundNum'], team.get(k['victim'])))
            starts[rr['roundNum']] = t0 + (k['gameTime'] - k['roundTime']) / 1000
kills.sort()
print(f"kills {len(kills)}, rounds with a start time {len(starts)}")

stamp = re.compile(r'^\[(\d{4})\.(\d\d)\.(\d\d)-(\d\d)\.(\d\d)\.(\d\d):(\d{3})\]')


def when(raw):
    y, mo, d, h, mi, s, ms = map(int, stamp.match(raw).groups())
    return datetime(y, mo, d, h, mi, s, ms * 1000, tzinfo=timezone.utc).timestamp()


gameplay, effects = [], {}
with open(log_path, encoding='utf-8', errors='replace') as fh:
    for raw in fh:
        if not raw.startswith('[') or raw[1:20] < start or raw[1:20] > end:
            continue
        if 'Gameplay started' in raw:
            gameplay.append(when(raw))
        e = re.search(r'LogAbilitySystem: Warning: \w+ (\w+?)_C_\d+ (\w+)', raw)
        if e:
            effects.setdefault(e.group(2), []).append((when(raw), e.group(1)))

# offset = log time - match time; pick the one that puts most round starts within 1 s of a Gameplay started line
best = None
for i in range(-3000, 3001):
    off = i / 10
    n = sum(any(abs(g - (s + off)) <= 1.0 for g in gameplay) for s in starts.values())
    if best is None or n > best[0]:
        best = (n, off)
n, off = best
print(f"clock offset log - match = {off:+.1f} s, aligning {n}/{len(starts)} round starts with 'Gameplay started' lines ({len(gameplay)} lines)")
print('round start residuals (s): ' + ', '.join(f"{min(abs(g - (s + off)) for g in gameplay):.1f}" for _, s in sorted(starts.items())))

ktimes = [k[0] + off for k in kills]
print('\neffects with 5+ lines, at that fixed offset: share of lines within 1 s of a kill, and of kills within 1 s of a line')
rows = []
for name, items in effects.items():
    if len(items) < 5:
        continue
    ts = [t for t, _ in items]
    near = sum(min(abs(t - k) for k in ktimes) <= 1.0 for t in ts)
    covered = sum(min(abs(t - k) for t in ts) <= 1.0 for k in ktimes)
    lag = sorted(t - min(ktimes, key=lambda k: abs(t - k)) for t in ts)
    rows.append((near / len(ts), name, len(ts), near, covered, lag[len(lag) // 2]))
rows.sort(reverse=True)
for share, name, total, near, covered, med in rows[:12]:
    print(f'  {share:5.0%} of {total:>3} lines near a kill; kills covered {covered}/{len(ktimes)}; median line-kill {med:+.2f}s  {name}')
