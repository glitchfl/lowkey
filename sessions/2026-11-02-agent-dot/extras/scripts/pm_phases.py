"""Summarise a PresentMon 2.x CSV by phase and per second.

usage: python -I pm_phases.py <csv> <offset_hours> <per_second_out> <phase>=<HH:MM:SS.fff>-<HH:MM:SS.fff> ...
Times in the CSV (TimeInDateTime) are shifted by -offset_hours to local time.
"""
import csv
import statistics
import sys
from collections import Counter, defaultdict


def parse_dt(s):
    # "2026-11-2 15:16:28.350251200"
    _, t = s.split(" ")
    h, m, rest = t.split(":")
    return int(h) * 3600 + int(m) * 60 + float(rest)


def parse_hms(s):
    h, m, sec = s.split(":")
    return int(h) * 3600 + int(m) * 60 + float(sec)


def fmt(t):
    h = int(t // 3600)
    m = int((t % 3600) // 60)
    s = t % 60
    return f"{h:02d}:{m:02d}:{s:06.3f}"


def pct(xs, p):
    if not xs:
        return float("nan")
    xs = sorted(xs)
    k = min(len(xs) - 1, max(0, int(round(p / 100 * (len(xs) - 1)))))
    return xs[k]


def main():
    path, offset, per_second_out = sys.argv[1], float(sys.argv[2]), sys.argv[3]
    phases = []
    for arg in sys.argv[4:]:
        name, span = arg.split("=", 1)
        a, b = span.split("-")
        phases.append((name, parse_hms(a), parse_hms(b)))

    modes = defaultdict(Counter)
    ft = defaultdict(list)
    swapchains = defaultdict(set)
    per_sec = defaultdict(lambda: {"n": 0, "ft": [], "modes": Counter()})
    transitions = []
    last_mode = None
    first = last = None
    rows = 0
    with open(path, newline="") as f:
        for row in csv.DictReader(f):
            try:
                t = parse_dt(row["TimeInDateTime"]) - offset * 3600
                mbp = float(row["MsBetweenPresents"])
            except (ValueError, KeyError):
                continue
            rows += 1
            first = t if first is None else first
            last = t
            mode = row["PresentMode"]
            if mode != last_mode:
                transitions.append((t, last_mode, mode))
                last_mode = mode
            sec = int(t)
            ps = per_sec[sec]
            ps["n"] += 1
            ps["ft"].append(mbp)
            ps["modes"][mode] += 1
            for name, a, b in phases:
                if a <= t < b:
                    modes[name][mode] += 1
                    ft[name].append(mbp)
                    swapchains[name].add(row["SwapChainAddress"])

    print(f"rows={rows} first={fmt(first)} last={fmt(last)} (local)")
    print()
    for name, a, b in phases:
        xs = ft[name]
        print(f"[{name}] {fmt(a)} - {fmt(b)}  frames={len(xs)}  swapchains={len(swapchains[name])}")
        for mode, n in modes[name].most_common():
            print(f"    {mode}: {n}")
        if xs:
            print(
                "    MsBetweenPresents median={:.2f} p95={:.2f} p99={:.2f} max={:.2f} (~{:.0f} fps)".format(
                    statistics.median(xs), pct(xs, 95), pct(xs, 99), max(xs), 1000 / statistics.median(xs)
                )
            )
    print()
    print(f"present mode changes: {len(transitions) - 1}")
    for t, a, b in transitions[:40]:
        print(f"    {fmt(t)}  {a} -> {b}")

    with open(per_second_out, "w", newline="") as out:
        w = csv.writer(out)
        w.writerow(["local_time", "frames", "present_modes", "ms_between_presents_median", "p99", "max"])
        for sec in sorted(per_sec):
            ps = per_sec[sec]
            w.writerow([
                fmt(sec)[:8],
                ps["n"],
                ";".join(f"{m}={n}" for m, n in ps["modes"].most_common()),
                f"{statistics.median(ps['ft']):.2f}",
                f"{pct(ps['ft'], 99):.2f}",
                f"{max(ps['ft']):.2f}",
            ])


if __name__ == "__main__":
    main()
