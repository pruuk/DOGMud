#!/usr/bin/env python3
"""Enumerate every player-facing text surface in the world data and Go tree.

M0 of the messaging unification arc. The arc exists because curated inventories
rot: a "verified" claim that quell had no messages survived two weeks past its
own fix, and a hand-built store list missed `idlemessages` -- 1,285 occurrences
and the largest narration surface in the game.

So this walks by PROPERTY, not by name. Anything that looks like it holds
player-facing text is reported, and `messaging_surface_guard_test.go` requires
every key spelling found here to be registered with a reason.

Note: `.template` files (help text, splash prose) are deliberately NOT scanned
here -- they are not YAML, and are covered by Method B (the ANSI-markup walk)
in a later task of this arc.

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
# surface. This is deliberately substring, not word-boundary, matching: some
# hits are coincidental (`iridescence` and `descent` both contain "desc"), but
# tightening the stems to whole-word matches would shrink recall on exactly
# the failure mode this tool exists to prevent -- a real key like `desc` or
# `subdesc` getting missed because it doesn't look like the word "description".
# Accept the noise; a false positive costs a registry line, a false negative
# is a silent miss.
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

# Keys appear at line start, after a sequence dash, and inside flow mappings.
# Multi-word keys are REAL: room `nouns:` blocks use author-chosen phrases like
# `hunt pool:`. Apostrophes occur too (`hunter's blind:`).
KEY_RE = re.compile(r"(?:^|[-{,])\s*([a-z_][a-z0-9_' -]*?)\s*:", re.IGNORECASE)

# Where a QUOTED VALUE begins. Everything after this on the line is prose, and
# a colon inside prose ("She said: run") is not a key.
VALUE_START_RE = re.compile(r":\s*[\"']")


def keys_in_line(line):
    """Yield lowercased key spellings found on one line."""
    m = VALUE_START_RE.search(line)
    head = line[: m.start() + 1] if m else line
    for km in KEY_RE.finditer(head):
        key = km.group(1).strip().lower()
        if key:
            yield key


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
                        for key in keys_in_line(line):
                            if is_candidate(key):
                                yield key, rel
            except OSError as exc:
                print(f"WARN: cannot read {rel}: {exc}", file=sys.stderr)


def collect():
    keys = defaultdict(lambda: {"count": 0, "dirs": set(), "files": set()})
    for key, rel in walk_yaml():
        entry = keys[key]
        entry["count"] += 1
        entry["dirs"].add(os.path.dirname(rel).replace("\\", "/"))
        entry["files"].add(rel)

    values = defaultdict(lambda: defaultdict(
        lambda: {"count": 0, "dirs": set(), "files": set()}))
    for method, key, rel in walk_values():
        entry = values[method][key]
        entry["count"] += 1
        entry["dirs"].add(os.path.dirname(rel).replace("\\", "/"))
        entry["files"].add(rel)

    return keys, values


def split_schema_content(keys):
    """Split keys into schema (found in 2+ files -- a loader reads it) vs.
    content (found in exactly 1 file -- most likely an author-invented noun,
    e.g. a room `nouns:` child). Author-chosen noun keys would otherwise
    demand 3,000+ registry lines; schema keys recur because a loader owns
    them, an invented noun appears once.
    """
    schema = {k: v for k, v in keys.items() if len(v["files"]) >= 2}
    content = {k: v for k, v in keys.items() if len(v["files"]) < 2}
    return schema, content


# Method B -- ANSI markup. A string carrying <ansi ...> is player-facing by
# construction, whatever its key is called and wherever it lives. This is the
# highest-signal detector we have, and unlike Method A it is not limited to
# YAML: .template files (help, splash, login prose) are read too.
ANSI_RE = re.compile(r"<ansi\s+[a-z]+=")

# Method C -- prose shape. A scalar that reads as a sentence is player-facing
# even when its key name gives nothing away. This is what must catch the room
# `nouns:` prose that Method A structurally cannot see. Deliberately
# conservative: four or more words AND terminal punctuation, so identifiers,
# tags and single-word enum values do not flood the report.
PROSE_WORDS = 4

# Extensions Method B reads. Method A is YAML-only by design; Method B is not.
ANSI_EXTS = (".yaml", ".yml", ".template")

# A YAML block-scalar opener: `key: |`, `key: >-`, `key: |2`, etc. Once a line
# like this is seen, every following line that is blank or indented MORE than
# this line is the scalar's VALUE, not new keys -- even if that value contains
# a colon (e.g. "You can specify which one: the blue or the green."). Without
# this, keys_in_line() misreads such a continuation line as a new key, which
# then becomes last_key and steals attribution from every line after it.
BLOCK_SCALAR_OPEN_RE = re.compile(r":\s*[|>][+\-]?\d*\s*(?:#.*)?$")


def _indent_of(line):
    return len(line) - len(line.lstrip(" "))


def looks_like_prose(value):
    v = value.strip().strip("\"'").strip()
    if not v.endswith((".", "!", "?", "…")):
        return False
    return len(v.split()) >= PROSE_WORDS


def walk_values():
    """Yield (method, key, repo_relative_path) for value-shaped detections.

    Method B fires on ANSI markup anywhere on the line. Method C fires when the
    value after a key's colon reads as a sentence. Both attribute to the key on
    that line, or to the nearest preceding key when the line is a list item or
    a block-scalar continuation -- which is how `nouns:` children and
    block-scalar prose get attributed to a real key rather than to nothing.

    Block-scalar aware: once a `key: |`/`key: >` line opens a block scalar, no
    key parsing happens on its continuation lines (blank, or indented more
    than the opening line) -- their detections attribute to the opening key
    instead. The block ends at the first non-blank line indented at or less
    than the opening line's indent, which is then parsed normally.
    """
    for root, dirs, files in os.walk(WORLD):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for name in files:
            if not name.endswith(ANSI_EXTS):
                continue
            full = os.path.join(root, name)
            rel = os.path.relpath(full, REPO).replace("\\", "/")
            try:
                with open(full, "r", encoding="utf-8", errors="replace") as fh:
                    last_key = None
                    in_block = False
                    block_key = None
                    block_indent = 0
                    for line in fh:
                        if in_block:
                            stripped = line.strip()
                            if stripped == "" or _indent_of(line) > block_indent:
                                attribute_to = block_key or "<no-key>"
                                if ANSI_RE.search(line):
                                    yield "ansi", attribute_to, rel
                                if looks_like_prose(line):
                                    yield "prose", attribute_to, rel
                                continue
                            in_block = False
                            # Not a continuation -- falls through and is
                            # parsed normally below.

                        line_keys = list(keys_in_line(line))
                        if line_keys:
                            last_key = line_keys[-1]
                        attribute_to = last_key or "<no-key>"
                        if ANSI_RE.search(line):
                            yield "ansi", attribute_to, rel
                        value = line.split(":", 1)[1] if ":" in line else line
                        if looks_like_prose(value):
                            yield "prose", attribute_to, rel

                        if BLOCK_SCALAR_OPEN_RE.search(line):
                            in_block = True
                            block_key = last_key
                            block_indent = _indent_of(line)
            except OSError as exc:
                print(f"WARN: cannot read {rel}: {exc}", file=sys.stderr)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--json", action="store_true", help="machine-readable output")
    args = ap.parse_args()

    if not os.path.isdir(WORLD):
        sys.exit(f"FATAL: world data not found at {WORLD}")

    keys, values = collect()
    if not keys:
        sys.exit("FATAL: no text-bearing keys found. The walk is broken, "
                  "not the data.")

    schema, content = split_schema_content(keys)

    if args.json:
        def _fmt(d):
            return {
                k: {
                    "count": v["count"],
                    "dirs": sorted(v["dirs"]),
                    "files": sorted(v["files"]),
                }
                for k, v in sorted(d.items())
            }

        def _fmt_dirs_only(d):
            return {k: sorted(v["dirs"]) for k, v in sorted(d.items())}

        def _fmt_values(d):
            out = {}
            for method, per_key in d.items():
                v_schema, v_content = split_schema_content(per_key)
                out[method] = {
                    "schema": _fmt_dirs_only(v_schema),
                    "content": _fmt_dirs_only(v_content),
                }
            return out

        out = {
            "schema": _fmt(schema),
            "content": _fmt(content),
            "values": _fmt_values(values),
        }
        print(json.dumps(out, indent=2, sort_keys=True))
        return

    print(f"TEXT-BEARING YAML KEYS ({len(keys)} distinct spellings)")
    print("=" * 72)
    print(f"SCHEMA KEYS ({len(schema)}) -- found in 2+ files, loader-owned")
    print("-" * 72)
    for key, v in sorted(schema.items(), key=lambda kv: -kv[1]["count"]):
        dirs = sorted(v["dirs"])
        shown = ", ".join(d.split("/")[-1] for d in dirs[:3])
        more = f" +{len(dirs) - 3} more" if len(dirs) > 3 else ""
        print(f"{v['count']:6d}  {key:<24} {shown}{more}")

    print()
    print(f"CONTENT KEYS ({len(content)}) -- found in exactly 1 file, "
          f"author-invented (e.g. room `nouns:` children)")
    print("-" * 72)
    for key, v in sorted(content.items(), key=lambda kv: kv[0])[:15]:
        f = sorted(v["files"])[0]
        print(f"{v['count']:6d}  {key:<24} {f}")
    if len(content) > 15:
        print(f"  ... and {len(content) - 15} more")

    for method, label in (
        ("ansi", "METHOD B — keys carrying ANSI markup"),
        ("prose", "METHOD C — keys whose values read as prose"),
    ):
        hits = values.get(method, {})
        v_schema, v_content = split_schema_content(hits)
        print()
        print(label)
        print("=" * 72)
        print(f"SCHEMA KEYS ({len(v_schema)}) -- found in 2+ files, loader-owned")
        print("-" * 72)
        for key, v in sorted(v_schema.items(), key=lambda kv: -kv[1]["count"]):
            dirs = sorted(v["dirs"])
            shown = ", ".join(d.split("/")[-1] for d in dirs[:3])
            more = f" +{len(dirs) - 3} more" if len(dirs) > 3 else ""
            print(f"{v['count']:6d}  {key:<28} {shown}{more}")

        print()
        total_occ = sum(v["count"] for v in v_content.values())
        examples = ", ".join(sorted(v_content)[:10])
        print(f"CONTENT: {len(v_content)} distinct keys, {total_occ} total "
              f"occurrences, found in exactly 1 file each")
        if examples:
            print(f"  examples: {examples}")


if __name__ == "__main__":
    main()
