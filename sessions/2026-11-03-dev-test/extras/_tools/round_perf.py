# Valorant's own per-round performance telemetry ("End of Round Perf statistics" in ShooterGame.log):
# 83 numbers per round - frame time, FPS histograms, engine latency, GPU time, Valorant and system CPU, memory.
# "Valorant CPU Util" is per core and "System CPU Util" is of the whole PC, so divide the first by the
# logical processor count before comparing them.
# usage: python -I round_perf.py "%LOCALAPPDATA%\VALORANT\Saved\Logs\ShooterGame.log" <out.json>
import json, re, sys
from datetime import datetime, timedelta

LOCAL_OFFSET_HOURS = 2
log, out = sys.argv[1], sys.argv[2]
rounds, cur, fields, match = [], None, [], 0
for line in open(log, encoding="utf-8", errors="replace"):
    if "Loopstate changed from" in line and "to INGAME" in line:
        match += 1
    stamp = re.match(r"^\[(\d{4})\.(\d\d)\.(\d\d)-(\d\d)\.(\d\d)\.(\d\d)", line)
    if "End of Round Perf statistics" in line:
        when = datetime(*map(int, stamp.groups())) + timedelta(hours=LOCAL_OFFSET_HOURS) if stamp else None
        cur = {"match": match, "time": when.strftime("%H:%M:%S") if when else ""}
        rounds.append(cur)
        continue
    if cur is None:
        continue
    if line.startswith("[") and "LogClientPerRoundTelemetryComponent" not in line:
        cur = None
        continue
    number = re.match(r"^\s+Round (\d+) Performance", line)
    kv = re.match(r"^\s+(.+?):\s*([-0-9.]+)\s*$", line)
    if number:
        cur["round"] = int(number.group(1))
    elif kv:
        cur[kv.group(1)] = float(kv.group(2))
        if kv.group(1) not in fields:
            fields.append(kv.group(1))
json.dump({"fields": fields, "rounds": rounds}, open(out, "w"), indent=1)
print(f"{len(rounds)} rounds, {len(fields)} fields per round -> {out}")
