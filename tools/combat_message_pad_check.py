"""Check a combat-message pad against a baseline git ref.

M3 item 8 PR 1 both APPENDS new lines and REORDERS existing ones, so that
index N describes the same moment in every role. Reordering means the golden
diff can no longer be "additions only", and a plain diff of the YAML cannot
tell a moved line from an edited one.

This compares, per (file, verb, split, role, tier) group, the multiset of
authored lines against a baseline ref. The contract it enforces:

  * no line is DELETED  (every baseline line still exists in its own group)
  * no line is EDITED   (an edit shows up as one deletion plus one addition)
  * no line MIGRATES    (groups are compared independently)
  * lines may be REORDERED freely within their group
  * lines may be ADDED

Authored files are in the record layout; the retired role-first reader stays
so the diff can read old commits:

  Records:    split -> tier -> [{actor, actee, observer[, remote_observer]}]
  Role-first (historical revisions only): split -> role -> tier -> [lines]

Flags:

  --ignore-tier      collapse groups to (verb, split, role), concatenating
                     tiers. A swap of lines between tiers, or a line moved
                     from one tier to another, keeps every multiset intact
                     and so passes. Use it for tier-bleed fixes.
  --manifest <json>  {"batch": N, "proposals": [{"file", "intensity",
                     "split", "tier", "role", "kind", "old", "new", ...}]}.
                     Each proposal with kind "rewrite" is a sanctioned edit:
                     one `old` is removed from the baseline multiset and one
                     `new` from the after multiset before the LOST check.
                     An `old` absent from the baseline, or a `new` absent
                     from the after file, is itself a violation (MANIFEST).
  --store <dir>      combat-message directory to check (default: the repo
                     store). Lets probes target a scratch copy.
  --baseline-dir <d> read baseline files from this directory instead of
                     `git show <ref>:<path>`. The ref is then only a label.

READ ONLY. Never write YAML from Python: yaml.dump would destroy the token
comment header, the quoting and the key order of every file it touched.

Usage:
    python tools/combat_message_pad_check.py <baseline-ref> [subtype]
        [--ignore-tier] [--manifest <file.json>] [--store <dir>]
        [--baseline-dir <dir>]

Exit 0 when the contract holds, 1 on any deletion, edit or bad manifest entry.
"""
import argparse
import collections
import glob
import json
import os
import subprocess
import sys

import yaml

STORE = os.path.join("_datafiles", "world", "dogmud", "combat-messages")
TIERS = ["beginner", "expert", "master"]
ROLES = ["actor", "actee", "observer", "remote_observer"]


def pools(doc):
    """Return {(verb, split, role, tier): [lines]} for either layout.

    Role-first: split -> role -> tier -> [lines].
    Records:    split -> tier -> [{role: line}], one record per blow.
    """
    out = {}
    for verb, verb_body in ((doc or {}).get("options") or {}).items():
        for split, split_body in (verb_body or {}).items():
            if not split_body:
                continue
            if any(k in TIERS for k in split_body):
                for tier in TIERS:
                    for rec in split_body.get(tier) or []:
                        for role, line in (rec or {}).items():
                            out.setdefault((verb, split, role, tier), []).append(line)
                continue
            for role, role_body in split_body.items():
                if not isinstance(role_body, dict):
                    continue
                for tier in TIERS:
                    lines = role_body.get(tier) or []
                    if lines:
                        out[(verb, split, role, tier)] = list(lines)
    return out


def collapse_tiers(p):
    """Merge {(verb, split, role, tier): lines} to {(verb, split, role): lines}."""
    out = {}
    for tier in TIERS:
        for key in sorted(p):
            if key[3] == tier:
                out.setdefault(key[:3], []).extend(p[key])
    return out


def baseline_doc(ref, path, baseline_dir):
    if baseline_dir:
        bpath = os.path.join(baseline_dir, os.path.basename(path))
        if not os.path.exists(bpath):
            return None
        with open(bpath, "r", encoding="utf-8") as fh:
            return yaml.safe_load(fh)
    rel = path.replace(os.sep, "/")
    try:
        blob = subprocess.run(
            ["git", "show", f"{ref}:{rel}"],
            capture_output=True, check=True, text=True, encoding="utf-8",
        ).stdout
    except subprocess.CalledProcessError:
        return None
    return yaml.safe_load(blob)


def manifest_rewrites(path):
    """Return {file_stem: [proposal, ...]} for kind == "rewrite" entries."""
    with open(path, "r", encoding="utf-8") as fh:
        data = json.load(fh)
    out = collections.defaultdict(list)
    for prop in data.get("proposals") or []:
        if prop.get("kind") == "rewrite":
            out[prop["file"]].append(prop)
    return out


def prop_key(prop, ignore_tier):
    key = (prop["intensity"], prop["split"], prop["role"])
    return key if ignore_tier else key + (prop["tier"],)


def main():
    ap = argparse.ArgumentParser(description="combat-message pad contract check")
    ap.add_argument("ref")
    ap.add_argument("subtype", nargs="?")
    ap.add_argument("--ignore-tier", action="store_true")
    ap.add_argument("--manifest")
    ap.add_argument("--store", default=STORE)
    ap.add_argument("--baseline-dir")
    args = ap.parse_args()

    ref = args.ref
    pattern = f"{args.subtype}.yaml" if args.subtype else "*.yaml"
    paths = sorted(glob.glob(os.path.join(args.store, pattern)))
    if not paths:
        print(f"no files matched {pattern} under {args.store}")
        return 1
    rewrites = manifest_rewrites(args.manifest) if args.manifest else {}

    bad = 0
    added_total = 0
    moved_total = 0
    for path in paths:
        name = os.path.basename(path)
        stem = os.path.splitext(name)[0]
        before = baseline_doc(ref, path, args.baseline_dir)
        if before is None:
            print(f"{name}: not present at {ref}, skipping")
            continue
        with open(path, "r", encoding="utf-8") as fh:
            after = yaml.safe_load(fh)

        b_pools = pools(before)
        a_pools = pools(after)
        if args.ignore_tier:
            b_pools = collapse_tiers(b_pools)
            a_pools = collapse_tiers(a_pools)

        b_counts = {k: collections.Counter(v) for k, v in b_pools.items()}
        a_counts = {k: collections.Counter(v) for k, v in a_pools.items()}
        rows = []
        added = 0
        moved = 0

        # Rewrites apply in manifest order, so a later proposal may rewrite
        # an earlier one's `new` (an addendum chaining X -> Y -> Z). Each
        # `old` is checked against the baseline as rewritten so far, and a
        # `new` must reach the after file unless a later proposal consumes it.
        props = rewrites.get(stem, [])
        for i, prop in enumerate(props):
            key = prop_key(prop, args.ignore_tier)
            old, new = prop["old"], prop["new"]
            label = f"{name} {'/'.join(key)}"
            if b_counts.get(key, collections.Counter())[old] < 1:
                rows.append(f"    MANIFEST {label}: old not in baseline: {old[:90]}")
                continue
            consumed = any(
                prop_key(later, args.ignore_tier) == key and later["old"] == new
                for later in props[i + 1:]
            )
            if not consumed and a_counts.get(key, collections.Counter())[new] < 1:
                rows.append(f"    MANIFEST {label}: new not in after file: {new[:90]}")
                continue
            b_counts[key][old] -= 1
            b_counts[key][new] += 1
            # Let the position check see the sanctioned edit as the
            # same line, so it does not read as a reorder.
            b_lines = list(b_pools[key])
            b_lines[b_lines.index(old)] = new
            b_pools[key] = b_lines

        for key in sorted(set(b_counts) | set(a_counts)):
            b_count = +b_counts.get(key, collections.Counter())
            a_count = +a_counts.get(key, collections.Counter())

            lost = b_count - a_count
            gained = a_count - b_count
            for text, n in lost.items():
                rows.append(f"    LOST  {'/'.join(key)} x{n}: {text[:90]}")
            added += sum(gained.values())
            # A line that stayed but changed position.
            if not lost:
                b_lines = b_pools.get(key, [])
                a_lines = a_pools.get(key, [])
                pos = {x: i for i, x in enumerate(a_lines)}
                common = [x for x in b_lines if x in pos]
                for i, x in enumerate(common):
                    if pos.get(x) != i:
                        moved += 1
                        break

        if rows:
            bad += 1
            print(f"{name}")
            for r in rows:
                print(r)
        added_total += added
        moved_total += moved

    print()
    if bad:
        print(f"CONTRACT VIOLATED in {bad} file(s): a line was deleted or edited.")
        print("A reorder never loses a line. Restore the text, then move it.")
        return 1
    print(f"OK against {ref}: nothing deleted or edited. "
          f"{added_total} line(s) added, {moved_total} group(s) reordered.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
