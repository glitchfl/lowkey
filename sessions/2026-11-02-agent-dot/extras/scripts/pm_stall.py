import csv, sys
def t(s):
    h, m, r = s.split(" ")[1].split(":")
    return int(h) * 3600 + int(m) * 60 + float(r) - 2 * 3600
def f(x):
    return "%02d:%02d:%06.3f" % (x // 3600, (x % 3600) // 60, x % 60)
rows = []
for row in csv.DictReader(open(sys.argv[1], newline="")):
    try:
        x = t(row["TimeInDateTime"])
    except (ValueError, KeyError):
        continue
    rows.append((x, row))
cols = ["MsBetweenPresents", "MsInPresentAPI", "MsCPUBusy", "MsCPUWait", "MsGPUTime", "MsUntilDisplayed", "MsBetweenDisplayChange", "PresentMode"]
print("time         " + " ".join(c[:14].rjust(14) for c in cols))
for target in (13*3600+17*60+55.939, 13*3600+17*60+59.586, 13*3600+18*60+40.845):
    for i, (x, row) in enumerate(rows):
        if abs(x - target) < 0.0005:
            for j in range(max(0, i - 3), min(len(rows), i + 4)):
                xx, rr = rows[j]
                print(f(xx) + " " + " ".join(str(rr[c])[:14].rjust(14) for c in cols))
            print()
            break
