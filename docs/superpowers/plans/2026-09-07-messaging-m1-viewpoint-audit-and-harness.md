# Messaging M1 — Viewpoint Audit and Snapshot Harness — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a ruled audit of every hand-rolled narration site — which
viewpoints it addresses, and whether that is correct or a gap — then freeze
today's narration output so M2 can refactor against a net.

**Architecture:** Task 1 is the audit and is the stage's main deliverable: a
markdown document every later stage reads. Task 2 lands the one production
change M1 owns, an injected picker, because two stores bypass the engine's
randomness seam and are otherwise unsnapshottable. Tasks 3 and 4 build the two
halves of the net: rendered snapshots for keyed stores, an AST inventory for
in-code sites.

**Tech Stack:** Go 1.x, `go/ast` + `go/parser` (the repo's existing guard
idiom), `gopkg.in/yaml.v2`, Python 3 for the audit scan tool.

**Source spec:** `docs/superpowers/specs/2026-08-31-messaging-unification-design.md`,
section **M1** (line 337).

---

## Facts verified against source, 2026-09-07

Read from the files at plan-writing time.

| Fact | Evidence |
|---|---|
| In-code narration splits **177 actor+observer / 50 actor+actee+observer / 20 actor+actee = 247 sites across 80 files** | 16-line-window scan of `user.SendText` / `actor.SendText` against `room.SendText(Visual)` and `target*/victim*/defender*/recipient*/other*/receiver*.SendText` |
| 🔴 That scan produces **FALSE POSITIVES** | `internal/usercommands/show.go` scans as actor+actee; its player branch is in fact a full trio whose `room.SendTextVisual` sits ~14 lines below, and its mob branch correctly has no actee |
| A real GAP exists and is a good exemplar | `internal/hooks/spell_resolution.go:395` sends *"Your spell backfires violently, wounding you!"* to the caster and nothing to the room |
| 🔴 **`messaging.Category` is NOT a reliable classifier** | `internal/usercommands/give.go:90-99` sends one event's three viewpoints as `CategorySystem`, `CategorySystem`, `CategoryLoot` |
| `CategorySystem` is ~2,003 of 2,936 category references, and is essentially always actor-directed | only **1** `CategorySystem` room broadcast exists (`internal/hooks/pinnacle_tick.go`) |
| Full-trio sites cluster in the special-move verbs | trip 8, grapple 6, bash 4, gore 4, kick 4, pounce 4, throttle 4, drain 3, give 3, maul 3, rake 3, report 1 |
| **Two stores bypass `util.Rand`** | `internal/grapplemessaging/render.go:52` `available[rand.Intn(len(available))]`; `internal/spells/casting_messages.go:96` `pool[rand.Intn(len(pool))]`; both import `math/rand` |
| The engine seam is `util.Rand(maxInt int) int` | `internal/util/util.go:203`, itself `rand.Intn` |
| The keyed-store picker already routes through the seam | `items.MessageOptions.Get` (`internal/items/attack_messages.go:59-75`) calls `util.Rand(ct)`, with a `seedNum ...int` override |
| The functions needing a picker parameter | `grapplemessaging.PickTemplate(pool []string, cooldowns map[string]bool, keyPrefix string) string` (`:30`); `spells.GetCastMessage(category, spellName string) string` (`:77`) |
| `indexOverride` exists and is the same idea | `internal/combat/defence_multiplier.go:294` `RenderChannelDefenceMessages(..., indexOverride ...int)`; also `internal/hooks/spell_resolution.go:477` |
| M0's guard exists at the REPO ROOT, `package main`, 541 lines, one test | `messaging_surface_guard_test.go` → `TestEveryTextSurfaceIsRegistered`; carries a `surfaceScope` enum of `narration / content / config` |
| The audit scan tool exists and already walks Go | `tools/messaging_surface_audit.py`, 589 lines, has `walk_go()` at `:281` |
| Narration stores live in **five** locations | `combat-messages/`, `defense-messages/`, `taunt-messages/`, `messaging/`, and the bare file `casting-messages.yaml` at the tree root |

---

## File Structure

**Created**

| File | Responsibility |
|---|---|
| `docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md` | **The deliverable.** Every candidate site, its viewpoints, and a `correct`/`gap` verdict with reason |
| `tools/narration_viewpoint_scan.py` | Produces audit CANDIDATES; never verdicts |
| `internal/narration/picker.go` | The injected `Picker` type and the production default |
| `internal/narration/picker_test.go` | Determinism and the production-default contract |
| `internal/narration/snapshot_test.go` | Rendered snapshots of every keyed store |
| `internal/narration/testdata/stores/*.golden` | The frozen store output |

**Modified**

| File | Responsibility |
|---|---|
| `internal/grapplemessaging/render.go` | `PickTemplate` takes a picker |
| `internal/spells/casting_messages.go` | `GetCastMessage` takes a picker |
| `internal/items/attack_messages.go` | `MessageOptions.Get` routes through the picker |
| `messaging_surface_guard_test.go` | Gains the in-code narration inventory |

---

## Task 1: The viewpoint audit

**This is the stage's deliverable.** Everything after it is scaffolding. It is
deliberately first: the harness is worth little until we know which sites are
correctly shaped and which are quietly missing a viewpoint.

**Files:**
- Create: `tools/narration_viewpoint_scan.py`
- Create: `docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md`

- [ ] **Step 1: Write the candidate scanner**

Create `tools/narration_viewpoint_scan.py`. It emits CANDIDATES, not verdicts:

```python
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
```

- [ ] **Step 2: Run it and confirm the counts**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
python tools/narration_viewpoint_scan.py | tail -3
python tools/narration_viewpoint_scan.py | grep -c 'actor+actee+observer'
```

Expected: **247 candidate sites**, of which **50** are the full trio. If the
totals differ, the tree has moved since 2026-09-07 — record the new numbers in
the audit rather than forcing the old ones.

- [ ] **Step 3: Create the audit document with its method stated first**

Create `docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md`,
opening with exactly this framing so no reader mistakes candidates for verdicts:

```markdown
# Narration Viewpoint Audit (M1 Task 1)

**What this is:** every hand-rolled narration site in Go, the viewpoints it
addresses, and a ruling on whether that shape is correct or a gap.

**Why it exists:** the messaging arc's core is Actor / Actee / Observer. Most
sites address only two of the three. A 2-of-3 site is NOT automatically wrong —
the actee may be a mob, which receives no narration, or the exchange may be
genuinely private. But some are gaps: a public act whose room message was never
written. Only reading the site tells them apart, and M2 must know which is
which before it designs a core.

## Method, and its known weakness

Candidates come from `tools/narration_viewpoint_scan.py`, which scans a 16-line
window after each actor-directed send.

⚠️ **The scan produces FALSE POSITIVES.** `internal/usercommands/show.go` reads
as actor+actee and is in fact a correct trio, its room send sitting just past
the window; its mob branch legitimately has no actee. Every candidate below was
READ before it was ruled on.

⚠️ **`messaging.Category` is not used as a classifier and must not be.**
`give.go` sends one event's three viewpoints as `CategorySystem`,
`CategorySystem` and `CategoryLoot`. The tag says nothing about what kind of
text a line is.

## Verdict key

| Verdict | Meaning |
|---|---|
| `correct` | The missing viewpoint SHOULD be missing. Reason given. |
| `gap` | A viewpoint that should exist does not. What it should say is named. |
| `unsure` | Could not rule from the code alone. Escalate; do not guess. |

## Sites

| File:line | Event | A | Ae | Ob | Verdict | Cats | Reason / missing viewpoint |
|---|---|:-:|:-:|:-:|---|---|---|

`Cats` records **how many distinct `messaging.Category` values this one event
spans**. It is evidence for the later refusal arc, not a verdict input:
`give.go` spans two (`CategorySystem`, `CategoryLoot`) for a single event, which
is why neither arc can select its work by tag.

```

- [ ] **Step 4: Rule on every candidate**

Work the list. For each site, open the file, read the surrounding function, and
fill one row. Recognised `correct` reasons, so they are not re-derived 247
times:

- **actee is a mob** — mobs receive no player narration. `show.go`'s mob branch.
- **deliberately private** — a reply, a guild invite, an admin action on a
  player. The room has no business seeing it.
- **actor-only by nature** — a refusal or a state readout that no one else can
  observe.
- **observer covered elsewhere** — the room message exists but outside the
  window. Note the line it is on. **This is the false-positive case; expect
  several.**

Mark `gap` when a publicly observable act tells the room nothing. The known
exemplar to include: `internal/hooks/spell_resolution.go:395` sends *"Your spell
backfires violently, wounding you!"* to the caster only. A spell backfiring in
front of people is not private; the missing observer line is named, not written.

⚠️ **`unsure` is a legitimate verdict and must not be avoided.** A wrong
`correct` silently blesses a gap; a wrong `gap` sends M2 writing text nobody
wanted. Escalate rather than guess.

- [ ] **Step 5: Summarise what M2 must absorb**

Close the audit with a section stating:

- counts by verdict (`correct` / `gap` / `unsure`);
- the `gap` list on its own, because that is M2's list of viewpoints it must be
  able to emit that nothing emits today;
- the observation that full-trio sites cluster in the twelve special-move verbs
  (trip, grapple, bash, gore, kick, pounce, throttle, drain, give, maul, rake,
  report) — one pattern copied twelve times, in the same verbs whose cooldown
  handling was unified on 2026-09-07.

- [ ] **Step 6: Index the document**

Add a row to `docs/README.md` next to the other `superpowers/audits/` entries,
one line, describing what the audit rules on.

- [ ] **Step 7: Commit**

```bash
git add tools/narration_viewpoint_scan.py docs/superpowers/audits/ docs/README.md
git commit -F - <<'MSG'
docs(audit): rule on every hand-rolled narration site's viewpoints

The messaging core is Actor / Actee / Observer, and most in-code narration
addresses only two of the three. That is not automatically wrong -- the actee
may be a mob, which receives no narration, or the exchange may be genuinely
private -- but some are gaps, where a public act tells the room nothing.

247 candidate sites across 80 files: 177 actor+observer, 50 the full trio, 20
actor+actee. Each was read and ruled on, because the scan that finds them looks
in a fixed window and produces false positives: show.go reads as actor+actee and
is in fact a correct trio whose room send sits just past the window.

The scan deliberately ignores messaging.Category. give.go sends one event's
three viewpoints as CategorySystem, CategorySystem and CategoryLoot, so the tag
cannot classify what kind of text a line is.

The gap list is what M2 needs: the viewpoints it must be able to emit that
nothing emits today.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 2: The injected picker

**M1's one production change, owned deliberately.** Two stores use stdlib
`math/rand` and are unreachable by any seam, so without this the net has holes
exactly where two stores live.

**Files:**
- Create: `internal/narration/picker.go`, `internal/narration/picker_test.go`
- Modify: `internal/grapplemessaging/render.go`, `internal/spells/casting_messages.go`, `internal/items/attack_messages.go`

- [ ] **Step 1: Write the failing test**

Create `internal/narration/picker_test.go`:

```go
package narration

import "testing"

// TestDefaultPickerStaysInRange pins the production picker's contract: a valid
// index for any positive n, and a safe 0 for a degenerate pool.
func TestDefaultPickerStaysInRange(t *testing.T) {
	for _, n := range []int{1, 2, 5, 50} {
		for i := 0; i < 200; i++ {
			got := DefaultPicker(n)
			if got < 0 || got >= n {
				t.Fatalf("DefaultPicker(%d) = %d, out of range", n, got)
			}
		}
	}
	if got := DefaultPicker(0); got != 0 {
		t.Errorf("DefaultPicker(0) = %d, want 0", got)
	}
	if got := DefaultPicker(-3); got != 0 {
		t.Errorf("DefaultPicker(-3) = %d, want 0", got)
	}
}

// TestSequencePickerIsDeterministic pins what the harness needs: the SAME
// sequence every run, independent of any global seed.
//
// ⚠️ A global seed would be process-wide, and this repo runs all tests in ONE
// binary -- so a seeded run's output would depend on which other tests ran
// first. That order-dependence is the trap this injected picker exists to
// avoid, and it is why the harness must never reach for rand.Seed.
func TestSequencePickerIsDeterministic(t *testing.T) {
	first := SequencePicker()
	second := SequencePicker()

	for i := 0; i < 20; i++ {
		a, b := first(7), second(7)
		if a != b {
			t.Fatalf("call %d: two fresh sequence pickers diverged (%d vs %d)", i, a, b)
		}
		if a < 0 || a >= 7 {
			t.Fatalf("call %d: %d out of range for n=7", i, a)
		}
	}
}

// TestSequencePickerCoversThePool pins that snapshots see EVERY entry rather
// than the same one repeatedly -- a picker that always returned 0 would satisfy
// determinism while freezing only a fraction of the text.
func TestSequencePickerCoversThePool(t *testing.T) {
	pick := SequencePicker()
	seen := map[int]bool{}
	for i := 0; i < 12; i++ {
		seen[pick(4)] = true
	}
	for want := 0; want < 4; want++ {
		if !seen[want] {
			t.Errorf("index %d never selected; the sequence must cover the pool", want)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/narration/ -run TestDefaultPicker -v`

Expected: the package does not exist yet. That is the correct first failure.

- [ ] **Step 3: Write the picker**

Create `internal/narration/picker.go`:

```go
// Package narration owns the seam every narration store uses to choose a line
// from a pool, so snapshot runs can be deterministic without touching global
// randomness.
package narration

import "github.com/GoMudEngine/GoMud/internal/util"

// Picker chooses an index in [0,n). Production supplies the engine's
// randomness; the snapshot harness supplies a fixed sequence.
type Picker func(n int) int

// DefaultPicker is what production uses. It routes through util.Rand, which is
// the engine's single randomness seam, rather than stdlib rand.
func DefaultPicker(n int) int {
	if n < 1 {
		return 0
	}
	return util.Rand(n)
}

// SequencePicker returns a picker that walks indices in order, wrapping at n.
//
// ⚠️ IT IS A CLOSURE, NOT A SEED, AND THAT IS THE WHOLE POINT. Seeding global
// randomness would be process-wide, and this repo runs every test in ONE
// binary, so a seeded snapshot's output would depend on which tests ran before
// it. Relative state that passes or fails by order is a trap this codebase has
// already been bitten by. Each caller gets its own counter instead.
func SequencePicker() Picker {
	i := 0
	return func(n int) int {
		if n < 1 {
			return 0
		}
		v := i % n
		i++
		return v
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/narration/ -v`
Expected: PASS, three tests.

- [ ] **Step 5: Thread the picker through the two stdlib stores**

⚠️ **Use a variadic optional picker so no existing caller changes.** These are
narration paths with many callers; a required parameter turns a two-file change
into a sweep, and this task is meant to stay small.

In `internal/grapplemessaging/render.go`, change:

```go
func PickTemplate(pool []string, cooldowns map[string]bool, keyPrefix string) string {
```

to:

```go
// PickTemplate chooses a template, avoiding recently used ones.
//
// The optional picker exists for the snapshot harness. Production passes none
// and gets narration.DefaultPicker, which routes through the engine's util.Rand
// seam. This file used stdlib math/rand directly until 2026-09-07, which made
// it unreachable by any seam and therefore unsnapshottable.
func PickTemplate(pool []string, cooldowns map[string]bool, keyPrefix string, picker ...narration.Picker) string {
```

and replace the selection line (`:52`):

```go
	pick := available[rand.Intn(len(available))]
```

with:

```go
	choose := narration.DefaultPicker
	if len(picker) > 0 && picker[0] != nil {
		choose = picker[0]
	}
	pick := available[choose(len(available))]
```

Remove the now-unused `math/rand` import and add the `narration` import.

In `internal/spells/casting_messages.go`, apply the same shape to
`GetCastMessage(category, spellName string) string` at `:77`, replacing
`pool[rand.Intn(len(pool))]` at `:96`.

⚠️ **Check for an import cycle before adding the import**: `go list -deps
./internal/narration | grep -c "GoMud/internal/spells"`. A non-zero count means
`narration` already depends on `spells`, and the picker type must move somewhere
neither imports. Report rather than improvising.

- [ ] **Step 6: Route the keyed-store picker through the same seam**

`items.MessageOptions.Get` (`internal/items/attack_messages.go:59`) already
calls `util.Rand`, so it is correct today but bypasses the injectable seam.
Give it the same optional picker, keeping its existing `seedNum ...int`
behaviour untouched.

⚠️ **Do not remove `seedNum`.** `RenderChannelDefenceMessages`' `indexOverride`
threads into it, and this task is not a refactor of that path.

- [ ] **Step 7: Verify no behaviour changed**

```bash
go build ./... && go test ./...
gofmt -l internal/ modules/
grep -rn "math/rand" internal/grapplemessaging/ internal/spells/casting_messages.go
```

Expected: PASS everywhere, gofmt silent, and no `math/rand` left in those two
files. **This is a refactor: a failing test means behaviour changed**, and the
fix is to restore the old semantics, not to update the test.

- [ ] **Step 8: Prove the seam actually bites**

Write a throwaway check that `PickTemplate` with a `SequencePicker` returns
successive entries, run it, then delete it. Paste the output. Without this the
picker could be plumbed but ignored, and every later snapshot would silently be
random.

- [ ] **Step 9: Commit**

```bash
git add internal/narration/ internal/grapplemessaging/ internal/spells/ internal/items/
git commit -F - <<'MSG'
feat(narration): one injectable picker for every message pool

M1 promises no production behaviour changes and this is its one exception,
taken deliberately. internal/grapplemessaging and internal/spells chose their
lines with stdlib math/rand, unreachable by any seam controlling util.Rand, so
a snapshot harness would have shipped with holes exactly where two stores live.

The mechanism is an injected picker, NOT a global seed. Seeding is process-wide
and this repo runs every test in one binary, so a seeded snapshot's output would
depend on which tests ran before it -- order-dependent relative state is a trap
this codebase has already been bitten by. Each caller gets its own closure.

The picker is variadic and optional, so no existing caller changes and
production keeps routing through util.Rand.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 3: Snapshot the keyed stores

**Files:**
- Create: `internal/narration/snapshot_test.go`, `internal/narration/testdata/stores/`

- [ ] **Step 1: Enumerate the stores before writing anything**

```bash
ls _datafiles/world/dogmud/combat-messages/
ls _datafiles/world/dogmud/defense-messages/
ls _datafiles/world/dogmud/taunt-messages/
ls _datafiles/world/dogmud/messaging/
ls _datafiles/world/dogmud/casting-messages.yaml
```

Five locations, and the last is a bare FILE at the tree root rather than a
directory. Record the file count per store in the test's doc comment so a
deleted store is visible as a shrinking snapshot rather than as silence.

- [ ] **Step 2: Write the snapshot test**

Create `internal/narration/snapshot_test.go`. It walks every store, and for
each `(store × key × band × role)` renders with a fresh `SequencePicker`,
writing one golden file per store:

```go
package narration

// Snapshots freeze what every narration store emits TODAY, including paths
// nobody plays, so M2 can refactor against a net rather than against hope.
//
// Regenerate deliberately with -update after reading the diff. A snapshot that
// changes without a matching intent in the commit message is the regression
// this file exists to catch.
//
// ⚠️ EMPTY CASES ARE SNAPSHOTTED TOO. RenderDefenseMessage returns an empty
// triad for a missing or malformed pool; a refactor that makes that path start
// EMITTING text is equally a regression, so the golden records the emptiness.
```

Use a `-update` flag guard following the repo's existing golden idiom, and
assert byte equality otherwise. Tokens are substituted with fixed stand-ins
(`{target}` → `Target`, `{source}` → `Source`, and so on) so a token-rendering
change shows up as a diff rather than as noise.

- [ ] **Step 2b: Snapshot the POST-PIPELINE level too**

The raw render is only half the net. The same events must also be captured
**after delivery filtering**: at each sight verdict (full / shapes / none) and
each verbosity tier.

⚠️ **This catches a class the raw snapshot cannot see: "the text survived but
the delivery changed."** A refactor that leaves every string identical while
quietly changing who receives it, or what a blinded player sees, passes a
raw-only snapshot completely.

Find the pipeline's entry point before writing this — grep for the sight-verdict
and verbosity plumbing rather than assuming its shape — and record in the test's
doc comment which verdicts and tiers exist, so a newly added tier shows up as a
missing dimension rather than as silence.

- [ ] **Step 3: Generate and READ the goldens**

```bash
go test ./internal/narration/ -run TestStoreSnapshots -update
git diff --stat internal/narration/testdata/
```

⚠️ **Read a sample of the generated output before committing it.** A golden
full of empty strings, or of `%!s(MISSING)`, means the harness is rendering
wrongly and would freeze the bug. The goldens are evidence only if they contain
real sentences.

- [ ] **Step 4: Prove the snapshot can fail**

Change one line in one store YAML, run the test, confirm it fails naming that
store, then restore. **Paste both outputs.** A snapshot that cannot go red is
decoration.

- [ ] **Step 5: Verify and commit**

```bash
go test ./... && gofmt -l internal/
git add internal/narration/
git commit -F - <<'MSG'
test(narration): freeze what every keyed store emits today

Every (store x key x band x role) tuple rendered with a deterministic picker and
written to a golden, so M2 can refactor the narration core against a net.

Empty cases are frozen too: RenderDefenseMessage returns an empty triad on a
malformed pool, and a refactor that makes that path start emitting is equally a
regression.

The goldens were read, not just generated -- a file of empty strings or format
verbs would freeze a rendering bug and look like success.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 4: Inventory the in-code narration

Freezes the string LITERALS, not rendered output. That is the honest limit:
these sites have no bands or pools to render, and building ~247 fixtures for
text M2 may replace outright is the most expensive thing in the arc for the
least return.

**Files:**
- Modify: `messaging_surface_guard_test.go` (repo root, `package main`)

- [ ] **Step 1: Read the existing guard first**

```bash
sed -n '1,80p' messaging_surface_guard_test.go
grep -n "surfaceScope\|narration\|content\|config" messaging_surface_guard_test.go | head -20
```

It already carries a `surfaceScope` enum (`narration` / `content` / `config`)
and a locked registry of YAML key spellings. **Extend that idiom; do not invent
a second mechanism beside it.**

- [ ] **Step 2: Add the in-code inventory**

Add a second locked registry: every in-code narration site, keyed
`file:line`, with the viewpoints it addresses. Source it from the same scan
Task 1 uses, so the audit and the guard cannot disagree.

The test fails when a site appears that is not registered, or a registered site
vanishes — both directions, matching the existing guard's contract.

⚠️ **Registry entries must carry the Task 1 verdict**, so this file and the
audit stay in step. A site marked `gap` in the audit and absent here means one
of them was updated without the other.

- [ ] **Step 3: Prove it fails in both directions**

Add a narration site to any command, run, confirm red. Remove a registered one,
run, confirm red. Restore both, paste all output.

- [ ] **Step 4: Commit**

```bash
git add messaging_surface_guard_test.go
git commit -F - <<'MSG'
test(messaging): lock the in-code narration inventory

Extends M0's surface guard rather than adding a second mechanism beside it: the
same file already classifies YAML key spellings as narration / content / config,
and in-code narration is the same question asked of Go.

This freezes the string LITERALS, not rendered output. These sites have no bands
or pools to render, and ~247 fixtures for text M2 may replace outright is the
most expensive thing in the arc for the least return. The literals still catch
a site being deleted, edited, or added without anyone noticing.

Each entry carries its Task 1 verdict, so the guard and the audit cannot drift.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 5: Verification

- [ ] **Step 1: Gates**

```bash
gofmt -l internal/ modules/     # must print NOTHING
go build ./... && go vet ./...
go test ./...
```

Recorded flake: `TestCheckConcentrationBreak_ProgressionFiresOnEveryResolvedContest`
fails ~1% of runs from a self-relative attack fumble. Re-run before investigating.

- [ ] **Step 2: Boot test in an isolated worktree**

```bash
rm -rf _datafiles/world/dogmud/mobs.instances _datafiles/world/dogmud/rooms.instances
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is the SUCCESS case.** Do not grep for the bare word `panic` —
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. Clean up
with `git worktree remove --force`.

- [ ] **Step 3: No playtest gate this stage, and say why in the PR**

M1 authors no player-facing content: the audit is a document, the picker is a
seam, the snapshots and inventory are tests. The Content Playtest-Review Gate
applies from **M5**, where wording actually changes. Stating this in the PR
stops a reviewer assuming it was skipped.

- [ ] **Step 4: Ship through a PR**

```bash
git push -u origin HEAD
gh pr create --repo pruuk/DOGMud --base master --head "$(git branch --show-current)" --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

⚠️ Always pass `--repo pruuk/DOGMud`; this is a fork and `gh` defaults to the
upstream parent. A green check is not proof — confirm with `gh run view <id>
--repo pruuk/DOGMud --log-failed`. **Do not deploy**; the owner runs deploys.

---

## Done when

1. Every one of the ~247 candidate sites has a ruled verdict in the audit, and
   `unsure` was used where the code did not settle it — Task 1.
2. The audit's gap list stands alone, because it is M2's list of viewpoints
   nothing emits today — Task 1.
3. No narration store chooses a line outside the injectable seam; `math/rand` is
   gone from both stores — Task 2.
4. The picker is proven to actually bite, not merely be plumbed — Task 2 Step 8.
5. Every keyed store has a golden, including its empty cases, and the goldens
   contain real sentences — Task 3.
6. Every in-code narration site is locked in the guard with its verdict, failing
   in both directions — Task 4.
7. Build, vet, full suite and a clean boot — Task 5.

## Out of scope

- **Changing any narration text.** M1 freezes; M5 improves.
- **Filling the gaps the audit finds.** Naming them is the deliverable; writing
  the missing lines is M2 or later, with the core in place.
- **Group A — refusals, admin output, status tables.** Its own arc, by owner
  decision. ⚠️ That arc must not select its work by `messaging.CategorySystem`,
  or it will drag ~247 narration sites in with it.
- **Message ORDER and interleaving within a round.** Snapshots cannot see it. It
  stays a playtest question, which is why M5 ends at the adversarial gate.
- **Retiring `seedNum` / `indexOverride`.** They fold in once the core exists.
