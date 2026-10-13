"""Check a combat-message file equals its reviewed after-state (read only).

Usage: python tools/combat_message_proposals_check.py <base-ref> <file> <manifest.json>

Loads <base-ref>:_datafiles/world/dogmud/combat-messages/<file>.yaml (record
layout: split -> tier -> list of records), applies the manifest's proposals for
that file in list order, in memory, and compares the result to the working
tree file slot by slot. Prints a DIFF line group per differing slot and exits
1 on any difference or apply error; exits 0 when identical. Never writes.

Manifest: {"batch": N, "proposals": [{"file", "kind", ...}]} where kind is
rewrite, swap, set_role_order or move (see apply()).
"""
import copy
import json
import os
import subprocess
import sys

import yaml

TIERS = ["beginner", "expert", "master"]
STORE = "_datafiles/world/dogmud/combat-messages"


def apply(doc, proposals):
    """Return (after_doc, errors). Pure; never touches disk."""
    after = copy.deepcopy(doc)
    errors = []
    opts = after["options"]
    for p in proposals:
        k = p["kind"]
        it, sp = p["intensity"], p["split"]
        try:
            block = opts[it][sp]
            if k == "rewrite":
                rec = block[p["tier"]][p["index"]]
                if rec[p["role"]] != p["old"]:
                    errors.append(f"rewrite old mismatch at {it}/{sp}/{p['tier']}[{p['index']}] {p['role']}")
                rec[p["role"]] = p["new"]
            elif k == "swap":
                a = block[p["tier"]][p["index"]]
                b = block[p["with_tier"]][p["with_index"]]
                r = p["role"]
                a[r], b[r] = b[r], a[r]
            elif k == "set_role_order":
                r = p["role"]
                before = {(t, i): rec[r] for t in TIERS for i, rec in enumerate(block.get(t) or [])}
                used = set()
                for o in p["order"]:
                    src = (o["from_tier"], o["from_index"])
                    if src in used:
                        errors.append(f"set_role_order reuses {src} in {it}/{sp} {r}")
                    used.add(src)
                    block[o["tier"]][o["index"]][r] = before[src]
            elif k == "move":
                rec = block[p["tier"]][p["index"]]
                block[p["tier"]][p["index"]] = None
                block.setdefault(p["to_tier"], []).append(rec)
            else:
                errors.append(f"unknown kind {k}")
        except (KeyError, IndexError, TypeError) as e:
            errors.append(f"{k} at {it}/{sp}: {e!r}")
    # Moves leave a None placeholder so indexes stay stable during the pass.
    for ib in opts.values():
        for sb in (ib or {}).values():
            for t in TIERS:
                if sb and sb.get(t):
                    sb[t] = [r for r in sb[t] if r is not None]
    return after, errors


def main():
    if len(sys.argv) != 4:
        print(__doc__)
        return 2
    ref, name, manifest = sys.argv[1:4]
    repo = os.getcwd()
    rel = f"{STORE}/{name}.yaml"
    shown = subprocess.run(
        ["git", "-C", repo, "show", f"{ref}:{rel}"],
        capture_output=True, check=True, text=True, encoding="utf-8",
    ).stdout
    base = yaml.safe_load(shown)
    with open(manifest, encoding="utf-8") as fh:
        props = [p for p in json.load(fh)["proposals"] if p["file"] == name]
    want, errors = apply(base, props)
    with open(os.path.join(repo, rel), encoding="utf-8") as fh:
        got = yaml.safe_load(fh)
    if errors:
        for e in errors:
            print("apply error:", e)
        return 1
    bad = 0
    for it in sorted(set(want["options"]) | set(got["options"])):
        w, g = want["options"].get(it), got["options"].get(it)
        if w == g:
            continue
        bad += 1
        for sp in sorted(set(w or {}) | set(g or {})):
            for t in TIERS:
                wl = ((w or {}).get(sp) or {}).get(t) or []
                gl = ((g or {}).get(sp) or {}).get(t) or []
                for i in range(max(len(wl), len(gl))):
                    a = wl[i] if i < len(wl) else None
                    b = gl[i] if i < len(gl) else None
                    if a != b:
                        print(f"DIFF {it}/{sp}/{t}[{i}]\n  want {a}\n  got  {b}")
    print(f"{name}: {len(props)} proposals, " + ("OK identical" if not bad else f"{bad} intensities differ"))
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
