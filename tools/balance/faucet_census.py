"""Mechanical census of every progression faucet call site.

Reads Go sources only; writes a fresh report. For each call to a progression
entry point it records: file:line, the enclosing func, the literal argument,
every `if` guard between the func opening and the call (the GATE), and any
TryCooldown/CooldownReady key seen in the same function (the THROTTLE).

This is deliberately mechanical. Two rounds of blind review failed because
uses/hour were reasoned about instead of extracted.
"""
import io
import os
import re
import json

ROOTS = ["internal", "modules"]
ENTRIES = [
    "OnStatUse", "OnSkillUse", "OnSkillUseScaled",
    "CheckStatProgression", "CheckSkillProgression", "CheckRegenProgression",
]
CALL_RE = re.compile(r"\.(" + "|".join(ENTRIES) + r")\(")
FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z0-9_]+)")
IF_RE = re.compile(r"^\s*(if|for|switch|case)\b(.*)")
# NOTE: cooldown keys appear in BOTH quote styles. `assess` and `special-move`
# are backtick-quoted; a double-quote-only pattern reports them as unthrottled
# and that error produced a wrong faucet map on the first pass.
CD_RE = re.compile(r"(TryCooldown|CooldownReady|GetCooldown)\(\s*"
                   r"(\"[^\"]+\"|`[^`]+`|[A-Za-z0-9_.]+)")

rows = []
for root in ROOTS:
    for dirpath, _, files in os.walk(root):
        for fn in files:
            if not fn.endswith(".go") or fn.endswith("_test.go"):
                continue
            path = os.path.join(dirpath, fn).replace("\\", "/")
            try:
                lines = io.open(path, encoding="utf-8").read().split("\n")
            except Exception:
                continue
            # index of enclosing func for each line
            func_at = [None] * len(lines)
            cur = None
            for i, ln in enumerate(lines):
                m = FUNC_RE.match(ln)
                if m:
                    cur = m.group(1)
                func_at[i] = cur
            for i, ln in enumerate(lines):
                if not CALL_RE.search(ln):
                    continue
                # argument: text inside the first (...) after the entry name
                m = CALL_RE.search(ln)
                entry = m.group(1)
                tail = ln[m.end():]
                depth, arg = 1, ""
                for ch in tail:
                    if ch == "(":
                        depth += 1
                    elif ch == ")":
                        depth -= 1
                        if depth == 0:
                            break
                    arg += ch
                if not arg.strip():
                    # arg continues on following lines
                    arg = " ".join(x.strip() for x in lines[i + 1:i + 3])
                # walk back to the func opening, collecting guards
                start = i
                while start > 0 and not FUNC_RE.match(lines[start]):
                    start -= 1
                guards, cooldowns = [], []
                for j in range(start, i):
                    g = IF_RE.match(lines[j])
                    if g:
                        txt = (g.group(1) + g.group(2)).strip().rstrip("{").strip()
                        if len(txt) < 160:
                            guards.append(txt)
                    c = CD_RE.search(lines[j])
                    if c:
                        cooldowns.append(c.group(2))
                rows.append({
                    "file": path,
                    "line": i + 1,
                    "func": func_at[i],
                    "entry": entry,
                    "arg": arg.strip()[:90],
                    "cooldowns": sorted(set(cooldowns)),
                    "guards": guards[-6:],
                })

rows.sort(key=lambda r: (r["file"], r["line"]))
out = io.open(os.environ["CENSUS_OUT"], "w", encoding="utf-8", newline="\n")
out.write("# Progression faucet census (mechanical extraction)\n\n")
out.write("Total call sites: %d\n\n" % len(rows))
for r in rows:
    out.write("## %s:%d  %s\n" % (r["file"], r["line"], r["entry"]))
    out.write("- func: `%s`\n" % r["func"])
    out.write("- arg: `%s`\n" % r["arg"])
    out.write("- cooldown keys in func: %s\n" %
              (", ".join("`%s`" % c for c in r["cooldowns"]) if r["cooldowns"] else "**NONE**"))
    if r["guards"]:
        out.write("- guards:\n")
        for g in r["guards"]:
            out.write("    - `%s`\n" % g)
    else:
        out.write("- guards: **NONE**\n")
    out.write("\n")
out.close()
print("wrote %d call sites" % len(rows))

# machine-readable too
io.open(os.environ["CENSUS_OUT"].replace(".md", ".json"), "w", encoding="utf-8", newline="\n").write(
    json.dumps(rows, indent=1))
