"""How well can Riot's performance score (scores.TempValueF) be rebuilt or predicted?

Reads lowkey record.jsonl files, keeps each match-details response once, then:
1. fits F on the 8 breakdown values (A, I, J, B, C, D, E, K) to see if the formula is linear
2. fits F on kills, deaths, assists per round, which a player can see live on the Tab scoreboard
3. applies fit 2 to each player's K/D/A after the first N rounds, against the final F
Prints only aggregate numbers: no IDs, names or single players.
"""
import json
import sys


def solve(rows, ys):
    """Least squares with an intercept, by normal equations and Gaussian elimination."""
    xs = [[1.0] + r for r in rows]
    n = len(xs[0])
    a = [[sum(x[i] * x[j] for x in xs) for j in range(n)] for i in range(n)]
    b = [sum(x[i] * y for x, y in zip(xs, ys)) for i in range(n)]
    for c in range(n):
        p = max(range(c, n), key=lambda r: abs(a[r][c]))
        a[c], a[p], b[c], b[p] = a[p], a[c], b[p], b[c]
        for r in range(n):
            if r != c and a[c][c]:
                f = a[r][c] / a[c][c]
                a[r] = [v - f * w for v, w in zip(a[r], a[c])]
                b[r] -= f * b[c]
    # a column that is all zero gets weight 0
    return [b[i] / a[i][i] if abs(a[i][i]) > 1e-12 else 0.0 for i in range(n)]


def predict(w, r):
    return w[0] + sum(c * v for c, v in zip(w[1:], r))


def grade(f):
    return 'distinction' if f >= 420 else 'merit' if f >= 330 else 'pass'


def report(name, w, rows, ys, labels):
    ps = [predict(w, r) for r in rows]
    mean = sum(ys) / len(ys)
    ss = sum((y - mean) ** 2 for y in ys)
    r2 = 1 - sum((y - p) ** 2 for y, p in zip(ys, ps)) / ss
    mae = sum(abs(y - p) for y, p in zip(ys, ps)) / len(ys)
    acc = sum(grade(y) == grade(p) for y, p in zip(ys, ps)) / len(ys)
    within = lambda d: sum(abs(y - p) <= d for y, p in zip(ys, ps)) / len(ys)
    print(f'{name}: n={len(ys)} R2={r2:.3f} MAE={mae:.1f} within25={within(25):.0%} within50={within(50):.0%} grade right={acc:.0%}')
    print('  weights: intercept %.2f, ' % w[0] + ', '.join(f'{l} {c:.2f}' for l, c in zip(labels, w[1:])))


matches = {}
for path in sys.argv[1:]:
    with open(path, encoding='utf-8') as fh:
        for line in fh:
            if '"/match-details/v1/matches/' not in line or '"kind":"remote"' not in line:
                continue
            rec = json.loads(line)
            body = rec.get('body')
            if not isinstance(body, dict) or 'matchInfo' not in body:
                continue
            matches[body['matchInfo']['matchId']] = body

full, live, partial = [], [], {3: [], 6: [], 9: [], 12: []}
queues = {}
for m in matches.values():
    rounds = m.get('roundResults') or []
    n = len(rounds)
    if n < 5:
        continue
    q = m['matchInfo'].get('queueID', '')
    for p in m.get('players', []):
        s = p.get('scores') or {}
        if 'TempValueF' not in s:
            continue
        queues[q] = queues.get(q, 0) + 1
        f = s['TempValueF']
        full.append(([s.get('TempValue' + k, 0.0) for k in 'AIJBCDEK'], f))
        st = p['stats']
        live.append(([st['kills'] / n, st['deaths'] / n, st['assists'] / n], f))
        # K/D/A after the first N rounds, from the per-round kill lists
        for cut in partial:
            if n <= cut:
                continue
            k = d = a = 0
            for rr in rounds[:cut]:
                for ps in rr.get('playerStats') or []:
                    for kill in ps.get('kills') or []:
                        k += kill.get('killer') == p['subject']
                        d += kill.get('victim') == p['subject']
                        a += p['subject'] in (kill.get('assistants') or [])
            partial[cut].append(([k / cut, d / cut, a / cut], f))

print(f'matches read {len(matches)}, players with a score {len(full)}, queues {queues}')
present = {k: sum(1 for m in matches.values() for p in m.get('players', []) if 'TempValue' + k in (p.get('scores') or {})) for k in 'AIJBCDEK'}
print('players carrying each scores.TempValue key:', present)
# per-round kill lists must add up to the match totals, or the partial fits mean nothing
ok = bad = 0
for m in matches.values():
    for p in m.get('players', []):
        k = d = a = 0
        for rr in m.get('roundResults') or []:
            for ps in rr.get('playerStats') or []:
                for kill in ps.get('kills') or []:
                    k += kill.get('killer') == p['subject']
                    d += kill.get('victim') == p['subject']
                    a += p['subject'] in (kill.get('assistants') or [])
        st = p['stats']
        if (k, d, a) == (st['kills'], st['deaths'], st['assists']):
            ok += 1
        else:
            bad += 1
print(f'per-round K/D/A equal to match totals: {ok} players, different: {bad}')
w1 = solve([r for r, _ in full], [y for _, y in full])
report('1 full breakdown (A I J B C D E K)', w1, [r for r, _ in full], [y for _, y in full], 'A I J B C D E K'.split())
w2 = solve([r for r, _ in live], [y for _, y in live])
report('2 final K/D/A per round', w2, [r for r, _ in live], [y for _, y in live], ['kills/r', 'deaths/r', 'assists/r'])
for cut, rows in partial.items():
    if rows:
        report(f'3 K/D/A after {cut} rounds, fit 2 applied', w2, [r for r, _ in rows], [y for _, y in rows], ['kills/r', 'deaths/r', 'assists/r'])
