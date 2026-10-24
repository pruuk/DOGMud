#!/usr/bin/env python3
"""Group a lighting golden's diff by sample section and biome.

Read-only. Never writes a golden; it only explains one that moved.

    python tools/lighting_golden_diff.py <before> <after>

<before> may be a git ref-ish path, in which case pipe it in yourself:

    git show HEAD:internal/guards/testdata/lighting_daycycle.golden > before.tmp
    python tools/lighting_golden_diff.py before.tmp internal/guards/testdata/lighting_daycycle.golden

Every golden move in the graded lighting arc must be explained room-for-room
before it is re-recorded. A move you cannot account for is a defect, however
plausible the totals look.
"""
import collections
import io
import re
import sys

ROOM = re.compile(r"^room (-?\d+) biome=(\S+) light=(-?\d+)")

# The PARITY golden is a different shape: no `== ` section headers, and each
# room is followed by one indented line per observer kind rather than a single
# light value.
#
#     room 75 biome=default
#       plain        sight=shapes clear=false shapes=true
#       nightvision  sight=full   clear=true shapes=true
#
# Task 4 of plan 3b fell back to a raw diff because this parser only knew the
# day-cycle format, and five later tasks move this file too. It is handled as
# one unnamed section whose value is the room's whole observer block, so a
# change to ANY sight tier shows up rather than only a light value.
PARITY_ROOM = re.compile(r"^room (-?\d+) biome=(\S+)\s*$")


def parse(path):
    """Return {section: {roomid: value}} and the section order.

    Auto-detects which golden this is. For the day-cycle golden a value is
    (biome, light); for the parity golden it is (biome, observer block).
    """
    text = io.open(path, encoding="utf-8").read()
    if "light=" in text:
        return parse_daycycle(text)
    return parse_parity(text)


def parse_daycycle(text):
    """Return {section: {roomid: (biome, light)}} and the section order."""
    sections, order, current = {}, [], None
    for line in text.splitlines():
        if line.startswith("== "):
            current = line
            sections[current] = {}
            order.append(current)
        elif current:
            m = ROOM.match(line)
            if m:
                sections[current][int(m.group(1))] = (m.group(2), int(m.group(3)))
    return sections, order


def parse_parity(text):
    """Return the parity golden as one section keyed by room id."""
    rooms, current = {}, None
    for line in text.splitlines():
        m = PARITY_ROOM.match(line)
        if m:
            current = int(m.group(1))
            rooms[current] = [m.group(2)]
        elif current is not None and line.startswith("  "):
            rooms[current].append(line.strip())
    section = "== parity (every observer kind)"
    packed = {r: (v[0], " | ".join(v[1:])) for r, v in rooms.items()}
    return {section: packed}, [section]


def main(before_path, after_path):
    before, order = parse(before_path)
    after, after_order = parse(after_path)

    if len(before) != len(after):
        print(f"SECTION COUNT CHANGED: {len(before)} -> {len(after)}")

    total = 0
    for section in order:
        b = before.get(section, {})
        a = after.get(section, {})
        if section not in after:
            print(f"{section}: SECTION REMOVED")
            continue

        gone = sorted(set(b) - set(a))
        added = sorted(set(a) - set(b))
        moved = [(r, b[r], a[r]) for r in sorted(set(b) & set(a)) if b[r] != a[r]]
        total += len(moved)

        if not (gone or added or moved):
            print(f"{section}: unchanged")
            continue

        print(f"{section}: {len(moved)} moved, {len(gone)} removed, {len(added)} added")
        kinds = collections.Counter(
            (old[0], new[0], old[1], new[1]) for _, old, new in moved
        )
        for (ob, nb, ol, nl), count in kinds.most_common():
            arrow = f"{ob} -> {nb}" if ob != nb else ob
            print(f"      {arrow:<28} {ol:>4} -> {nl:<4} x{count}")
        if gone:
            gone_biomes = collections.Counter(b[r][0] for r in gone)
            print(f"      removed by biome: {dict(gone_biomes)}")
        if added:
            add_biomes = collections.Counter(a[r][0] for r in added)
            print(f"      added by biome:   {dict(add_biomes)}")

    print(f"\nTOTAL room-readings moved: {total}")
    rooms_before = len(before[order[0]]) if order else 0
    rooms_after = len(after[after_order[0]]) if after_order else 0
    if rooms_before != rooms_after:
        print(f"ROOM COUNT CHANGED: {rooms_before} -> {rooms_after}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(__doc__)
        sys.exit(2)
    main(sys.argv[1], sys.argv[2])
