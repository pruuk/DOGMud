#!/usr/bin/env python3
"""Enumerate every player-facing text surface in the world data and Go tree.

M0 of the messaging unification arc. The arc exists because curated inventories
rot: a "verified" claim that quell had no messages survived two weeks past its
own fix, and a hand-built store list missed `idlemessages` -- 1,285 occurrences
and the largest narration surface in the game.

So this walks by PROPERTY, not by name. Anything that looks like it holds
player-facing text is reported, and `messaging_surface_guard_test.go` requires
every key spelling found here to be registered with a reason.

Usage:
    python tools/messaging_surface_audit.py            # human report
    python tools/messaging_surface_audit.py --json     # machine inventory

Filename-only YAML scanning, no yaml library needed -- same approach as
tools/id_inventory.py, and deliberate: a real parse would need every loader's
schema, and the point is to find keys no loader owns yet.
"""

import argparse
import json
import os
import re
import sys
from collections import defaultdict

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WORLD = os.path.join(REPO, "_datafiles", "world", "dogmud")

# Directories that are runtime state, not authored content. Instance saves
# mirror templates, user saves are per-player, shops/guilds/moderation are
# living state (see CLAUDE.md).
SKIP_DIRS = {
    "mobs.instances", "rooms.instances", "users", "shops",
    "guilds", "moderation", "plugin-data", "warehouses",
}

# A key is a text candidate if its name contains any of these stems. Broad on
# purpose: over-reporting costs a registry line, under-reporting hides a
# surface.
KEY_STEMS = (
    "text", "message", "msg", "lines", "hint", "prose", "desc",
    "say", "emote", "voice", "phrase", "greeting", "taunt",
)

# Audience/role keys carry no stem but ARE the narration shape.
AUDIENCE_KEYS = {
    "toattacker", "todefender", "toroom", "observers",
    "controller", "controlled", "together", "separate",
    "options", "optionid",
}

KEY_RE = re.compile(r"^\s*([a-z_][a-z0-9_]*):", re.IGNORECASE)


def is_candidate(key):
    k = key.lower()
    if k in AUDIENCE_KEYS:
        return True
    return any(stem in k for stem in KEY_STEMS)


def walk_yaml():
    """Yield (key, repo_relative_path) for every candidate key occurrence."""
    for root, dirs, files in os.walk(WORLD):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for name in files:
            if not name.endswith((".yaml", ".yml")):
                continue
            full = os.path.join(root, name)
            rel = os.path.relpath(full, REPO).replace("\\", "/")
            try:
                with open(full, "r", encoding="utf-8", errors="replace") as fh:
                    for line in fh:
                        m = KEY_RE.match(line)
                        if m and is_candidate(m.group(1)):
                            yield m.group(1).lower(), rel
            except OSError as exc:
                print(f"WARN: cannot read {rel}: {exc}", file=sys.stderr)


def collect():
    keys = defaultdict(lambda: {"count": 0, "dirs": set()})
    for key, rel in walk_yaml():
        entry = keys[key]
        entry["count"] += 1
        entry["dirs"].add(os.path.dirname(rel).replace("\\", "/"))
    return keys


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--json", action="store_true", help="machine-readable output")
    args = ap.parse_args()

    keys = collect()
    if args.json:
        out = {
            k: {"count": v["count"], "dirs": sorted(v["dirs"])}
            for k, v in sorted(keys.items())
        }
        print(json.dumps(out, indent=2, sort_keys=True))
        return

    print(f"TEXT-BEARING YAML KEYS ({len(keys)} distinct spellings)")
    print("=" * 72)
    for key, v in sorted(keys.items(), key=lambda kv: -kv[1]["count"]):
        dirs = sorted(v["dirs"])
        shown = ", ".join(d.split("/")[-1] for d in dirs[:3])
        more = f" +{len(dirs) - 3} more" if len(dirs) > 3 else ""
        print(f"{v['count']:6d}  {key:<24} {shown}{more}")


if __name__ == "__main__":
    main()
