# Messaging Unification M0 — The Sweep: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a mechanically-derived, machine-checkable inventory of every
player-facing text surface in DOGMud, and lock it with a guard test that fails
when an unregistered surface appears.

**Architecture:** Two deliverables and one document. A Python enumerator
(`tools/messaging_surface_audit.py`) walks the data tree and the Go tree and
emits both a human report and a machine-readable inventory. A repo-root Go guard
test (`messaging_surface_guard_test.go`) holds the registry as a documented map —
the same idiom as `durable_write_guard_test.go` — and fails in **both**
directions: an unregistered text-key spelling fails, and a registry entry that no
longer matches anything also fails. The sweep document is written from the tool's
output, not from memory.

**Tech Stack:** Python 3 (stdlib only, matching `tools/id_inventory.py` and
`tools/context_md_audit.py`), Go 1.x `testing` + `go/ast` + `io/fs`.

**Spec:** [`2026-08-31-messaging-unification-design.md`](../specs/2026-08-31-messaging-unification-design.md)

**No production behavior changes in M0.** Nothing under `internal/` or `modules/`
is modified. The only Go file added is a test.

---

## Why the registry keys on KEY SPELLINGS, not paths

The design assumed a store inventory. Building this plan disproved that: a
mechanical key enumeration immediately found two surfaces the curated list had
missed — **`idlemessages`, at 1,285 occurrences across room files, the largest
single narration surface in the game** — and behavior-tree nodes emitting
`user_text` / `room_text`. It also found four singleton spellings
(`texts:`, `saytext:`, `room_text:`, `on_use_user_text:`) that are almost
certainly drift.

A path registry would not have caught any of them, because they live inside
`rooms/`, `behaviors/` and `items/` — directories nobody thinks of as message
stores. **A key-spelling registry catches a new surface the moment someone
invents a key for it**, which is the property the arc needs to keep the inventory
from rotting the way the quell claim did.

---

## File Structure

| File | Responsibility |
|---|---|
| `tools/messaging_surface_audit.py` | Enumerate text-bearing YAML keys and Go send sites. Emit human report + `--json`. Read-only. |
| `messaging_surface_guard_test.go` (repo root) | Registry of known key spellings and send sites, with a reason per entry. Fails both directions. |
| `docs/superpowers/audits/2026-08-31-messaging-surface-sweep.md` | The sweep document, written from tool output. |

---

## Task 1: The YAML key enumerator

**Files:**
- Create: `tools/messaging_surface_audit.py`

- [ ] **Step 1: Create the tool with the YAML key walk**

```python
#!/usr/bin/env python3
"""Enumerate every player-facing text surface in the world data and Go tree.

M0 of the messaging unification arc. The arc exists because curated inventories
rot: a "verified" claim that quell had no messages survived two weeks past its
own fix, and a hand-built store list missed `idlemessages` -- 1,285 occurrences
and the largest narration surface in the game.

So this walks by PROPERTY, not by name. Anything that looks like it holds
player-facing text is reported, and `messaging_surface_guard_test.go` requires
every key spelling found here to be registered with a reason.

Usage:
    python tools/messaging_surface_audit.py            # human report
    python tools/messaging_surface_audit.py --json     # machine inventory

Filename-only YAML scanning, no yaml library needed -- same approach as
tools/id_inventory.py, and deliberate: a real parse would need every loader's
schema, and the point is to find keys no loader owns yet.
"""

import argparse
import json
import os
import re
import sys
from collections import defaultdict

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WORLD = os.path.join(REPO, "_datafiles", "world", "dogmud")

# Directories that are runtime state, not authored content. Instance saves
# mirror templates, user saves are per-player, shops/guilds/moderation are
# living state (see CLAUDE.md).
SKIP_DIRS = {
    "mobs.instances", "rooms.instances", "users", "shops",
    "guilds", "moderation", "plugin-data", "warehouses",
}

# A key is a text candidate if its name contains any of these stems. Broad on
# purpose: over-reporting costs a registry line, under-reporting hides a
# surface.
KEY_STEMS = (
    "text", "message", "msg", "lines", "hint", "prose", "desc",
    "say", "emote", "voice", "phrase", "greeting", "taunt",
)

# Audience/role keys carry no stem but ARE the narration shape.
AUDIENCE_KEYS = {
    "toattacker", "todefender", "toroom", "observers",
    "controller", "controlled", "together", "separate",
    "options", "optionid",
}

KEY_RE = re.compile(r"^\s*([a-z_][a-z0-9_]*):", re.IGNORECASE)


def is_candidate(key):
    k = key.lower()
    if k in AUDIENCE_KEYS:
        return True
    return any(stem in k for stem in KEY_STEMS)


def walk_yaml():
    """Yield (key, repo_relative_path) for every candidate key occurrence."""
    for root, dirs, files in os.walk(WORLD):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for name in files:
            if not name.endswith((".yaml", ".yml")):
                continue
            full = os.path.join(root, name)
            rel = os.path.relpath(full, REPO).replace("\\", "/")
            try:
                with open(full, "r", encoding="utf-8", errors="replace") as fh:
                    for line in fh:
                        m = KEY_RE.match(line)
                        if m and is_candidate(m.group(1)):
                            yield m.group(1).lower(), rel
            except OSError as exc:
                print(f"WARN: cannot read {rel}: {exc}", file=sys.stderr)


def collect():
    keys = defaultdict(lambda: {"count": 0, "dirs": set()})
    for key, rel in walk_yaml():
        entry = keys[key]
        entry["count"] += 1
        entry["dirs"].add(os.path.dirname(rel).replace("\\", "/"))
    return keys


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--json", action="store_true", help="machine-readable output")
    args = ap.parse_args()

    keys = collect()
    if args.json:
        out = {
            k: {"count": v["count"], "dirs": sorted(v["dirs"])}
            for k, v in sorted(keys.items())
        }
        print(json.dumps(out, indent=2, sort_keys=True))
        return

    print(f"TEXT-BEARING YAML KEYS ({len(keys)} distinct spellings)")
    print("=" * 72)
    for key, v in sorted(keys.items(), key=lambda kv: -kv[1]["count"]):
        dirs = sorted(v["dirs"])
        shown = ", ".join(d.split("/")[-1] for d in dirs[:3])
        more = f" +{len(dirs) - 3} more" if len(dirs) > 3 else ""
        print(f"{v['count']:6d}  {key:<24} {shown}{more}")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Run it and confirm it reproduces the independently-measured counts**

Run: `python tools/messaging_surface_audit.py`

Expected: a table whose top rows match the counts measured by hand during
design. These four are the check — if any differs, the walk is wrong, not the
data:

```
  3338  description
  1878  text
  1285  idlemessages
  1266  hints
```

`idlemessages` at 1285 is the specific one to verify: it is the surface the
curated inventory missed, and its presence proves the walk reaches `rooms/`.

- [ ] **Step 3: Confirm the JSON mode is machine-readable**

Run: `python tools/messaging_surface_audit.py --json | python -c "import json,sys; d=json.load(sys.stdin); print(len(d), 'keys'); print(d['idlemessages']['count'])"`

Expected: prints the key count, then `1285`.

- [ ] **Step 4: Commit**

```bash
git add tools/messaging_surface_audit.py
git commit -m "feat(tools): enumerate text-bearing YAML keys for the messaging sweep"
```

---

## Task 2: Add the Go-side send-site and token-engine enumeration

**Files:**
- Modify: `tools/messaging_surface_audit.py`

- [ ] **Step 1: Add the Go walk above `main()`**

```python
GO_ROOTS = ("internal", "modules")

# A raw queue push bypasses the messaging pipeline entirely: no Category, so no
# color, no normalize, no sight gate, no verbosity. internal/characters/
# progression.go does this six times and is the known bypass.
RAW_SEND_RE = re.compile(r"events\.AddToQueue\(events\.Message\{")

# Token substitution entry points. Two engines exist with an overlapping
# vocabulary ({source} and {target} mean different things depending on which
# one renders); a third appearing is exactly what this should catch.
TOKEN_ENTRY_RE = re.compile(r"func\s+(SubstituteTokens|SetTokenValue)\b")

# Delivery-layer surface. The spec puts the pipeline in M0's scope alongside the
# stores, because the two are coupled through Category: auditing a store without
# knowing what the pipeline does to its output audits half a system.
SEND_HELPER_RE = re.compile(r"func\s+(\([^)]*\)\s*)?(SendText|SendTextVisual|SendTextVisualToUser|SendPhaseText)\b")
CATEGORY_RE = re.compile(r"^\tCategory[A-Z][A-Za-z]*\b")
WRAPPER_CFG_RE = re.compile(r"(Enter|Exit)RoomMessageWrapper")


def walk_go():
    """Yield (kind, repo_relative_path, line_no) for Go-side surfaces."""
    for root_name in GO_ROOTS:
        for root, _dirs, files in os.walk(os.path.join(REPO, root_name)):
            for name in files:
                if not name.endswith(".go") or name.endswith("_test.go"):
                    continue
                full = os.path.join(root, name)
                rel = os.path.relpath(full, REPO).replace("\\", "/")
                try:
                    with open(full, "r", encoding="utf-8", errors="replace") as fh:
                        for n, line in enumerate(fh, 1):
                            if RAW_SEND_RE.search(line):
                                yield "raw_send", rel, n
                            if TOKEN_ENTRY_RE.search(line):
                                yield "token_engine", rel, n
                            if SEND_HELPER_RE.search(line):
                                yield "send_helper", rel, n
                            if CATEGORY_RE.search(line):
                                yield "category", rel, n
                            if WRAPPER_CFG_RE.search(line):
                                yield "config_wrapper", rel, n
                except OSError as exc:
                    print(f"WARN: cannot read {rel}: {exc}", file=sys.stderr)
```

- [ ] **Step 2: Wire it into `collect()` and both output modes**

Replace `collect()` and `main()` with:

```python
def collect():
    keys = defaultdict(lambda: {"count": 0, "dirs": set()})
    for key, rel in walk_yaml():
        entry = keys[key]
        entry["count"] += 1
        entry["dirs"].add(os.path.dirname(rel).replace("\\", "/"))

    go = defaultdict(list)
    for kind, rel, line in walk_go():
        go[kind].append(f"{rel}:{line}")
    return keys, go


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--json", action="store_true", help="machine-readable output")
    args = ap.parse_args()

    keys, go = collect()
    if args.json:
        out = {
            "yaml_keys": {
                k: {"count": v["count"], "dirs": sorted(v["dirs"])}
                for k, v in sorted(keys.items())
            },
            "go": {k: sorted(v) for k, v in sorted(go.items())},
        }
        print(json.dumps(out, indent=2, sort_keys=True))
        return

    print(f"TEXT-BEARING YAML KEYS ({len(keys)} distinct spellings)")
    print("=" * 72)
    for key, v in sorted(keys.items(), key=lambda kv: -kv[1]["count"]):
        dirs = sorted(v["dirs"])
        shown = ", ".join(d.split("/")[-1] for d in dirs[:3])
        more = f" +{len(dirs) - 3} more" if len(dirs) > 3 else ""
        print(f"{v['count']:6d}  {key:<24} {shown}{more}")

    for kind, label in (
        ("raw_send", "RAW events.Message PUSHES (bypass the pipeline)"),
        ("token_engine", "TOKEN SUBSTITUTION ENTRY POINTS"),
        ("send_helper", "SEND HELPERS (delivery-layer entry points)"),
        ("category", "MESSAGE CATEGORIES"),
        ("config_wrapper", "CONFIG-DRIVEN MESSAGE WRAPPERS"),
    ):
        sites = go.get(kind, [])
        print()
        print(f"{label} ({len(sites)})")
        print("=" * 72)
        for s in sites:
            print(f"  {s}")
```

- [ ] **Step 3: Run and verify against the hand-measured Go facts**

Run: `python tools/messaging_surface_audit.py | tail -25`

Expected, verified by hand during design:
- **13 raw `events.Message` pushes**, of which **6 are in
  `internal/characters/progression.go`** — the known bypass — and the rest are
  the pipeline's own plumbing in `internal/rooms/rooms.go` (5),
  `internal/users/userrecord.go` (1) and `internal/usercommands/print.go` (1).
- **2 token substitution entry points**: `internal/textutil/tokens.go`
  (`SubstituteTokens`) and `internal/items/itemspec.go` (`SetTokenValue`).
- **3 distinct send helpers**: `SendText` and `SendTextVisual` (plus
  `SendTextVisualToUser`) on `internal/rooms`, and `SendPhaseText` in
  `internal/textutil/spelltext.go`.
- **60 `Category` constants**, all in `internal/messaging/messaging.go`.
- **`EnterRoomMessageWrapper` / `ExitRoomMessageWrapper`** in
  `internal/configs/config.textformats.go` — the config-driven decoration layer.

If the category count differs from 60, the enum changed since the design sweep;
record the new number in the document rather than "correcting" it here.

- [ ] **Step 4: Commit**

```bash
git add tools/messaging_surface_audit.py
git commit -m "feat(tools): enumerate pipeline bypasses and token engines in the messaging audit"
```

---

## Task 3: The guard test, written to fail first

**Files:**
- Create: `messaging_surface_guard_test.go` (repo root, `package main`)

- [ ] **Step 1: Write the guard with an EMPTY registry so it fails loudly**

```go
package main

// messaging_surface_guard_test.go — messaging unification arc, M0.
//
// THE PROBLEM THIS GUARDS. The arc's inventory kept growing every time someone
// looked harder. A curated store list missed `idlemessages` (1,285 occurrences,
// the largest narration surface in the game), behavior-tree `user_text` /
// `room_text`, and four singleton key spellings. A "verified" memory that quell
// had no messages survived two weeks past its own fix.
//
// So the inventory is not a document anyone maintains by hand. It is this
// registry, and the walk below fails in BOTH directions:
//
//   * a text-bearing key spelling that is not registered fails, which is how a
//     newly-invented surface announces itself; and
//   * a registered spelling that no longer appears anywhere ALSO fails, which
//     is how a stale registry announces itself. A guard that only checks one
//     direction rots into a guard that passes forever.
//
// Registry entries carry a SCOPE, because the messaging arc deliberately does
// not own everything that holds text:
//
//   narration — an event narrated at the player. The arc owns these.
//   content   — authored text a player reads on request (room and item
//               descriptions, dialogue, help). Out of scope by design.
//   config    — colour aliases, keyword tables. Not player prose at all.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type surfaceScope string

const (
	scopeNarration surfaceScope = "narration"
	scopeContent   surfaceScope = "content"
	scopeConfig    surfaceScope = "config"
)

type surfaceEntry struct {
	Scope  surfaceScope
	Reason string
}

// messagingSurfaces registers every text-bearing YAML key spelling in the world
// data. Populated in Task 4 from the audit tool's output.
var messagingSurfaces = map[string]surfaceEntry{}

var (
	guardKeyRe = regexp.MustCompile(`(?i)^\s*([a-z_][a-z0-9_]*):`)

	guardKeyStems = []string{
		"text", "message", "msg", "lines", "hint", "prose", "desc",
		"say", "emote", "voice", "phrase", "greeting", "taunt",
	}

	guardAudienceKeys = map[string]bool{
		"toattacker": true, "todefender": true, "toroom": true,
		"observers": true, "controller": true, "controlled": true,
		"together": true, "separate": true, "options": true,
		"optionid": true,
	}

	// Runtime state, not authored content. Mirrors SKIP_DIRS in
	// tools/messaging_surface_audit.py — keep the two in step.
	guardSkipDirs = map[string]bool{
		"mobs.instances": true, "rooms.instances": true, "users": true,
		"shops": true, "guilds": true, "moderation": true,
		"plugin-data": true, "warehouses": true,
	}
)

func guardIsCandidate(key string) bool {
	k := strings.ToLower(key)
	if guardAudienceKeys[k] {
		return true
	}
	for _, stem := range guardKeyStems {
		if strings.Contains(k, stem) {
			return true
		}
	}
	return false
}

// collectTextKeys walks the world data and returns every candidate key spelling
// found, mapped to one example file for the failure message.
func collectTextKeys(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}
	root := filepath.Join("_datafiles", "world", "dogmud")

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if guardSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, line := range strings.Split(string(data), "\n") {
			m := guardKeyRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			key := strings.ToLower(m[1])
			if !guardIsCandidate(key) {
				continue
			}
			if _, seen := found[key]; !seen {
				found[key] = filepath.ToSlash(path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return found
}

func TestEveryTextSurfaceIsRegistered(t *testing.T) {
	found := collectTextKeys(t)

	var unregistered []string
	for key, example := range found {
		if _, ok := messagingSurfaces[key]; !ok {
			unregistered = append(unregistered, key+"  (e.g. "+example+")")
		}
	}
	sort.Strings(unregistered)
	for _, u := range unregistered {
		t.Errorf("unregistered text-bearing key: %s\n"+
			"  Add it to messagingSurfaces with a scope and a reason.\n"+
			"  narration = the arc owns it; content = authored text read on\n"+
			"  request; config = not player prose.", u)
	}

	var stale []string
	for key := range messagingSurfaces {
		if _, ok := found[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, s := range stale {
		t.Errorf("registered key %q no longer appears in the world data.\n"+
			"  Remove it, or find out what deleted the surface.", s)
	}
}
```

- [ ] **Step 2: Run it and watch it fail on every key**

Run: `go test -run TestEveryTextSurfaceIsRegistered ./... 2>&1 | head -20`

Expected: FAIL, with roughly 30 `unregistered text-bearing key:` errors —
including `idlemessages`, `success_message`, `cast_user_text` and the singleton
`texts`. This proves the walk reaches the data before any registry exists.

- [ ] **Step 3: Commit the failing guard**

```bash
git add messaging_surface_guard_test.go
git commit -m "test: add the messaging surface guard, registry still empty"
```

---

## Task 4: Populate the registry from the tool's output

**Files:**
- Modify: `messaging_surface_guard_test.go`

- [ ] **Step 1: Generate the current key list**

Run: `python tools/messaging_surface_audit.py --json | python -c "import json,sys; [print(k) for k in sorted(json.load(sys.stdin)['yaml_keys'])]"`

Use the printed list as the authoritative set. Do not type it from memory.

- [ ] **Step 2: Replace the empty registry with the classified one**

Every key below was measured during the design sweep. Classify any key the tool
prints that is absent here rather than deleting it — an unexpected key is the
guard doing its job.

```go
var messagingSurfaces = map[string]surfaceEntry{
	// ── narration: the arc owns these ────────────────────────────────────
	"idlemessages":     {scopeNarration, "room and zone ambient flavor; 1,285 occurrences, the largest narration surface in the game. Rendered by hooks/NewRound_UserRoundTick.go"},
	"success_message":  {scopeNarration, "crafting outcome; recipes/, 126 occurrences. No audience split at all today"},
	"failure_message":  {scopeNarration, "crafting outcome; recipes/, 126 occurrences. No audience split at all today"},
	"tier_up_message":  {scopeNarration, "mutation tier advancement"},
	"playermessage":    {scopeNarration, "quest step narration, actor side"},
	"roommessage":      {scopeNarration, "quest step narration, observer side"},
	"cast_user_text":   {scopeNarration, "spell cast, actor side"},
	"cast_room_text":   {scopeNarration, "spell cast, observer side"},
	"wait_user_text":   {scopeNarration, "spell wind-up, actor side"},
	"wait_room_text":   {scopeNarration, "spell wind-up, observer side"},
	"start_user_text":  {scopeNarration, "buff applied, actor side"},
	"start_room_text":  {scopeNarration, "buff applied, observer side"},
	"end_user_text":    {scopeNarration, "buff expired, actor side"},
	"end_room_text":    {scopeNarration, "buff expired, observer side"},
	"trigger_user_text": {scopeNarration, "buff tick, actor side"},
	"trigger_room_text": {scopeNarration, "buff tick, observer side"},
	"user_text":        {scopeNarration, "behavior-tree node emission, actor side. A singleton spelling — candidate for merging into the canonical role vocabulary at M4"},
	"room_text":        {scopeNarration, "behavior-tree node emission, observer side. Singleton spelling, same note"},
	"on_use_user_text": {scopeNarration, "item use narration. Singleton spelling, same note"},
	"message":          {scopeNarration, "generic single-audience narration across several stores"},
	"lines":            {scopeNarration, "itemvoices event pools, keyed by event name"},
	"saytext":          {scopeConfig, "ansi-aliases.yaml; an alias definition, not prose"},
	"say":              {scopeConfig, "keywords.yaml; a command keyword, not prose"},

	// ── narration shape: audience and pool structure ─────────────────────
	"toattacker": {scopeNarration, "combat/defence/taunt triad, actor side"},
	"todefender": {scopeNarration, "combat/defence/taunt triad, actee side"},
	"toroom":     {scopeNarration, "combat/defence triad, observer side"},
	"observers":  {scopeNarration, "grapple's spelling of toroom"},
	"controller": {scopeNarration, "grapple's spelling of the actor role"},
	"controlled": {scopeNarration, "grapple's spelling of the actee role"},
	"together":   {scopeNarration, "attack-message shape: participants in one room"},
	"separate":   {scopeNarration, "attack-message shape: participants in different rooms"},
	"options":    {scopeNarration, "band-keyed pool container in the triad stores"},
	"optionid":   {scopeNarration, "pool identity in the triad stores"},

	// ── content: authored text read on request, OUT of scope ─────────────
	"description":         {scopeContent, "room, item, mob and quest descriptions"},
	"description_suffix":  {scopeContent, "appended description fragment"},
	"hidden_description":  {scopeContent, "description shown once a hidden thing is found"},
	"descriptionmodifier": {scopeContent, "conditional description fragment"},
	"text":                {scopeContent, "dialogue node text, spoken by an NPC on request"},
	"texts":               {scopeContent, "singleton plural spelling in one room file; drift, not a new surface"},
	"hints":               {scopeContent, "dialogue narrator hints shown to the player"},
}
```

- [ ] **Step 3: Run the guard and watch it pass**

Run: `go test -run TestEveryTextSurfaceIsRegistered ./...`

Expected: PASS. If a key is reported unregistered, the tool found something this
list does not have — classify it, do not delete it.

- [ ] **Step 4: Commit**

```bash
git add messaging_surface_guard_test.go
git commit -m "test: register every text-bearing key spelling with a scope and reason"
```

---

## Task 5: Prove the guard can fail, in both directions

A guard that cannot go red is worthless, and this repo has been burned by null
probes three times in one day. Both directions get proven, and the sabotage is
reverted immediately.

**Files:**
- Temporarily create then delete: `_datafiles/world/dogmud/buffs/zzz_guard_probe.yaml`
- Temporarily modify then restore: `messaging_surface_guard_test.go`

- [ ] **Step 1: Prove direction one — a NEW surface fails**

```bash
printf 'buffid: 999\nname: Guard Probe\nwhispered_room_text: "probe"\n' \
  > _datafiles/world/dogmud/buffs/zzz_guard_probe.yaml
go test -run TestEveryTextSurfaceIsRegistered ./... 2>&1 | head -5
```

Expected: FAIL naming `whispered_room_text` and the probe file.

- [ ] **Step 2: Remove the probe and confirm green again**

```bash
rm _datafiles/world/dogmud/buffs/zzz_guard_probe.yaml
go test -run TestEveryTextSurfaceIsRegistered ./...
```

Expected: PASS.

- [ ] **Step 3: Prove direction two — a STALE registry entry fails**

Add this line inside `messagingSurfaces`, run, then delete the line:

```go
	"nonexistent_probe_text": {scopeNarration, "temporary probe, delete me"},
```

Run: `go test -run TestEveryTextSurfaceIsRegistered ./... 2>&1 | head -5`

Expected: FAIL with `registered key "nonexistent_probe_text" no longer appears`.

- [ ] **Step 4: Delete the probe line and confirm green**

Run: `go test -run TestEveryTextSurfaceIsRegistered ./...`

Expected: PASS.

- [ ] **Step 5: Confirm the tree is clean before committing**

```bash
git status --short
```

Expected: empty. If the probe file or probe line survives, remove it now.

- [ ] **Step 6: Commit the verification note**

Append to the guard file's header comment, directly under the "both directions"
paragraph:

```go
// Both directions were verified on 2026-08-31 by adding an unregistered
// `whispered_room_text` key to a probe buff (guard failed, naming it) and by
// adding a `nonexistent_probe_text` registry entry (guard failed, naming it).
// Re-verify the same way after any change to the walk.
```

```bash
git add messaging_surface_guard_test.go
git commit -m "test: record that the messaging surface guard fails in both directions"
```

---

## Task 6: Write the sweep document from the tool output

**Files:**
- Create: `docs/superpowers/audits/2026-08-31-messaging-surface-sweep.md`
- Modify: `docs/README.md`

- [ ] **Step 1: Capture the tool output**

```bash
python tools/messaging_surface_audit.py > /tmp/sweep.txt
python tools/messaging_surface_audit.py --json > /tmp/sweep.json
wc -l /tmp/sweep.txt
```

- [ ] **Step 2: Write the document**

Structure, with every number taken from `/tmp/sweep.txt` and **not** from this
plan or from the design spec:

1. **How this was produced** — the exact command, the date, and the statement
   that the numbers are tool output. Anyone re-running it should get the same
   answer or a diff worth investigating.
2. **The key table** — every spelling, its count, its scope, its directories,
   sorted by count descending so the large surfaces are impossible to miss. Use
   exactly this header, so the table can be diffed against a later re-run:

   ```markdown
   | Key | Count | Scope | Directories |
   |---|---:|---|---|
   ```

   The `Scope` column must agree with `messagingSurfaces` in the guard test. If
   writing the row makes a classification look wrong, change the guard and the
   document together — they are one statement in two places.
3. **Narration surfaces, categorised** for the arc, using the design's
   classification: *fits the one model* / *fits only if the core supports X* /
   *deliberately out of scope* / *not narration at all*.
4. **Go-side findings** — the 13 raw `events.Message` pushes split into the 6
   progression bypasses and the 7 legitimate plumbing sites, and the 2 token
   engines.
5. **The delivery layer** — the 3 send helpers, the 60 `Category` constants, the
   config-driven enter/exit wrappers, and two facts verified during design that
   belong in the record: the pipeline's **wrap stage never runs** (`shouldWrap`
   returns false for every category, while `internal/messaging/context.md` still
   documents wrap as active — chunk 5.12 material), and `world/default/`
   shadows `world/dogmud/` with a parallel template tree.
6. **Drift found** — the singleton spellings (`texts`, `saytext`, `room_text`,
   `on_use_user_text`, `user_text`) and what each should become at M4.
7. **What the curated inventory missed**, named explicitly: `idlemessages` at
   1,285, and behavior-tree `user_text` / `room_text`. This is the evidence for
   why M0 is mechanical, and it belongs in the record.

- [ ] **Step 3: Index the document**

Add to the table in `docs/README.md`, next to the other audit rows:

```markdown
| [`superpowers/audits/2026-08-31-messaging-surface-sweep.md`](superpowers/audits/2026-08-31-messaging-surface-sweep.md) | M0 of the messaging arc: a mechanically-derived inventory of every text-bearing key spelling and every pipeline bypass, locked by `messaging_surface_guard_test.go` |
```

- [ ] **Step 4: Confirm the guard still passes and the build is clean**

```bash
gofmt -l . | grep -v vendor
go build ./...
go test -run TestEveryTextSurfaceIsRegistered ./...
```

Expected: `gofmt` prints nothing, build succeeds, test passes.

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/audits/2026-08-31-messaging-surface-sweep.md docs/README.md
git commit -m "docs(audit): the messaging surface sweep, derived mechanically"
```

---

## Done when

1. `tools/messaging_surface_audit.py` reproduces the hand-measured counts,
   `idlemessages` at 1,285 among them.
2. `TestEveryTextSurfaceIsRegistered` passes, and has been **shown to fail** in
   both directions with the sabotage recorded in its header.
3. The sweep document exists, is indexed in `docs/README.md`, and every number
   in it came from tool output.
4. `gofmt -l` is clean and `go build ./...` succeeds.
5. **No file under `internal/` or `modules/` was modified.** M0 changes no
   production behavior; the only Go file added is a test.

## Explicitly NOT in M0

- The snapshot harness. That is M1, and it needs the injected picker first.
- Any change to a message, a loader, or a role vocabulary.
- Fixing the `{source_plain}` anonymizer leak — owner deferred it to M5.
- The crime-in-the-dark gate — M5, and blocked on the `NightVision` decision.
