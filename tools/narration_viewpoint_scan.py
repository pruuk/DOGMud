#!/usr/bin/env python3
"""Emit candidate narration sites for the M1 viewpoint audit.

⚠️ THIS TOOL PRODUCES CANDIDATES, NOT VERDICTS. It scans a fixed window after
each actor-directed send, so it MISCLASSIFIES sites whose other viewpoints sit
further away: internal/usercommands/show.go reads as actor+actee here and is in
fact a correct trio. A human or agent reads each candidate and rules on it.

It also deliberately ignores messaging.Category. give.go sends one event's three
viewpoints as CategorySystem, CategorySystem and CategoryLoot, so the tag says
nothing about what kind of text a line is.
"""
import re, glob, io, json, sys

WINDOW = 16  # lines after an actor send to look for other viewpoints

ACTOR = re.compile(r'\b(?:user|actor)\.SendText\(')
OBSERVER = re.compile(r'\broom\.SendText(?:Visual)?(?:ToUser)?\(')
ACTEE = re.compile(r'\b(target\w*|victim\w*|defender\w*|recipient\w*|other\w*|receiver\w*)\.SendText\(')

def scan():
    out = []
    for path in sorted(glob.glob('internal/**/*.go', recursive=True)):
        if path.endswith('_test.go'):
            continue
        try:
            lines = io.open(path, encoding='utf-8', errors='replace').read().split('\n')
        except OSError:
            continue
        for i, line in enumerate(lines):
            if not ACTOR.search(line):
                continue
            window = '\n'.join(lines[i:i + WINDOW])
            actee = bool(ACTEE.search(window))
            observer = bool(OBSERVER.search(window))
            if not (actee or observer):
                continue  # single-viewpoint: refusal territory, not this arc
            out.append({
                'file': path.replace('\\', '/'),
                'line': i + 1,
                'actor': True,
                'actee': actee,
                'observer': observer,
            })
    return out

if __name__ == '__main__':
    sites = scan()
    if '--json' in sys.argv:
        print(json.dumps(sites, indent=2))
    else:
        for s in sites:
            vp = 'actor' + ('+actee' if s['actee'] else '') + ('+observer' if s['observer'] else '')
            print('%-52s :%-5d %s' % (s['file'], s['line'], vp))
        print('\n%d candidate sites' % len(sites))
