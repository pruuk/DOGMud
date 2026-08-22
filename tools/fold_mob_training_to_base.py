#!/usr/bin/env python3
"""Fold authored mob `training:` into `base:` (U10b-0 Phase A, task A3).

WHY
---
`StatInfo.Training` becomes the progression curve's rank input in U10b-0, i.e.
"points gained since spawn". 599 of 641 mob templates author stat values there
instead, so without this fold a mob would start partway down the decay curve and
Phase C's gain cap would freeze the ~15 mobs whose authored value already exceeds
it, at spawn.

THE ARITHMETIC (per mob, per stat)
----------------------------------
    base_new = (that stat's existing `base:`, else its species base) + training

then the `training:` line goes away. This is value-neutral: the loader hydrates
`Base` from the species record only when `Base == 0` and then adds authored
`training`, so afterwards `base:` carries both and hydration correctly skips it.

`training: 0` is deleted rather than pinned, so that stat keeps tracking future
species rebalances. If that empties the stat key (or the whole `stats:` block),
the now-childless key goes too -- a bare `dexterity:` parses as null and is
equivalent, but it reads like an authoring mistake.

Two stats fold to exactly zero, both authored as the negation of their species
baseline (a scrubland dog's willpower against 15, a scavenger bird's vitality
against 10). `base: 0` only expresses that because Phase A also taught species
hydration to key on whether a `base:` key was authored rather than on
`Base == 0` -- see stats.StatInfo.BaseAuthored and characters.Validate pass 1.
Before that change, writing `base: 0` would have handed both stats their
baseline straight back.

HOW, AND WHY NOT A YAML LIBRARY
-------------------------------
Line-based transform, following `tools/id_inventory.py`. Round-tripping through
a YAML library reformats every file and destroys `#` comments -- 21 comment
lines live inside these stat blocks, and three of them are the only record of why
a training dummy's vitality was dropped by 185.

It also never reads and writes the same path: it reads `X.yaml` and writes
`X.yaml.new`, leaving the move to a separate step. A Python read-modify-write
truncates before the write expression is evaluated and has destroyed files in
this repo twice.

USAGE
-----
    python tools/fold_mob_training_to_base.py --dry-run > report.csv
    python tools/fold_mob_training_to_base.py            # writes *.yaml.new
    python tools/fold_mob_training_to_base.py --verify    # after the move

Run from the repo root.
"""

import argparse
import csv
import os
import re
import sys

MOB_ROOT = os.path.join("_datafiles", "world", "dogmud", "mobs")
SPECIES_ROOT = os.path.join("_datafiles", "world", "dogmud", "species")

STAT_NAMES = ("strength", "dexterity", "perception", "vitality", "willpower", "charisma")

# YAML key -> the entry-dict slot it fills. `training` is stored under "train"
# so the line-index slot can be "train_i" without colliding.
SLOT = {"base": "base", "training": "train"}

# `  stats:` / `  speciesid: N` sit at indent 2 under the top-level `character:`.
RE_STATS_KEY = re.compile(r"^(\s*)stats:\s*$")
RE_SPECIESID = re.compile(r"^\s*speciesid:\s*(\d+)\s*$")
# Block form: `    strength:` with the value lines nested under it.
RE_STAT_KEY = re.compile(r"^(\s*)([a-z]+):\s*$")
RE_STAT_VALUE = re.compile(r"^(\s*)(base|training):\s*(-?\d+)\s*$")
# Flow form: `    strength:    {training: 35}` -- six files use this.
RE_STAT_FLOW = re.compile(r"^(\s*)([a-z]+):(\s*)\{\s*(base|training):\s*(-?\d+)\s*\}\s*$")


def die(msg):
    sys.stderr.write("ERROR: %s\n" % msg)
    sys.exit(1)


def load_species_bases():
    """Return {speciesid: {stat: base}} parsed line-by-line.

    A species with no `stats:` block (20-orb is real) contributes 0 for every
    stat; that is not an error.
    """
    out = {}
    for name in sorted(os.listdir(SPECIES_ROOT)):
        if not name.endswith(".yaml"):
            continue
        path = os.path.join(SPECIES_ROOT, name)
        with open(path, encoding="utf-8") as fh:
            lines = fh.read().splitlines()

        species_id = None
        bases = {}
        in_stats = False
        stat_indent = None
        current = None
        for line in lines:
            m = RE_SPECIESID.match(line)
            if m and species_id is None:
                species_id = int(m.group(1))
                continue
            m = RE_STATS_KEY.match(line)
            if m and len(m.group(1)) == 0:
                in_stats = True
                stat_indent = None
                current = None
                continue
            if not in_stats:
                continue
            if line.strip() and not line.startswith(" "):
                in_stats = False
                continue
            m = RE_STAT_KEY.match(line)
            if m and m.group(2) in STAT_NAMES:
                if stat_indent is None:
                    stat_indent = len(m.group(1))
                if len(m.group(1)) == stat_indent:
                    current = m.group(2)
                    continue
            m = RE_STAT_VALUE.match(line)
            if m and current and m.group(2) == "base":
                bases[current] = int(m.group(3))
                continue
        if species_id is None:
            die("species file %s has no speciesid:" % path)
        out[species_id] = bases
    return out


def mob_files():
    found = []
    for dirpath, _dirnames, filenames in os.walk(MOB_ROOT):
        for name in sorted(filenames):
            if name.endswith(".yaml"):
                found.append(os.path.join(dirpath, name))
    return sorted(found)


class Rewrite(object):
    """One planned edit: replace a line, or drop it."""

    def __init__(self, index, text=None):
        self.index = index
        self.text = text  # None means delete the line


def plan_file(path, species_bases):
    """Return (rows, rewrites) for one mob file. rows feed the CSV report."""
    with open(path, encoding="utf-8") as fh:
        raw = fh.read()
    lines = raw.splitlines()

    species_id = None
    for line in lines:
        m = RE_SPECIESID.match(line)
        if m:
            species_id = int(m.group(1))
            break
    if species_id is None:
        die("%s has no speciesid: -- refusing to guess a species baseline" % path)
    if species_id not in species_bases:
        die("%s references speciesid %d, which has no species file" % (path, species_id))
    bases = species_bases[species_id]

    # Locate the stats block: from `  stats:` to the next line at or above its
    # own indentation.
    start = None
    stats_indent = None
    for i, line in enumerate(lines):
        m = RE_STATS_KEY.match(line)
        if m:
            if start is not None:
                die("%s has more than one stats: block" % path)
            start = i
            stats_indent = len(m.group(1))
    if start is None:
        return [], []

    end = len(lines)
    for i in range(start + 1, len(lines)):
        line = lines[i]
        if not line.strip():
            continue
        indent = len(line) - len(line.lstrip(" "))
        if indent <= stats_indent:
            end = i
            break

    # Group the block into per-stat entries.
    entries = []  # (stat, key_index, flow, base_idx, base_val, train_idx, train_val)
    stat_indent = None
    current = None
    for i in range(start + 1, end):
        line = lines[i]
        m = RE_STAT_FLOW.match(line)
        if m and m.group(2) in STAT_NAMES:
            if stat_indent is None:
                stat_indent = len(m.group(1))
            if len(m.group(1)) != stat_indent:
                die("%s:%d unexpected indentation inside stats:" % (path, i + 1))
            entry = {"stat": m.group(2), "key": i, "flow": m.group(3),
                     "base": None, "base_i": None, "train": None, "train_i": None}
            slot = SLOT[m.group(4)]
            entry[slot] = int(m.group(5))
            entry[slot + "_i"] = i
            entries.append(entry)
            current = None
            continue
        m = RE_STAT_KEY.match(line)
        if m and m.group(2) in STAT_NAMES:
            if stat_indent is None:
                stat_indent = len(m.group(1))
            if len(m.group(1)) != stat_indent:
                die("%s:%d unexpected indentation inside stats:" % (path, i + 1))
            current = {"stat": m.group(2), "key": i, "flow": None,
                       "base": None, "base_i": None, "train": None, "train_i": None}
            entries.append(current)
            continue
        m = RE_STAT_VALUE.match(line)
        if m:
            if current is None:
                die("%s:%d value line with no stat key above it" % (path, i + 1))
            slot = SLOT[m.group(2)]
            if current[slot] is not None:
                die("%s:%d duplicate %s: for %s" % (path, i + 1, m.group(2), current["stat"]))
            current[slot] = int(m.group(3))
            current[slot + "_i"] = i
            continue
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        die("%s:%d unrecognised line inside stats: %r" % (path, i + 1, line))

    rows = []
    rewrites = []
    live_keys = 0
    for e in entries:
        training = e["train"]
        if training is None:
            live_keys += 1
            continue

        species_base = bases.get(e["stat"], 0)
        old_base = e["base"]
        if training == 0:
            # Drop the value line. In flow form that line IS the stat key, so
            # nothing is left over. In block form, drop the now-childless key
            # too -- unless the stat also had an authored base, which stays.
            rewrites.append(Rewrite(e["train_i"], None))
            if e["flow"] is None:
                if old_base is None:
                    rewrites.append(Rewrite(e["key"], None))
                else:
                    live_keys += 1
            rows.append({"file": path.replace(os.sep, "/"), "stat": e["stat"],
                         "old_base": "" if old_base is None else old_base,
                         "old_training": 0, "species_base": species_base,
                         "new_base": "" if old_base is None else old_base})
            continue

        new_base = (old_base if old_base is not None else species_base) + training
        live_keys += 1
        if e["flow"] is not None:
            indent = " " * (len(lines[e["key"]]) - len(lines[e["key"]].lstrip(" ")))
            rewrites.append(Rewrite(e["key"],
                                    "%s%s:%s{base: %d}" % (indent, e["stat"], e["flow"], new_base)))
        elif old_base is not None:
            base_line = lines[e["base_i"]]
            indent = " " * (len(base_line) - len(base_line.lstrip(" ")))
            rewrites.append(Rewrite(e["base_i"], "%sbase: %d" % (indent, new_base)))
            rewrites.append(Rewrite(e["train_i"], None))
        else:
            train_line = lines[e["train_i"]]
            indent = " " * (len(train_line) - len(train_line.lstrip(" ")))
            rewrites.append(Rewrite(e["train_i"], "%sbase: %d" % (indent, new_base)))

        rows.append({"file": path.replace(os.sep, "/"), "stat": e["stat"],
                     "old_base": "" if old_base is None else old_base,
                     "old_training": training, "species_base": species_base,
                     "new_base": new_base})

    # An emptied stats: block is dropped as well, for the same reason.
    if entries and live_keys == 0:
        rewrites.append(Rewrite(start, None))

    return rows, rewrites


def apply_rewrites(path, rewrites):
    """Write path + '.new'. Never writes back to the path it read."""
    with open(path, encoding="utf-8", newline="") as fh:
        raw = fh.read()
    newline = "\r\n" if "\r\n" in raw else "\n"
    trailing = raw.endswith(("\n", "\r"))
    lines = raw.splitlines()

    drop = set()
    replace = {}
    for r in rewrites:
        if r.text is None:
            drop.add(r.index)
        else:
            replace[r.index] = r.text

    out = []
    for i, line in enumerate(lines):
        if i in drop:
            continue
        out.append(replace.get(i, line))

    body = newline.join(out) + (newline if trailing else "")
    with open(path + ".new", "w", encoding="utf-8", newline="") as fh:
        fh.write(body)


def cmd_fold(dry_run):
    species_bases = load_species_bases()
    writer = csv.DictWriter(sys.stdout,
                            fieldnames=["file", "stat", "old_base", "old_training",
                                        "species_base", "new_base"],
                            lineterminator="\n")
    writer.writeheader()

    files_touched = 0
    stats_touched = 0
    zeroes = []
    for path in mob_files():
        rows, rewrites = plan_file(path, species_bases)
        if not rewrites:
            continue
        files_touched += 1
        stats_touched += len(rows)
        for row in rows:
            writer.writerow(row)
            if row["new_base"] == 0 and int(row["old_training"]) != 0:
                zeroes.append((row["file"].split("mobs/")[-1], row["stat"],
                               row["species_base"], int(row["old_training"])))
        if not dry_run:
            apply_rewrites(path, rewrites)

    sys.stderr.write("%s: %d files, %d stat entries\n"
                     % ("would change" if dry_run else "wrote .new for",
                        files_touched, stats_touched))

    # Called out because it is the one arithmetic result that depends on the
    # BaseAuthored hydration fix landing alongside it.
    if zeroes:
        sys.stderr.write("stats folding to exactly `base: 0` (%d) -- these rely on\n"
                         "stats.StatInfo.BaseAuthored to avoid re-hydrating:\n" % len(zeroes))
        for name, stat, species_base, training in zeroes:
            sys.stderr.write("  %s %s (species base %s %+d)\n"
                             % (name, stat, species_base, training))


def cmd_verify(report_path):
    """Re-check the arithmetic against the report and the files on disk."""
    bad = 0
    with open(report_path, encoding="utf-8") as fh:
        rows = list(csv.DictReader(fh))
    if not rows:
        die("report %s has no rows" % report_path)

    for row in rows:
        old_base = row["old_base"]
        old_training = int(row["old_training"])
        recorded_species_base = int(row["species_base"])
        expected = (int(old_base) if old_base != "" else recorded_species_base) + old_training
        new_base = row["new_base"]
        if old_training == 0:
            if new_base != old_base:
                sys.stderr.write("arithmetic: %s %s expected the base untouched\n"
                                 % (row["file"], row["stat"]))
                bad += 1
            continue
        if int(new_base) != expected:
            sys.stderr.write("arithmetic: %s %s expected %d, report says %s\n"
                             % (row["file"], row["stat"], expected, new_base))
            bad += 1

    # And the files themselves: no training may remain anywhere.
    residue = 0
    for path in mob_files():
        with open(path, encoding="utf-8") as fh:
            for i, line in enumerate(fh.read().splitlines()):
                if re.search(r"\btraining:\s*-?\d+", line):
                    sys.stderr.write("residue: %s:%d %s\n" % (path, i + 1, line.strip()))
                    residue += 1

    # And each written base is what the report claims.
    for row in rows:
        if row["new_base"] == "":
            continue
        with open(row["file"], encoding="utf-8") as fh:
            body = fh.read()
        want_block = "%s:\n      base: %s\n" % (row["stat"], row["new_base"])
        want_flow = "%s:" % row["stat"]
        if want_block not in body and ("{base: %s}" % row["new_base"]) not in body:
            if want_flow not in body:
                sys.stderr.write("missing: %s %s base %s\n"
                                 % (row["file"], row["stat"], row["new_base"]))
                bad += 1

    if bad or residue:
        die("verify failed: %d arithmetic/absence problems, %d training residues" % (bad, residue))
    sys.stderr.write("verify OK: %d rows, no training: remains in %d mob files\n"
                     % (len(rows), len(mob_files())))


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--dry-run", action="store_true",
                    help="report only; write no .new files")
    ap.add_argument("--verify", metavar="REPORT",
                    help="re-check the arithmetic in REPORT against the files on disk")
    args = ap.parse_args()

    if not os.path.isdir(MOB_ROOT):
        die("run me from the repo root: %s not found" % MOB_ROOT)

    if args.verify:
        cmd_verify(args.verify)
    else:
        cmd_fold(args.dry_run)


if __name__ == "__main__":
    main()
