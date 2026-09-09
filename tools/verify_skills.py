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

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
SKILLS = PROJECT_ROOT / ".claude" / "skills"
EXPECTED = [
    "dogmud-shipping", "dogmud-deploying", "dogmud-authoring-content",
    "dogmud-authoring-quests", "dogmud-player-copy", "dogmud-writing-tests",
    "dogmud-playtesting", "dogmud-persistence", "dogmud-refactoring",
    "dogmud-combat", "dogmud-progression-model", "dogmud-balance-config",
]


def parse_frontmatter(text):
    """Return (dict, None) for a closed --- block, or (None, reason) on failure."""
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return None, "no opening --- line"
    out = {}
    for line in lines[1:]:
        if line.strip() == "---":
            return out, None
        if line.startswith(("  ", "\t")) or ":" not in line:
            continue
        key, _, val = line.partition(":")
        out[key.strip()] = val.strip().strip('"').strip("'")
    return None, "frontmatter block is never closed"


def main():
    errors = []
    if not SKILLS.is_dir():
        print("FAIL: .claude/skills does not exist")
        return 1

    found = sorted(p.name for p in SKILLS.iterdir() if p.is_dir())
    valid = set()
    for name in found:
        path = SKILLS / name / "SKILL.md"
        if not path.is_file():
            errors.append(f"{name}: no SKILL.md")
            continue
        try:
            text = path.read_text(encoding="utf-8-sig")
        except (UnicodeDecodeError, OSError) as exc:
            errors.append(f"{name}: SKILL.md unreadable ({exc.__class__.__name__})")
            continue
        fm, reason = parse_frontmatter(text)
        if fm is None:
            errors.append(f"{name}: {reason}")
            continue
        ok = True
        if fm.get("name") != name:
            errors.append(f"{name}: frontmatter name is {fm.get('name')!r}")
            ok = False
        desc = fm.get("description", "")
        if not desc:
            errors.append(f"{name}: empty description")
            ok = False
        elif len(desc) > 500:
            errors.append(f"{name}: description is {len(desc)} chars, over 500")
            ok = False
        if ok:
            valid.add(name)

    present = [n for n in EXPECTED if n in valid]
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
