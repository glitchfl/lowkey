# For every match found: when agent select loaded, when you picked, when the agent dots went and why,
# when lowkey's pregame GET first showed your pick, and the first ares-pregame event after the pick.
# usage: python -I pick_latency.py <agent-select-timeline.log or ShooterGame.log> <record.jsonl>...
# Game log stamps are UTC; lowkey's are local. LOCAL_OFFSET_HOURS lines them up (the PC was UTC+2).
import json, re, sys
from datetime import datetime, timedelta

LOCAL_OFFSET_HOURS = 2
SELF = "5e1f5e1f-5e1f-4e1f-8e1f-5e1f5e1f5e1f"  # the scrubber's alias for you


def local_time(s):
    m = re.match(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d+))?", s)
    return datetime.fromisoformat(m.group(1)) + timedelta(microseconds=int((m.group(2) or "0").ljust(6, "0")[:6]))


def game_time(line):
    m = re.match(r"\[(\d{4})\.(\d\d)\.(\d\d)-(\d\d)\.(\d\d)\.(\d\d):(\d{3})", line)
    return datetime(*map(int, m.groups()[:6]), int(m.group(7)) * 1000) + timedelta(hours=LOCAL_OFFSET_HOURS)


glog, recs = sys.argv[1], sys.argv[2:]
found, loaded, picks = [], [], []
for line in open(glog, encoding="utf-8"):
    if "to PREGAME" in line:
        found.append(game_time(line))
    elif "TransitionToPregame to Pregame" in line:
        loaded.append(game_time(line))
    elif "Pregame_SelectCharacter" in line:
        picks.append(game_time(line))
found = sorted(set(found))

fetches, events, dots = [], [], []
for path in recs:
    for line in open(path, encoding="utf-8"):
        head = line[:400]
        if '"kind":"dot"' in head:
            r = json.loads(line)
            dots.append((local_time(r["time"]), r["body"]["shown"], r["body"]["why"]))
        elif '"kind":"event"' in head and "ares-pregame" in line[:3000]:
            events.append(local_time(json.loads(line)["time"]))
        elif '"kind":"remote"' in head and "/pregame/v1/matches/" in head:
            r = json.loads(line)
            me = [p for p in (r.get("body") or {}).get("AllyTeam", {}).get("Players", []) if p.get("Subject") == SELF]
            fetches.append((local_time(r["time"]), bool(me and me[0].get("CharacterSelectionState"))))

print(f"{'found':9} {'loaded':>7} {'pick':>7} {'dots gone':>19} {'lowkey saw':>11} {'1st event':>10}  {'delay':>6}")
for f in found:
    later = [x for x in found if x > f]
    end = later[0] if later else f + timedelta(minutes=3)
    ld = next((x for x in loaded if f < x < end), None)
    pk = next((x for x in picks if f < x < end), None)
    gone = next(((t, why) for t, shown, why in dots if not shown and f < t < end), None)
    seen = next((t for t, sel in fetches if sel and pk and pk < t < end), None)
    ev = next((t for t in events if pk and pk < t < end), None)
    rel = lambda x: f"+{(x - f).total_seconds():.1f}" if x else "-"
    delay = f"{(seen - pk).total_seconds():.1f}s" if seen and pk else "-"
    print(f"{f:%H:%M:%S}  {rel(ld):>7} {rel(pk):>7} {rel(gone[0]) if gone else '-':>7} {gone[1] if gone else '':<11} {rel(seen):>11} {rel(ev):>10}  {delay:>6}")
