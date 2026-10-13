"""Read-only audit of combat-message role-pool equality.

PR 2 of messaging M3 item 8 renders every audience of one swing from a single
coordinated index, which requires the role pools in each (verb, split, tier)
group to be equal in length. This reports every group where they are not.

Both layouts are read: role-first (split -> role -> tier -> [lines]) and
records (split -> tier -> [{actor, actee, observer[, remote_observer]}]).
Records keep pools equal by construction; the audit still reports any short
pool, such as a record missing a role.

READ ONLY. Never write YAML from Python: yaml.dump would destroy the token
comment header, the quoting and the key order of every file it touched.

Usage:
    python tools/combat_message_pool_audit.py            # whole store
    python tools/combat_message_pool_audit.py slashing   # one subtype

Exit 0 when every group is equal, 1 when any gap remains.
"""
import glob
import os
import sys

import yaml

STORE = os.path.join("_datafiles", "world", "dogmud", "combat-messages")
TIERS = ["beginner", "expert", "master"]


def groups(path):
    """Yield (verb, split, tier, {role: count}) for one file."""
    with open(path, "r", encoding="utf-8") as fh:
        doc = yaml.safe_load(fh)
    for verb, verb_body in ((doc or {}).get("options") or {}).items():
        for split, split_body in (verb_body or {}).items():
            if not split_body:
                continue
            if any(k in TIERS for k in split_body):
                # Record layout: split -> tier -> [{role: line}]. Count each
                # role across the tier's records; a record missing a role
                # shows as a short pool.
                # Roles used anywhere in this split; a role no record uses
                # (remote_observer on together) is not a short pool.
                used = sorted({
                    r for t in TIERS for rec in (split_body.get(t) or [])
                    for r in (rec or {})
                })
                for tier in TIERS:
                    recs = split_body.get(tier) or []
                    if not recs:
                        continue
                    counts = {
                        r: sum(1 for rec in recs if (rec or {}).get(r))
                        for r in used
                    }
                    yield verb, split, tier, counts
                continue
            # An explicitly-null role is a present role with an empty pool,
            # not an absent one: shooting.yaml nulls todefenderroom.
            roles = {
                role: (body if isinstance(body, dict) else {})
                for role, body in split_body.items()
            }
            for tier in TIERS:
                counts = {r: len(b.get(tier) or []) for r, b in roles.items()}
                if any(counts.values()):
                    yield verb, split, tier, counts


def main():
    only = sys.argv[1] if len(sys.argv) > 1 else None
    pattern = f"{only}.yaml" if only else "*.yaml"
    paths = sorted(glob.glob(os.path.join(STORE, pattern)))
    if not paths:
        print(f"no files matched {pattern} under {STORE}")
        return 1

    total_lines = 0
    total_groups = 0
    for path in paths:
        name = os.path.basename(path)
        rows = []
        for verb, split, tier, counts in groups(path):
            widest = max(counts.values())
            need = sum(widest - n for n in counts.values())
            if need:
                detail = ", ".join(
                    f"{r}={n}" + ("" if n == widest else f" (+{widest - n})")
                    for r, n in sorted(counts.items())
                )
                rows.append(f"    {verb}/{split}/{tier}: {detail}")
                total_lines += need
                total_groups += 1
        if rows:
            print(f"{name}")
            for row in rows:
                print(row)

    if total_lines:
        print(f"\nGAPS: {total_lines} lines across {total_groups} groups")
        return 1
    print(f"OK: every role pool is equal per tier ({len(paths)} file(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
