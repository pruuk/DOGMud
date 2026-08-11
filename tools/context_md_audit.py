#!/usr/bin/env python3
"""Audit package context.md files for symbols the package does not define.

CLAUDE.md requires every package under internal/ and modules/ to carry a
context.md, and requires it to be updated when the package's API changes. That
second half drifts silently: a context.md describing an invented API is worse
than no file at all, because an agent will code against it and the mistake only
surfaces at compile time or, worse, at runtime.

Coverage is easy to check and is currently 100%. ACCURACY is what rots. This
script finds the rot.

Usage:
    python tools/context_md_audit.py                 # whole repo
    python tools/context_md_audit.py internal/mobs   # one package
    python tools/context_md_audit.py --verbose       # list every symbol

What it checks
--------------
Only `func Name(`, `func (r *T) Name(` and `type Name` lines that appear inside
```go fenced blocks, and only reports a symbol when it is absent from EVERY .go
file in that package (tests included, since a documented test helper is still a
real symbol).

Known limits -- triage the output, do not treat it as a defect list:

  * Fenced blocks under an "Example"/"Usage" heading are skipped, but only when
    the heading is a real markdown heading. A bolded pseudo-heading
    (**Example: ...**) is also skipped, but other phrasings may slip through
    and show up as false positives.
  * A symbol legitimately owned by another package but shown in a go block for
    context will be reported. That is usually a sign the block wants a comment
    saying where it lives.
"""
import os
import re
import sys

FUNC = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(")
TYPE = re.compile(r"^type\s+([A-Za-z_]\w*)\b")
# Headings that mark a block as illustrative rather than a claimed API.
ILLUSTRATIVE = re.compile(r"example|usage|how to|pattern|consumer|writing",
                          re.IGNORECASE)
SKIP_DIRS = {".git", "vendor", "node_modules", "_datafiles", "docs", "bin"}


def find_packages(root):
    """Yield (dir, filenames) for every dir holding .go files AND a context.md."""
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        if not any(f.endswith(".go") for f in filenames):
            continue
        if "context.md" not in filenames:
            continue
        yield dirpath, filenames


def documented_symbols(md_text):
    """Extract (kind, name) from go blocks that are NOT illustrative."""
    out = []
    heading = ""
    in_block = False
    keep = False
    for line in md_text.splitlines():
        stripped = line.strip()
        # Track both real headings and bolded pseudo-headings.
        if stripped.startswith("#") or (
                stripped.startswith("**") and stripped.endswith("**")):
            heading = stripped
        if stripped.startswith("```go"):
            in_block, keep = True, not ILLUSTRATIVE.search(heading)
            continue
        if stripped.startswith("```") and in_block:
            in_block = False
            continue
        if not (in_block and keep):
            continue
        m = FUNC.match(stripped)
        if m:
            out.append(("func", m.group(1)))
            continue
        m = TYPE.match(stripped)
        if m:
            out.append(("type", m.group(1)))
    return out


def package_source(pkg, filenames):
    src = []
    for f in filenames:
        if not f.endswith(".go"):
            continue
        try:
            with open(os.path.join(pkg, f), encoding="utf-8",
                      errors="ignore") as fh:
                src.append(fh.read())
        except OSError:
            pass
    return "\n".join(src)


def declares(src, kind, name):
    if kind == "func":
        pat = re.compile(r"^func\s+(?:\([^)]*\)\s*)?" + re.escape(name) +
                         r"\s*\(", re.M)
    else:
        pat = re.compile(r"^type\s+" + re.escape(name) + r"\b", re.M)
    return bool(pat.search(src))


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    verbose = "--verbose" in sys.argv or "-v" in sys.argv
    root = args[0] if args else "."

    report = []
    checked = 0
    for pkg, filenames in find_packages(root):
        checked += 1
        src = package_source(pkg, filenames)
        with open(os.path.join(pkg, "context.md"), encoding="utf-8",
                  errors="ignore") as fh:
            documented = documented_symbols(fh.read())

        missing = [f"{k} {n}" for k, n in documented if not declares(src, k, n)]
        if missing:
            report.append((pkg, len(documented), missing))

    report.sort(key=lambda r: -len(r[2]))
    total = sum(len(r[2]) for r in report)

    print(f"packages checked:                {checked}")
    print(f"packages with phantom symbols:   {len(report)}")
    print(f"total phantom symbols:           {total}")
    if not report:
        print("\nAll documented symbols resolve.")
        return 0
    print()
    for pkg, n_doc, missing in report:
        rel = pkg.replace("\\", "/").lstrip("./")
        print(f"{rel}   ({len(missing)} phantom of {n_doc} documented)")
        shown = missing if verbose else missing[:8]
        for m in shown:
            print(f"     {m}")
        if len(missing) > len(shown):
            print(f"     ... +{len(missing) - len(shown)} more (--verbose)")
    print("\nTriage these -- see 'Known limits' in this script's docstring.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
