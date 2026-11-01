"""Checks the live K/D/A estimates from match 3 against Riot's final scores.

argv: match details json, the user's PUUID. Prints no IDs or names: players are listed by final score.
"""
import json
import sys

W = (51.36, 310.80, -78.28, 148.05)  # intercept, kills/r, deaths/r, assists/r from perf_fit.py


def est(k, d, a, n):
    return W[0] + W[1] * k / n + W[2] * d / n + W[3] * a / n


def grade(f):
    return 'masterful' if f >= 420 else 'merit' if f >= 330 else 'pass'


m = json.load(open(sys.argv[1], encoding='utf-8'))
me = sys.argv[2]
rounds = sorted(m['roundResults'], key=lambda r: r['roundNum'])
n = len(rounds)
print(f"queue {m['matchInfo']['queueID']}, map {m['matchInfo']['mapId'].split('/')[-1]}, rounds {n}, hasTempValues {m.get('hasTempValues')}")
print('teams: ' + ', '.join(f"{t['teamId']} won={t['won']} {t['roundsWon']}" for t in m['teams']))


def kda_after(sub, cut):
    k = d = a = 0
    for rr in rounds[:cut]:
        for ps in rr.get('playerStats') or []:
            for kill in ps.get('kills') or []:
                k += kill.get('killer') == sub
                d += kill.get('victim') == sub
                a += sub in (kill.get('assistants') or [])
    return k, d, a


print('\nall 10 players, final K/D/A estimate vs Riot (sorted by Riot score):')
errs = []
for p in sorted(m['players'], key=lambda p: -p['scores']['TempValueF']):
    st, f = p['stats'], p['scores']['TempValueF']
    e = est(st['kills'], st['deaths'], st['assists'], n)
    errs.append(abs(e - f))
    tag = ' <- you' if p['subject'] == me else ''
    print(f"  {st['kills']:>2}/{st['deaths']:>2}/{st['assists']:>2}  estimate {e:5.0f} {grade(e):9}  Riot {f:5.0f} {grade(f):9}  off {e - f:+5.0f}{tag}")
print(f'  mean abs error {sum(errs) / len(errs):.1f}, grades right {sum(grade(est(p["stats"]["kills"], p["stats"]["deaths"], p["stats"]["assists"], n)) == grade(p["scores"]["TempValueF"]) for p in m["players"])}/10')

me_p = next(p for p in m['players'] if p['subject'] == me)
s = me_p['scores']
print(f"\nyou: Riot {s['TempValueF']:.1f} {s['TempValueL']['TempValueO']}, combat {s['TempValueL']['TempValueM']}, support {s['TempValueL']['TempValueN']}, mvp {m.get('matchMvp') == me}")
print('  breakdown: ' + ', '.join(f"{lab} {s.get('TempValue' + k, 0):.2f} {imp}" for lab, k, imp in [
    ('damage', 'A', s['TempValueL']['TempValueP']['damage']), ('adj kills', 'I', s['TempValueL']['TempValueP']['killImpact']),
    ('adj deaths', 'J', s['TempValueL']['TempValueP']['deathImpact']), ('trades', 'B', s['TempValueL']['TempValueP']['trades']),
    ('assists', 'C', s['TempValueL']['TempValueQ']['assists']), ('util', 'D', s['TempValueL']['TempValueQ']['utilityUsage']),
    ('plants', 'E', s['TempValueL']['TempValueQ']['plants']), ('defuses', 'K', s['TempValueL']['TempValueQ']['defuses'])]))
print('\nyour K/D/A from match details after each round, vs the Tab screenshots:')
shots = {4: (4, 3, 2), 6: (5, 5, 3)}
for cut in range(1, n + 1):
    k, d, a = kda_after(me, cut)
    note = f'  screenshot said {shots[cut]}' if cut in shots else ''
    print(f'  after {cut:>2}: {k}/{d}/{a}  live estimate {est(k, d, a, cut):5.0f}{note}')
print('lobby K/D/A after 6 rounds (details), to compare with the 3-3 screenshot: ' +
      ', '.join('/'.join(map(str, kda_after(p['subject'], 6))) for p in m['players']))
