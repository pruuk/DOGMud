#!/usr/bin/env python3
"""List every CategorySystem send with a proposed class (messaging M7, #371).

Spec section 6: a read-only classifier that reads Go source and never writes
it. It PROPOSES; a human or agent rules. The slice 3 ledger
(docs/superpowers/audits/messaging-m7-refusal-ledger.md) is its output ruled
row by row. Classes:

  usage      the text holds "Usage:" or opens "Syntax:"
  prompt     a bare question back ("X what?")
  refusal    opens with a refusal phrase ("You can't", "There is no", "Only")
  narration  names a third party seeing something ("enters", "notices you")
  refusal?   holds a refusal word mid-line ("can't", "already", "failed")
  other      reports, tables, confirmations
  dynamic    no literal reaches the send; the expression column shows what does

Usage:
    python tools/refusal_classify.py                    # summary by file
    python tools/refusal_classify.py --tsv              # one row per send
    python tools/refusal_classify.py internal/actions   # one tree

Admin files (internal/usercommands/admin.*.go) and internal/refusal are
skipped: slice 4 retags admin output by file.
"""
import argparse
import os
import re
import sys
from collections import Counter, defaultdict

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

LIT = re.compile(r'`([^`]*)`|"((?:[^"\\]|\\.)*)"')
TAG = re.compile(r'<[^>]+>')

REFUSAL_OPEN = re.compile(
    r"^(?:You (?:can'?t|cannot|can not|don'?t|do not|aren'?t|are not|have no|"
    r"haven'?t|need|must|lack|'?re (?:not|too|already)|are (?:too|already)|"
    r"already|have nothing|may not|won'?t|'?d need|fail)|"
    r"There (?:is|are|'s) no|There isn'?t|Nothing |No such|No one|Nobody|No |None |"
    r"(?:That|This|It|They) (?:is|isn'?t|doesn'?t|can'?t|won'?t|aren'?t|are not|has no)|"
    r"Only |Not |Invalid|Unknown|Can'?t|Cannot|Sorry|"
    r"%s (?:is|isn'?t|can'?t|doesn'?t|won'?t|has no|does not|are not))",
    re.I)
REFUSAL_WORD = re.compile(
    r"\b(can'?t|cannot|isn'?t|aren'?t|doesn'?t|don'?t|not enough|too far|"
    r"already|must be|need to|unable|vanished|missing|failed)\b", re.I)
USAGE = re.compile(r"Usage:|^\s*Syntax:", re.I)
PROMPT = re.compile(r"^\S+( (on|from|to|at|with))? (what|whom|who|which|how much)\?", re.I)
NARRATION = re.compile(r"\b(enters|leaves|notices you|arrives|the room)\b", re.I)


def call_span(text, start):
    """Text from start to the paren that closes the call holding start."""
    depth, i, n = 0, start, len(text)
    while i < n:
        c = text[i]
        if c in '`"':
            q = c
            i += 1
            while i < n and text[i] != q:
                if q == '"' and text[i] == '\\':
                    i += 1
                i += 1
        elif c == '(':
            depth += 1
        elif c == ')':
            if depth == 0:
                return text[start:i]
            depth -= 1
        elif c == '\n' and depth == 0 and text[i - 1] not in ',(':
            return text[start:i]
        i += 1
    return text[start:]


def classify(lit):
    if not lit:
        return 'dynamic'
    plain = TAG.sub('', lit).strip()
    if USAGE.search(plain):
        return 'usage'
    if PROMPT.search(plain):
        return 'prompt'
    if REFUSAL_OPEN.search(plain):
        return 'refusal'
    if NARRATION.search(plain):
        return 'narration'
    if REFUSAL_WORD.search(plain):
        return 'refusal?'
    return 'other'


def scan(roots):
    rows = []
    for root in roots:
        for dp, _, fns in os.walk(os.path.join(REPO, root)):
            for fn in sorted(fns):
                if not fn.endswith('.go') or fn.endswith('_test.go'):
                    continue
                path = os.path.join(dp, fn)
                rel = os.path.relpath(path, REPO).replace('\\', '/')
                if rel.startswith('internal/usercommands/admin.') or rel.startswith('internal/refusal/'):
                    continue
                with open(path, encoding='utf-8') as f:
                    text = f.read()
                for m in re.finditer(r'messaging\.CategorySystem', text):
                    line_start = text.rfind('\n', 0, m.start()) + 1
                    if text[line_start:m.start()].lstrip().startswith('//'):
                        continue
                    lineno = text.count('\n', 0, m.start()) + 1
                    span = call_span(text, m.end())
                    lits = [a or b for a, b in LIT.findall(span)]
                    lits = [l for l in lits if TAG.sub('', l).strip()]
                    lit = max(lits, key=lambda l: len(TAG.sub('', l))) if lits else ''
                    expr = ' '.join(span.lstrip(', ').split())[:100]
                    rows.append((rel, lineno, classify(lit), lit, expr))
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('roots', nargs='*', default=['internal', 'modules'])
    ap.add_argument('--tsv', action='store_true')
    a = ap.parse_args()
    sys.stdout.reconfigure(encoding='utf-8')
    rows = scan(a.roots)
    if a.tsv:
        clean = lambda s: s.replace('\t', ' ').replace('\n', '\\n')
        for rel, ln, cls, lit, expr in rows:
            print(f'{rel}\t{ln}\t{cls}\t{clean(lit)}\t{clean(expr)}')
        return 0
    print(f'{len(rows)} CategorySystem sends')
    for cls, n in Counter(r[2] for r in rows).most_common():
        print(f'  {cls:10s} {n}')
    byfile = defaultdict(Counter)
    for r in rows:
        byfile[r[0]][r[2]] += 1
    print()
    for fn, c in sorted(byfile.items(), key=lambda kv: (-sum(kv[1].values()), kv[0])):
        print(f'{sum(c.values()):4d}  {fn}  ' + ' '.join(f'{k}={v}' for k, v in sorted(c.items())))
    return 0


if __name__ == '__main__':
    sys.exit(main())
