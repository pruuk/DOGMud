#!/usr/bin/env python3
"""Ambient telemetry report (#317).

Reads a DOGMud server log with Balance.AmbientTelemetry on and prints, as
Markdown:
  1. lines per listener-minute for each room and source, worst first, with
     offered, delivered and suppressed counts (per listener);
  2. each room's total across sources, worst first;
  3. mob moves per minute per zone (idle, tracking, path).

The log can be the playtest's tools/playtest/.run/<run_id>/server.log, a
`docker logs <container>` dump, or a live server's log file. Colour codes
are stripped, so either mudlog handler's output parses.

The server numbers its report windows from 1 on every boot. A log that spans
a restart is counted by distinct (boot, window) pairs: a "Server Ready" line
(logged once per process), or a window number that falls, starts a new boot.
"Minutes measured" counts the windows that had at least one active room; a
fully quiet window writes no lines, so it is not counted.

SOURCES and MOVES below mirror ambient.Sources and ambient.MoveKinds in
internal/ambient and must be kept in step with them.

Usage:
  python -I tools/ambient_report.py <server.log>
  python -I tools/ambient_report.py --self-test
"""
import argparse
import re
import sys

ANSI = re.compile(r"\x1b\[[0-9;]*m")
PAIR = re.compile(r'(\w+)=("(?:[^"\\]|\\.)*"|\S+)')
MARK = "ambient.telemetry"
# main.go logs this once per process, the same text in both log handlers.
BOOT_MARK = "Server Ready"
SOURCES = ["room_idle", "mob_chatter", "weather", "gossip", "item_chatter", "say_echo"]
MOVES = ["move_idle", "move_track", "move_path"]


def parse_line(line):
    line = ANSI.sub("", line)
    if MARK not in line:
        return None
    rec = {}
    for key, val in PAIR.findall(line):
        if len(val) >= 2 and val[0] == '"' and val[-1] == '"':
            val = val[1:-1]
        rec[key] = val
    if "room" not in rec or "window" not in rec:
        return None
    return rec


def num(rec, key):
    try:
        return int(rec.get(key, "0"))
    except ValueError:
        return 0


def read_records(lines):
    """Parse the telemetry lines, tagging each with its boot number."""
    records = []
    boot = 0
    last_window = None
    for line in lines:
        if BOOT_MARK in ANSI.sub("", line):
            boot += 1
            last_window = None
            continue
        rec = parse_line(line)
        if not rec:
            continue
        window = num(rec, "window")
        if last_window is not None and window < last_window:
            boot += 1
        last_window = window
        rec["boot"] = boot
        records.append(rec)
    return records


def build(records):
    rooms = {}
    zones = {}
    windows = set()
    window_seconds = 0
    for rec in records:
        secs = num(rec, "round_seconds") or 4
        windows.add((rec.get("boot", 0), rec["window"]))
        window_seconds = max(window_seconds, num(rec, "window_rounds") * secs)
        room = rooms.setdefault(rec["room"], {
            "zone": rec.get("zone", ""),
            "listener_seconds": 0,
            "src": {s: [0, 0, 0] for s in SOURCES},
        })
        room["listener_seconds"] += num(rec, "listener_rounds") * secs
        for s in SOURCES:
            room["src"][s][0] += num(rec, s + "_offered")
            room["src"][s][1] += num(rec, s + "_delivered")
            room["src"][s][2] += num(rec, s + "_suppressed")
        zone = zones.setdefault(rec.get("zone", ""), {m: 0 for m in MOVES})
        for m in MOVES:
            zone[m] += num(rec, m)
    minutes = len(windows) * window_seconds / 60.0
    return rooms, zones, minutes


def render(rooms, zones, minutes):
    out = []
    rows = []
    totals = []
    for room_id, room in rooms.items():
        lm = room["listener_seconds"] / 60.0
        if lm <= 0:
            continue
        all_off = all_del = all_sup = 0
        for s in SOURCES:
            off, dl, sup = room["src"][s]
            all_off, all_del, all_sup = all_off + off, all_del + dl, all_sup + sup
            if off == 0:
                continue
            rows.append((dl / lm, room_id, room["zone"], s, lm, off, dl, sup))
        totals.append((all_del / lm, room_id, room["zone"], lm, all_off, all_del, all_sup))
    rows.sort(key=lambda r: (-r[0], r[1], r[3]))
    totals.sort(key=lambda r: (-r[0], r[1]))

    out.append("## Ambient lines per listener-minute (worst first)")
    out.append("")
    out.append("| Room | Zone | Source | Listener-min | Offered | Delivered | Suppressed | Delivered per listener-min |")
    out.append("|---|---|---|---|---|---|---|---|")
    for rate, room_id, zone, s, lm, off, dl, sup in rows:
        out.append(f"| {room_id} | {zone} | {s} | {lm:.1f} | {off} | {dl} | {sup} | {rate:.2f} |")
    out.append("")
    out.append("## Rooms, all sources (worst first)")
    out.append("")
    out.append("| Room | Zone | Listener-min | Offered | Delivered | Suppressed | Delivered per listener-min |")
    out.append("|---|---|---|---|---|---|---|")
    for rate, room_id, zone, lm, off, dl, sup in totals:
        out.append(f"| {room_id} | {zone} | {lm:.1f} | {off} | {dl} | {sup} | {rate:.2f} |")
    out.append("")
    out.append(f"## Mob moves per minute by zone ({minutes:.0f} minutes measured)")
    out.append("")
    out.append("| Zone | Idle | Tracking | Path | Total |")
    out.append("|---|---|---|---|---|")
    zrows = sorted(zones.items(), key=lambda kv: (-sum(kv[1].values()), kv[0]))
    for zone, z in zrows:
        total = sum(z.values())
        if minutes <= 0 or total == 0:
            continue
        idle, track, path = (z[m] / minutes for m in MOVES)
        out.append(f"| {zone} | {idle:.2f} | {track:.2f} | {path:.2f} | {total / minutes:.2f} |")
    return "\n".join(out) + "\n"


def report(lines):
    return render(*build(read_records(lines)))


def _fixture_line(colour, **kv):
    """One line as the server writes it: the colour handler (strings quoted,
    ANSI around each value) or the plain slog text handler."""
    parts = []
    for k, v in kv.items():
        val = f'"{v}"' if isinstance(v, str) else str(v)
        parts.append(f"{k}=\x1b[31;49m{val}\x1b[0m " if colour else f"{k}={val} ")
    if colour:
        return "\x1b[90m[12:00:00]\x1b[0m \x1b[32mINFO: \x1b[0m \x1b[36mambient.telemetry\x1b[39;49m " + "".join(parts)
    return "time=12:00:00 level=INFO msg=ambient.telemetry " + "".join(parts)


def self_test():
    base = {s + k: 0 for s in SOURCES for k in ("_offered", "_delivered", "_suppressed")}
    base.update({m: 0 for m in MOVES})

    def rec(**kv):
        d = dict(base)
        d.update(kv)
        return d

    fixture = [
        "unrelated line=1",
        _fixture_line(True, **rec(window=1, room=472, zone="Thornwall City", round_seconds=4, window_rounds=75,
                                  listener_rounds=75, mob_chatter_offered=12, mob_chatter_delivered=10,
                                  mob_chatter_suppressed=2, room_idle_offered=2, room_idle_delivered=2)),
        _fixture_line(False, **rec(window=1, room=5003, zone="Instance Planar Oasis", round_seconds=4,
                                   window_rounds=75, listener_rounds=0, move_idle=6, move_path=4)),
        _fixture_line(False, **rec(window=2, room=472, zone="Thornwall City", round_seconds=4, window_rounds=75,
                                   listener_rounds=75, mob_chatter_offered=8, mob_chatter_delivered=8)),
        _fixture_line(True, **rec(window=2, room=5003, zone="Instance Planar Oasis", round_seconds=4,
                                  window_rounds=75, listener_rounds=0, move_idle=4)),
    ]
    out = report(fixture)
    want = [
        "| 472 | Thornwall City | mob_chatter | 10.0 | 20 | 18 | 2 | 1.80 |",
        "| 472 | Thornwall City | room_idle | 10.0 | 2 | 2 | 0 | 0.20 |",
        "| 472 | Thornwall City | 10.0 | 22 | 20 | 2 | 2.00 |",
        "## Mob moves per minute by zone (10 minutes measured)",
        "| Instance Planar Oasis | 1.00 | 0.00 | 0.40 | 1.40 |",
    ]
    missing = [w for w in want if w not in out]
    order_ok = out.find(want[0]) < out.find(want[1])
    if missing or not order_ok:
        print("self-test FAILED")
        for w in missing:
            print("  missing: " + w)
        if not order_ok:
            print("  rows are not worst first")
        print(out)
        return 1

    # A log spanning restarts. Windows run 1, 2, then 1 again after a restart
    # (the window number fell), then 1 again after a Server Ready line (the
    # restart came before the number could fall). That is four distinct
    # (boot, window) pairs of 5 minutes each, with 4 + 4 + 4 + 4 idle moves.
    def moves(window):
        return _fixture_line(False, **rec(window=window, room=5003, zone="Instance Planar Oasis",
                                          round_seconds=4, window_rounds=75, listener_rounds=0, move_idle=4))

    ready = 'time=12:00:00 level=INFO msg="Server Ready" "Time Taken"=2.1s'
    restart = [moves(1), moves(2), moves(1), ready, moves(1)]
    rout = report(restart)
    rwant = "| Instance Planar Oasis | 0.80 | 0.00 | 0.00 | 0.80 |"
    if "## Mob moves per minute by zone (20 minutes measured)" not in rout or rwant not in rout:
        print("self-test FAILED (restart)")
        print(rout)
        return 1

    # The plain slog text handler: msg is quoted and padded, strings are
    # quoted only when they need it, and an empty string is "".
    def plain(window, room, zone, **kv):
        d = rec(window=window, room=room, round_seconds=4, window_rounds=75, listener_rounds=75)
        d.update(kv)
        d.pop("window"), d.pop("room")
        tail = "".join(f"{k}={v} " for k, v in d.items())
        return (f'time=12:00:00 level=INFO msg="ambient.telemetry       " window={window} room={room} '
                f'zone={zone} ' + tail)

    pfix = [
        plain(1, 472, "Plain", mob_chatter_offered=3, mob_chatter_delivered=3),
        plain(1, 473, '"Thornwall City"', weather_offered=5, weather_delivered=4, weather_suppressed=1),
        plain(1, 474, '""', gossip_offered=2, gossip_delivered=2),
        'time=12:00:00 level=INFO msg="Server Ready" "Time Taken"=2.1s',
        plain(1, 472, "Plain", mob_chatter_offered=3, mob_chatter_delivered=3),
    ]
    pout = report(pfix)
    pwant = [
        "| 472 | Plain | mob_chatter | 10.0 | 6 | 6 | 0 | 0.60 |",
        "| 473 | Thornwall City | weather | 5.0 | 5 | 4 | 1 | 0.80 |",
        "| 474 |  | gossip | 5.0 | 2 | 2 | 0 | 0.40 |",
    ]
    pmissing = [w for w in pwant if w not in pout]
    # Room 472 is heard in two boots (10 listener-minutes); the other rooms
    # in one (5). Two distinct (boot, window) pairs of 5 minutes make 10.
    if pmissing or "(10 minutes measured)" not in pout:
        print("self-test FAILED (plain)")
        for w in pmissing:
            print("  missing: " + w)
        print(pout)
        return 1
    print("self-test ok")
    return 0


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("log", nargs="?", help="server log to read")
    ap.add_argument("--self-test", action="store_true", help="run the built-in fixture check")
    args = ap.parse_args()
    if args.self_test:
        return self_test()
    if not args.log:
        ap.error("give a log file or --self-test")
    with open(args.log, encoding="utf-8", errors="replace") as f:
        sys.stdout.write(report(f))
    return 0


if __name__ == "__main__":
    sys.exit(main())
