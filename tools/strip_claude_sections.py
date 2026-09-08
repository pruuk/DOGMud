#!/usr/bin/env python3
"""Remove named ## sections from CLAUDE.md.

Sections are identified by their EXACT heading line, never by line number,
because deleting one section renumbers every line below it. All 44 headings in
CLAUDE.md are unique (verified 2026-09-08) and four contain non-ASCII
characters, so the manifest must carry exact bytes.

If ANY manifest heading is not found, the script aborts and writes nothing.
A typo that silently removes nothing, or removes the wrong span, is the
failure mode this guards against.

Usage:
    python tools/strip_claude_sections.py --manifest tools/claude_strip_manifest.txt
    python tools/strip_claude_sections.py --manifest tools/claude_strip_manifest.txt --apply
"""
import argparse
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent


def read_manifest(path):
    """Return the list of heading lines to remove, ignoring blanks and #-comments."""
    out = []
    for raw in path.read_text(encoding="utf-8-sig").splitlines():
        line = raw.rstrip()
        if not line.strip() or line.lstrip().startswith("# "):
            continue
        out.append(line)
    return out


def find_sections(lines, headings):
    """Map each heading to (start_index, end_index_exclusive). Missing -> None."""
    spans = {}
    for h in headings:
        try:
            start = lines.index(h)
        except ValueError:
            spans[h] = None
            continue
        end = len(lines)
        for i in range(start + 1, len(lines)):
            if lines[i].startswith("## "):
                end = i
                break
        spans[h] = (start, end)
    return spans


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--target", default=str(PROJECT_ROOT / "CLAUDE.md"))
    ap.add_argument("--apply", action="store_true",
                    help="write the file; without this it is a dry run")
    args = ap.parse_args()

    target = Path(args.target)
    manifest = Path(args.manifest)
    if not target.is_file():
        print(f"FAIL: no such target {target}")
        return 1
    if not manifest.is_file():
        print(f"FAIL: no such manifest {manifest}")
        return 1

    lines = target.read_text(encoding="utf-8-sig").splitlines()
    headings = read_manifest(manifest)
    spans = find_sections(lines, headings)

    missing = [h for h, s in spans.items() if s is None]
    if missing:
        print(f"FAIL: {len(missing)} manifest heading(s) not found in {target.name}:")
        for h in missing:
            print(f"  {h!r}")
        print("Nothing was written. Fix the manifest; headings must match exactly,")
        print("including non-ASCII characters.")
        return 1

    doomed = set()
    total = 0
    for h in headings:
        start, end = spans[h]
        n = end - start
        total += n
        print(f"  {n:4d} lines  {h}")
        doomed.update(range(start, end))

    print(f"{len(headings)} sections, {total} lines, "
          f"{len(lines)} -> {len(lines) - total} lines")

    if not args.apply:
        print("DRY RUN, nothing written. Re-run with --apply to write.")
        return 0

    kept = [l for i, l in enumerate(lines) if i not in doomed]
    target.write_text("\n".join(kept) + "\n", encoding="utf-8")
    print(f"WROTE {target}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
