#!/usr/bin/env python3
"""Generate blank housing unit rooms for a lodging building.

A unit is an ordinary authored room with one exit, named after the building's
door, leading back to the door room. The unit has no inbound exit: players
reach it only through internal/housing's door router. Units sit on their own
plane with distinct x so no two share a cell, and the `door` exit has no map
direction, so the mapper treats each unit as a one-room map.

Writes new files only. It refuses to overwrite a room that already exists, so
it is safe to run again with a wider range to add capacity. After generating,
add the new ids to the building's unit_rooms in housing_buildings/.

Usage:
  python tools/housing_units.py --zone-folder new_plymouth_lodgings \\
      --zone "New Plymouth Lodgings" --first 6470 --count 24 \\
      --door-room 5625 --door-exit door --plane 14
"""
import argparse
import os
import sys

ROOT = os.path.join(os.path.dirname(__file__), "..", "_datafiles", "world", "dogmud", "rooms")

UNIT = """roomid: {roomid}
zone: {zone}
title: A Bare Lodging Room
description: >
  Four whitewashed walls, a plank floor scrubbed pale, and a single shuttered
  <ansi fg="itemname">window</ansi> that lets a stripe of light in across
  the boards. Nothing else. The room is empty, and it is yours, and there is
  nothing in it yet but what you bring. The green
  <ansi fg="itemname">door</ansi> behind you opens back onto the alley.
biome: interior
exits:
  {door_exit}:
    roomid: {door_room}
nouns:
  window: A small window with a pair of wooden shutters, latched from the
    inside. Through the slats you can see a strip of washing line and a
    slice of the building opposite.
  door: The inside of the green door. On this side it has an ordinary iron
    latch, worn smooth. It opens for you, and it lets you back into the
    alley whenever you like.
x: {x}
y: 0
z: 0
plane: {plane}
"""

ZONE_CONFIG = """name: {zone}
roomid: {first}
defaultbiome: interior
region: {region}
non_cartesian: true
"""


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--zone-folder", required=True)
    p.add_argument("--zone", required=True)
    p.add_argument("--region", default="Windward Marches")
    p.add_argument("--first", type=int, required=True)
    p.add_argument("--count", type=int, required=True)
    p.add_argument("--door-room", type=int, required=True)
    p.add_argument("--door-exit", default="door")
    p.add_argument("--plane", type=int, required=True)
    a = p.parse_args()

    folder = os.path.join(ROOT, a.zone_folder)
    os.makedirs(folder, exist_ok=True)

    zc = os.path.join(folder, "zone-config.yaml")
    if not os.path.exists(zc):
        with open(zc, "x") as f:
            f.write(ZONE_CONFIG.format(zone=a.zone, first=a.first, region=a.region))
        print("wrote", zc)

    made = 0
    for i in range(a.count):
        roomid = a.first + i
        path = os.path.join(folder, f"{roomid}.yaml")
        if os.path.exists(path):
            print("exists, skipped", path, file=sys.stderr)
            continue
        with open(path, "x") as f:
            f.write(UNIT.format(roomid=roomid, zone=a.zone, door_exit=a.door_exit,
                                door_room=a.door_room, x=i, plane=a.plane))
        made += 1
    print(f"wrote {made} unit rooms {a.first}-{a.first + a.count - 1}")


if __name__ == "__main__":
    main()
