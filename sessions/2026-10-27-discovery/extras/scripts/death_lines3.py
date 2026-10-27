"""Whose death does each candidate log line mark, and which deaths does the log miss?

argv: game log, match details json, log window start, log window end, clock offset (s), user's PUUID.
Agents print as the first 8 characters of their content ID; players never print.
"""
import json
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone

log_path, md_path, start, end, off, me = sys.argv[1:7]
off = float(off)
m = json.load(open(md_path, encoding='utf-8'))
t0 = m['matchInfo']['gameStartMillis'] / 1000
team = {p['subject']: p['teamId'] for p in m['players']}
agent = {p['subject']: p['characterId'][:8] for p in m['players']}
mine = team[me]
kills = []
for rr in m['roundResults']:
    for ps in rr['playerStats']:
        for k in ps['kills']:
            kills.append({'t': t0 + k['gameTime'] / 1000 + off, 'victim': k['victim'], 'killer': k['killer'],
                          'round': rr['roundNum']})
kills.sort(key=lambda k: k['t'])

stamp = re.compile(r'^\[(\d{4})\.(\d\d)\.(\d\d)-(\d\d)\.(\d\d)\.(\d\d):(\d{3})\]')
lines = defaultdict(list)
with open(log_path, encoding='utf-8', errors='replace') as fh:
    for raw in fh:
        if not raw.startswith('[') or raw[1:20] < start or raw[1:20] > end:
            continue
        e = re.search(r'LogAbilitySystem: Warning: \w+ (\w+?)_C_\d+ (\w+)', raw)
        if e and e.group(2) in ('Debuff_Disabled_OwnerDead_C', 'Debuff_GrievousWounds_C', 'Buff_Smonk_PostDeathAttributes_C'):
            y, mo, d, h, mi, s, ms = map(int, stamp.match(raw).groups())
            t = datetime(y, mo, d, h, mi, s, ms * 1000, tzinfo=timezone.utc).timestamp()
            lines[e.group(2)].append((t, e.group(1)))

for name, items in lines.items():
    pair = Counter()
    hit = set()
    for t, who in items:
        near = [k for k in kills if abs(t - k['t']) <= 1.0]
        if not near:
            pair[(who, 'no kill')] += 1
            continue
        k = min(near, key=lambda k: abs(t - k['t']))
        hit.add(id(k))
        pair[(who, 'victim ' + agent[k['victim']] + (' ally' if team[k['victim']] == mine else ' enemy'))] += 1
    print(f'\n{name}: {len(items)} lines')
    for (who, what), c in sorted(pair.items(), key=lambda x: -x[1]):
        print(f'  {c:>3}  on {who:<40} {what}')
    cov = Counter(('ally' if team[k['victim']] == mine else 'enemy', id(k) in hit) for k in kills)
    print(f"  kills covered: ally victims {cov[('ally', True)]}/{cov[('ally', True)] + cov[('ally', False)]}, enemy victims {cov[('enemy', True)]}/{cov[('enemy', True)] + cov[('enemy', False)]}")

print('\nagents in the match: ' + ', '.join(sorted(f"{a} {'ally' if team[s] == mine else 'enemy'}{' (you)' if s == me else ''}" for s, a in agent.items())))
