#!/usr/bin/env python3
"""Verify every .claude/skills/<name>/SKILL.md is well-formed.

Checks, per skill:
  - the directory contains a SKILL.md
  - SKILL.md opens with a --- frontmatter block
  - frontmatter has both `name` and `description`
  - frontmatter `name` matches the directory name
  - `description` is non-empty and under 500 chars

Exits 1 and prints every failure if any check fails.
"""
import sys
from pathlib import Path

SKILLS = Path(".claude/skills")
EXPECTED = [
    "dogmud-shipping", "dogmud-deploying", "dogmud-authoring-content",
    "dogmud-authoring-quests", "dogmud-player-copy", "dogmud-writing-tests",
    "dogmud-playtesting", "dogmud-persistence", "dogmud-refactoring",
    "dogmud-combat", "dogmud-progression-model", "dogmud-balance-config",
]


def parse_frontmatter(text):
    """Return dict of top-level scalar keys in the leading --- block, or None."""
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return None
    out = {}
    for line in lines[1:]:
        if line.strip() == "---":
            return out
        if line.startswith(("  ", "\t")) or ":" not in line:
            continue
        key, _, val = line.partition(":")
        out[key.strip()] = val.strip().strip('"').strip("'")
    return None


def main():
    errors = []
    if not SKILLS.is_dir():
        print("FAIL: .claude/skills does not exist")
        return 1

    found = sorted(p.name for p in SKILLS.iterdir() if p.is_dir())
    for name in found:
        path = SKILLS / name / "SKILL.md"
        if not path.is_file():
            errors.append(f"{name}: no SKILL.md")
            continue
        fm = parse_frontmatter(path.read_text(encoding="utf-8"))
        if fm is None:
            errors.append(f"{name}: no closed --- frontmatter block")
            continue
        if fm.get("name") != name:
            errors.append(f"{name}: frontmatter name is {fm.get('name')!r}")
        desc = fm.get("description", "")
        if not desc:
            errors.append(f"{name}: empty description")
        elif len(desc) > 500:
            errors.append(f"{name}: description is {len(desc)} chars, over 500")

    present = [n for n in EXPECTED if n in found]
    missing = [n for n in EXPECTED if n not in found]
    for name in missing:
        errors.append(f"{name}: expected skill not present")

    unexpected = [n for n in found if n not in EXPECTED]
    for name in unexpected:
        errors.append(f"{name}: unexpected directory, not one of the twelve")

    for e in errors:
        print("FAIL:", e)
    print(f"{len(present)}/{len(EXPECTED)} skills present, {len(errors)} errors")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
