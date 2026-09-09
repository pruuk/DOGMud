# Messaging M3.1 + M3.2: Store Core Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract one rendering core into `internal/narration` that picks a single coordinated variant index across roles, and migrate the defence, itemvoices and casting stores onto it without changing a single line a player reads.

**Architecture:** The core owns exactly three things: coordinate one index across roles, substitute tokens, validate a pool set. It owns nothing else. Each store keeps its own YAML struct, loader, band computation and pool assembly, and hands the core a finished `Variants`. This "assembly rule" is what keeps skill tiers (a pool union), grapple cooldowns (a pool filter) and defence banding out of the core, and it is what leaves M4 a parameter flip rather than a core rewrite.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v2`, the repo's `fileloader` + `narration.Picker` seams, golden-file snapshot tests under `internal/narration/testdata/stores/`.

**Spec:** [`docs/superpowers/specs/2026-09-09-messaging-m3-store-core-extraction-design.md`](../specs/2026-09-09-messaging-m3-store-core-extraction-design.md)

---

## 🔴 Read this before Task 1

Three rules govern this whole plan. Breaking any one of them silently destroys the evidence that the refactor was safe.

**1. The goldens must come out BYTE-IDENTICAL. Never run `-update` to make a red test green.**
`defense_messages.golden` keys its rows by the **authored** role name (`block|weak|todefender => ...`). That is the only thing in the repo that can catch the mistake this refactor makes easy: swapping which authored pool lands in which role. Re-recording the golden under new role names would bake a swap into the baseline invisibly. If a golden goes red, the migration is wrong. Fix the migration.

**2. `pick(n)` is called EVEN WHEN an index override is supplied.**
`DefenseOptions.RenderTriad` (`internal/items/defensive_messages.go:133`) calls `index := pick(n)` first and only then overwrites `index` from `indexOverride`. The draw is discarded, not skipped. `narration.DefaultPicker` routes through `util.Rand`, which is global engine randomness, so "optimising" this into an early return consumes one fewer random number and shifts every subsequent draw in the process. Preserve it exactly.

**3. `-update` rewrites `defense_messages.golden` line endings.**
`.gitattributes` covers `*.go`, `go.mod` and `go.sum` but no golden, so that file is checked out CRLF on Windows and `-update` writes it LF. It then shows as modified while diffing to nothing. Task 0b fixes this properly; until it lands, `git restore` that file rather than committing the churn.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/narration/render.go` | **New.** The entire core: `Selector`, `Variants`, `Roles`, `Render`, `ValidateVariants`. Nothing else goes here. |
| `internal/narration/render_test.go` | **New.** Unit tests for the core, including the coordinated-index probe. |
| `internal/narration/context.md` | **Modify.** Document the new public surface. |
| `internal/items/defensive_messages.go` | **Modify.** `RenderTriad` becomes a thin adapter onto `narration.Render`. Public signatures unchanged. |
| `internal/itemvoices/itemvoices.go` | **Modify.** Gains a picker seam (Task 1), then migrates (Task 4). |
| `internal/itemvoices/context.md` | **Modify.** Document the picker seam. |
| `internal/spells/casting_messages.go` | **Modify.** Validator, boot-fail, fallback deleted, migrate onto the core. |
| `internal/narration/snapshot_test.go` | **Modify.** Add an itemvoices golden builder. |
| `internal/narration/testdata/stores/itemvoices.golden` | **New.** Built from PRE-migration code in Task 1. |
| `contest_floor_guard_test.go`, `durable_write_guard_test.go` | **Modify.** Skip dot-directories (Task 0a). |
| `.gitattributes` | **Modify.** Pin `*.golden` to LF (Task 0b). |

---

## Task 0a: Stop the root guards walking into agent worktrees

Three root guard tests walk from the repo root and skip no dot-directories, so a git worktree under `.claude/worktrees/` presents a second full copy of `internal/` and they report its files as violations. `go test ./...` is unreliable while any agent worktree exists, which now happens routinely. **Do this first or you cannot trust any later test run.**

**Files:**
- Modify: `contest_floor_guard_test.go:169`
- Modify: `durable_write_guard_test.go:106` and `durable_write_guard_test.go:186`

- [ ] **Step 1: Confirm the bug is real before fixing it**

```bash
git worktree add --detach C:/tmp/guard-probe HEAD
go test . -run 'TestOpposedContestsAreFloored|TestLivingStateWritesAreDurable|TestNoHandRolledTempRename' 2>&1 | tail -20
```

Expected: FAIL. The offender list names paths containing `worktrees`. If it passes, the worktree is not inside the repo; put it under `.claude/worktrees/guard-probe` instead and retry.

- [ ] **Step 2: Apply the fix at all three sites**

All three sites are **byte-identical**, so edit them by line number and verify each individually. In each of `contest_floor_guard_test.go:169`, `durable_write_guard_test.go:106`, `durable_write_guard_test.go:186`, replace:

```go
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin", "_datafiles", "docs", "tools":
				return filepath.SkipDir
			}
			return nil
```

with:

```go
		if d.IsDir() {
			// Skip every dot-directory, not just .git. A git worktree under
			// .claude/worktrees/ presents a SECOND full copy of internal/, and
			// this walk would then report that copy's files as violations of a
			// rule the real tree does not break. Agent worktrees are routine.
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "vendor", "node_modules", "bin", "_datafiles", "docs", "tools":
				return filepath.SkipDir
			}
			return nil
```

- [ ] **Step 3: Add the `strings` import where missing**

```bash
head -20 contest_floor_guard_test.go durable_write_guard_test.go | grep -n strings
```

If either file lacks `"strings"` in its import block, add it.

- [ ] **Step 4: Verify the fix, then verify the probe still could have failed**

```bash
go test . -run 'TestOpposedContestsAreFloored|TestLivingStateWritesAreDurable|TestNoHandRolledTempRename' -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all three PASS **with the worktree still present**. Then remove it:

```bash
git worktree remove --force C:/tmp/guard-probe || (powershell -Command "Remove-Item -Recurse -Force C:\tmp\guard-probe"; git worktree prune)
```

- [ ] **Step 5: Commit**

```bash
git add contest_floor_guard_test.go durable_write_guard_test.go
git commit -m "test: stop the root guards walking into agent worktrees

All three walk from the repo root and skipped only .git, so a worktree
under .claude/worktrees/ presented a second copy of internal/ and they
reported its files as violations. go test ./... was unreliable whenever
an agent worktree existed, which is now routine."
```

---

## Task 0b: Pin goldens to LF

**Files:**
- Modify: `.gitattributes`

- [ ] **Step 1: Reproduce the pre-existing Windows failure**

```bash
go test ./internal/narration/ -run 'TestSnapshotStores/defense_messages' 2>&1 | tail -6
```

Expected on Windows: FAIL, with `want:` and `got:` lines that look **identical**. That visual identity is the tell: the difference is CRLF versus LF, not content. On Linux/CI this passes, which is why it survived.

- [ ] **Step 2: Add the rule**

Append to `.gitattributes`:

```
# Golden snapshot files are written by `go test -update` with LF, and compared
# byte-for-byte. Without this, a Windows checkout converts them to CRLF and
# TestSnapshotStores fails locally while passing in CI, which trains people to
# ignore a red golden. That is the one test in the repo that must never be
# ignored: it is the net under the M3 store migration.
*.golden text eol=lf
```

- [ ] **Step 3: Renormalise the already-committed goldens**

```bash
git add --renormalize internal/narration/testdata/stores/
git status --short internal/narration/testdata/stores/
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/narration/ -run TestSnapshotStores 2>&1 | tail -5
```

Expected: `ok`. All six goldens pass on Windows now.

- [ ] **Step 5: Commit**

```bash
git add .gitattributes internal/narration/testdata/stores/
git commit -m "test: pin goldens to LF so TestSnapshotStores passes on Windows

.gitattributes covered *.go, go.mod and go.sum but no golden, so
defense_messages.golden was checked out CRLF and compared against
LF-written output. It failed on Windows and passed in CI, which trains
people to ignore a red golden right before M3 makes that golden the net
under a nine-store migration."
```

---

## Task 1: Give itemvoices a picker seam and a golden, BEFORE migrating it

`VoiceSpec.Line` calls `util.Rand` directly (`internal/itemvoices/itemvoices.go:73`), so it cannot be snapshotted, and `testdata/stores/` holds six goldens, none of them itemvoices. **A golden written after the migration proves nothing.** Build the net from pre-migration code.

**Files:**
- Modify: `internal/itemvoices/itemvoices.go:64-74`
- Modify: `internal/narration/snapshot_test.go`
- Create: `internal/narration/testdata/stores/itemvoices.golden`

- [ ] **Step 1: Write the failing test for the seam**

Create `internal/itemvoices/picker_test.go`:

```go
package itemvoices

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// TestLineWithHonoursThePicker is what makes this store snapshottable at all.
// Before it, Line() called util.Rand directly and no test could pin its output.
func TestLineWithHonoursThePicker(t *testing.T) {
	v := &VoiceSpec{
		VoiceId: "test-voice",
		Lines:   map[string][]string{"on_equip": {"first", "second", "third"}},
	}

	if got := v.LineWith(narration.SequencePicker(), "on_equip"); got != "first" {
		t.Errorf("fresh SequencePicker should yield index 0, got %q", got)
	}

	always2 := func(n int) int { return 2 }
	if got := v.LineWith(always2, "on_equip"); got != "third" {
		t.Errorf("picker index 2 should yield the third variant, got %q", got)
	}
}

// TestLineWithNilPickerIsProduction pins the contract every other store in the
// repo uses: a nil Picker means narration.DefaultPicker, never a panic.
func TestLineWithNilPickerIsProduction(t *testing.T) {
	v := &VoiceSpec{
		VoiceId: "test-voice",
		Lines:   map[string][]string{"on_equip": {"only"}},
	}
	if got := v.LineWith(nil, "on_equip"); got != "only" {
		t.Errorf("nil picker should still render, got %q", got)
	}
}

// TestLineWithUnknownEventIsEmpty pins the existing contract Line() has.
func TestLineWithUnknownEventIsEmpty(t *testing.T) {
	v := &VoiceSpec{VoiceId: "test-voice", Lines: map[string][]string{}}
	if got := v.LineWith(narration.SequencePicker(), "on_equip"); got != "" {
		t.Errorf("unknown event should render empty, got %q", got)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/itemvoices/ -run TestLineWith 2>&1 | head -5
```

Expected: FAIL to build, `v.LineWith undefined`.

- [ ] **Step 3: Add the seam**

In `internal/itemvoices/itemvoices.go`, replace the `Line` method (currently at `:64-74`):

```go
// Line returns a random line from the event's pool, or "" if the event has
// no authored lines (unknown event or empty pool).
func (v *VoiceSpec) Line(event string) string {
	pool := v.Lines[event]
	if len(pool) == 0 {
		return ""
	}
	return pool[util.Rand(len(pool))]
}
```

with:

```go
// Line returns a random line from the event's pool, or "" if the event has
// no authored lines (unknown event or empty pool).
func (v *VoiceSpec) Line(event string) string {
	return v.LineWith(nil, event)
}

// LineWith is Line with an explicit picker, for the snapshot harness. A nil
// picker means production behaviour: narration.DefaultPicker, which routes
// through util.Rand, the engine's single randomness seam.
//
// This seam exists because this store had none: Line() called util.Rand
// directly, so nothing could pin its output and it was the one message store
// with no golden. It was added BEFORE the M3 migration so the golden it
// enables is a baseline of pre-migration behaviour rather than a record of
// whatever the migration happened to produce.
func (v *VoiceSpec) LineWith(pick narration.Picker, event string) string {
	pool := v.Lines[event]
	if len(pool) == 0 {
		return ""
	}
	if pick == nil {
		pick = narration.DefaultPicker
	}
	return pool[pick(len(pool))]
}
```

Add `"github.com/GoMudEngine/GoMud/internal/narration"` to the import block. Leave `util` imported: it is still used by `Filepath()`.

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/itemvoices/ 2>&1 | tail -3
```

Expected: `ok`.

- [ ] **Step 5: Add the itemvoices golden builder**

In `internal/narration/snapshot_test.go`, add this function next to the other store builders:

```go
// ---------------------------------------------------------------------
// Store 6: itemvoices (internal/itemvoices)
// ---------------------------------------------------------------------

func buildItemVoicesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "itemvoices")
	files := yamlKeysInDir(t, dir)

	var b strings.Builder
	fmt.Fprintf(&b, "# itemvoices store snapshot\n")
	fmt.Fprintf(&b, "# voice files at time of writing: %d (%s)\n", len(files), strings.Join(files, ","))
	fmt.Fprintf(&b, "# dimensions: voiceid x event. Single role (the item speaks), no band, no tokens.\n")
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# This store had NO picker seam and NO golden until 2026-09-09: VoiceSpec.Line\n")
	fmt.Fprintf(&b, "# called util.Rand directly. LineWith was added FIRST, so this golden is a\n")
	fmt.Fprintf(&b, "# baseline of PRE-migration behaviour, not a record of the migration's output.\n")
	fmt.Fprintf(&b, "# A fresh SequencePicker per tuple pins index 0.\n\n")

	events := []string{
		"on_equip", "on_unequip", "on_kill", "on_idle",
		"on_hunger_warning", "on_hunger_feeding", "on_taunt", "on_grudge",
	}

	for _, id := range itemvoices.AllVoiceIds() {
		v := itemvoices.GetVoice(id)
		if v == nil {
			t.Fatalf("voice %q loaded in AllVoiceIds but GetVoice returned nil", id)
		}
		for _, event := range events {
			fmt.Fprintf(&b, "%s|%s => %q\n", id, event, v.LineWith(narration.SequencePicker(), event))
		}
	}

	// EMPTY CASE: an event that is not in the valid set at all.
	fmt.Fprintf(&b, "\n# EMPTY CASE: unknown event -> \"\"\n")
	if ids := itemvoices.AllVoiceIds(); len(ids) > 0 {
		v := itemvoices.GetVoice(ids[0])
		fmt.Fprintf(&b, "%s|bogus-event => %q\n", ids[0], v.LineWith(narration.SequencePicker(), "bogus-event"))
	}

	return b.String()
}
```

Add `"github.com/GoMudEngine/GoMud/internal/itemvoices"` to the import block. In `setupRealStores`, add the loader call after `combat.LoadTauntMessageFiles()`:

```go
	itemvoices.LoadDataFiles()
```

And register the golden inside `TestSnapshotStores`, alongside the existing `checkGolden` calls:

```go
	t.Run("itemvoices", func(t *testing.T) {
		checkGolden(t, "itemvoices.golden", buildItemVoicesGolden(t))
	})
```

- [ ] **Step 6: Record the baseline and READ IT**

```bash
go test ./internal/narration/ -run TestSnapshotStores -update
cat internal/narration/testdata/stores/itemvoices.golden
```

Expected: real authored voice lines, not a wall of `""`. **If every value is empty the loader did not run** and the golden is worthless as a baseline. Fix `setupRealStores` before continuing.

- [ ] **Step 7: Verify it is now a real gate**

```bash
go test ./internal/narration/ -run TestSnapshotStores 2>&1 | tail -3
```

Expected: `ok`.

- [ ] **Step 8: Commit**

```bash
git add internal/itemvoices/itemvoices.go internal/itemvoices/picker_test.go \
        internal/narration/snapshot_test.go \
        internal/narration/testdata/stores/itemvoices.golden
git commit -m "test(itemvoices): add the picker seam and the golden it enables

VoiceSpec.Line called util.Rand directly, so this was the one message
store nothing could pin, and testdata/stores held six goldens with none
of them itemvoices. Both are added BEFORE the M3 migration touches the
store, so the golden is a baseline of pre-migration behaviour rather
than a record of whatever the migration produced.

Third slice running where the previous slice's net does not reach the
next slice's target. Assume it next time rather than rediscovering it."
```

---

## Task 2: The core

**Files:**
- Create: `internal/narration/render.go`
- Create: `internal/narration/render_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/narration/render_test.go`:

```go
package narration

import (
	"strings"
	"testing"
)

// TestRenderCoordinatesOneIndexAcrossRoles is THE test. The whole reason this
// core exists is that variant N of every role describes the same moment, so a
// renderer that picks per role narrates three different events to three people.
//
// The probe is a SINGLE SHARED SequencePicker: one pick yields A0/B0/C0, four
// picks yield A0/B1/C2/D3. The two outcomes are distinguishable by
// construction, which is what makes this capable of failing.
func TestRenderCoordinatesOneIndexAcrossRoles(t *testing.T) {
	v := Variants{
		Actor:         []string{"a0", "a1", "a2"},
		Actee:         []string{"b0", "b1", "b2"},
		Observer:      []string{"c0", "c1", "c2"},
		ActeeObserver: []string{"d0", "d1", "d2"},
	}

	got := Render(v, nil, SequencePicker())

	want := Roles{Actor: "a0", Actee: "b0", Observer: "c0", ActeeObserver: "d0"}
	if got != want {
		t.Fatalf("roles came from different indices:\n got %+v\nwant %+v\n(per-role picking would give a0/b1/c2/d3)", got, want)
	}
}

// TestRenderHonoursANonZeroIndex guards the case index 0 cannot see: a
// renderer that ignored the picker entirely would pass the test above.
func TestRenderHonoursANonZeroIndex(t *testing.T) {
	v := Variants{
		Actor:    []string{"a0", "a1", "a2"},
		Observer: []string{"c0", "c1", "c2"},
	}
	always2 := func(n int) int { return 2 }

	got := Render(v, nil, always2)

	if got.Actor != "a2" || got.Observer != "c2" {
		t.Fatalf("picker index not applied to every role: %+v", got)
	}
}

// TestRenderSkipsEmptyRoles is the Kind B case: buffs, spells, quests and
// crafting hold a single string and have no actee, so most roles are absent.
func TestRenderSkipsEmptyRoles(t *testing.T) {
	v := Variants{Actor: []string{"only the actor"}}

	got := Render(v, nil, SequencePicker())

	if got.Actor != "only the actor" {
		t.Errorf("actor = %q", got.Actor)
	}
	if got.Actee != "" || got.Observer != "" || got.ActeeObserver != "" {
		t.Errorf("absent roles must render empty, got %+v", got)
	}
}

// TestRenderRejectsUnequalRoles: unequal pools cannot be coordinated, because
// index N would name a line in one role and nothing in another. Returning a
// zero Roles matches DefenseOptions.RenderTriad's existing behaviour.
func TestRenderRejectsUnequalRoles(t *testing.T) {
	v := Variants{
		Actor:    []string{"a0", "a1", "a2"},
		Observer: []string{"c0", "c1"},
	}

	if got := Render(v, nil, SequencePicker()); got != (Roles{}) {
		t.Fatalf("unequal role pools must render a zero Roles, got %+v", got)
	}
}

// TestRenderIndexOverrideStillConsumesAPick is bug-compatibility with
// DefenseOptions.RenderTriad, and it is NOT a detail.
//
// That function calls pick(n) FIRST and only then overwrites the index from
// indexOverride, so the draw is discarded rather than skipped. DefaultPicker
// routes through util.Rand, which is global engine randomness, so skipping the
// call would consume one fewer random number and shift every subsequent draw
// in the process.
func TestRenderIndexOverrideStillConsumesAPick(t *testing.T) {
	v := Variants{Actor: []string{"a0", "a1", "a2"}}

	calls := 0
	counting := func(n int) int { calls++; return 0 }

	got := Render(v, nil, counting, 2)

	if got.Actor != "a2" {
		t.Errorf("override should select index 2, got %q", got.Actor)
	}
	if calls != 1 {
		t.Errorf("pick must still be called exactly once when overridden, called %d times", calls)
	}
}

// TestRenderOverrideWraps pins RenderTriad's existing modulo behaviour,
// including its handling of a negative override.
func TestRenderOverrideWraps(t *testing.T) {
	v := Variants{Actor: []string{"a0", "a1", "a2"}}

	if got := Render(v, nil, SequencePicker(), 4); got.Actor != "a1" {
		t.Errorf("override 4 of 3 should wrap to index 1, got %q", got.Actor)
	}
	if got := Render(v, nil, SequencePicker(), -1); got.Actor != "a2" {
		t.Errorf("override -1 of 3 should wrap to index 2, got %q", got.Actor)
	}
}

// TestRenderSubstitutesTokensInEveryRole guards a role rendered without its
// token pass.
func TestRenderSubstitutesTokensInEveryRole(t *testing.T) {
	v := Variants{
		Actor:    []string{"you hit {target}"},
		Actee:    []string{"{source} hits you"},
		Observer: []string{"{source} hits {target}"},
	}

	got := Render(v, map[string]string{"{source}": "Alice", "{target}": "Bob"}, SequencePicker())

	for role, text := range map[string]string{"actor": got.Actor, "actee": got.Actee, "observer": got.Observer} {
		if strings.Contains(text, "{") {
			t.Errorf("%s has an unsubstituted token: %q", role, text)
		}
	}
	if got.Observer != "Alice hits Bob" {
		t.Errorf("observer = %q", got.Observer)
	}
}

// TestRenderTokenSubstitutionIsSinglePass: a token VALUE that happens to
// contain a token spelling must not be substituted again. A player name is
// attacker-controlled in principle, so this is the safe behaviour.
func TestRenderTokenSubstitutionIsSinglePass(t *testing.T) {
	v := Variants{Actor: []string{"{source} waves"}}

	got := Render(v, map[string]string{"{source}": "{target}", "{target}": "Bob"}, SequencePicker())

	if got.Actor != "{target} waves" {
		t.Errorf("substitution must be single pass, got %q", got.Actor)
	}
}

func TestValidateVariants(t *testing.T) {
	ok := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c", "d", "e"},
	}
	if err := ValidateVariants(ok, 5); err != nil {
		t.Fatalf("equal-length pools should validate: %v", err)
	}

	unequal := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c"},
	}
	if err := ValidateVariants(unequal, 5); err == nil {
		t.Error("unequal role pools must not validate")
	}

	short := Variants{Actor: []string{"a", "b"}}
	if err := ValidateVariants(short, 5); err == nil {
		t.Error("pools below the minimum must not validate")
	}

	blank := Variants{Actor: []string{"a", "   ", "c", "d", "e"}}
	if err := ValidateVariants(blank, 5); err == nil {
		t.Error("a whitespace-only variant must not validate")
	}

	if err := ValidateVariants(Variants{}, 0); err == nil {
		t.Error("a Variants with no roles at all must not validate")
	}
}
```

- [ ] **Step 2: Run and watch it fail**

```bash
go test ./internal/narration/ -run 'TestRender|TestValidateVariants' 2>&1 | head -5
```

Expected: FAIL to build, `undefined: Variants`.

- [ ] **Step 3: Write the core**

Create `internal/narration/render.go`:

```go
package narration

import (
	"fmt"
	"sort"
	"strings"
)

// Selector names which variant group an event draws from.
//
// It is an OPAQUE STRING on purpose. Whether the groups are ordered (defence's
// weak/normal/heavy) or merely named (itemvoices' on_equip, casting's
// cast_started) is a property of the function that COMPUTES the selector, not
// of the pool it selects. Keeping that distinction outside the core is what
// leaves M4 a parameter flip instead of a core rewrite.
type Selector string

// Variants holds one candidate list per role. Any role may be empty.
//
// Every non-empty role must hold the SAME number of variants, because variant
// N of each role describes the same moment. ValidateVariants enforces that at
// load time; Render refuses to guess at render time.
type Variants struct {
	Actor         []string
	Actee         []string
	Observer      []string
	ActeeObserver []string
}

// Roles is one narrated event as each audience is told it.
//
// The names match messaging.Trio deliberately: rendering-side roles and
// delivery-side roles must not drift apart.
//
// ActeeObserver is observers where the ACTEE is, and is empty whenever the
// participants share a room. It exists for combat-messages' `separate` case,
// where a ranged attacker and defender are not co-located and there are
// genuinely two observer audiences.
type Roles struct {
	Actor         string
	Actee         string
	Observer      string
	ActeeObserver string
}

// roleLists returns the four pools in a fixed order for iteration.
func (v Variants) roleLists() [][]string {
	return [][]string{v.Actor, v.Actee, v.Observer, v.ActeeObserver}
}

// Len is the coordinated variant count, or 0 if the non-empty roles disagree
// or there are none. Zero means "cannot be coordinated", and every caller
// treats it as "render nothing".
func (v Variants) Len() int {
	n := 0
	for _, pool := range v.roleLists() {
		if len(pool) == 0 {
			continue
		}
		if n == 0 {
			n = len(pool)
			continue
		}
		if len(pool) != n {
			return 0
		}
	}
	return n
}

// Render picks ONE index and applies it to every non-empty role, then
// substitutes tokens.
//
// ONE INDEX FOR ALL ROLES IS THE ENTIRE POINT. Picking per role produces a
// coherent-looking line for each audience that describes a different moment,
// which is the defect this core exists to make unrepresentable. It shipped
// twice in this codebase before the core existed: melee defence (PR #112) and
// taunt (PR #115).
//
// A nil picker means production behaviour, DefaultPicker.
//
// ⚠️ indexOverride does NOT skip the pick. pick(n) is called first and the
// draw is then discarded. This is bug-compatibility with
// items.DefenseOptions.RenderTriad, and it matters: DefaultPicker routes
// through util.Rand, which is global engine randomness, so returning early
// would consume one fewer random number and shift every subsequent draw in the
// process.
func Render(v Variants, tokens map[string]string, pick Picker, indexOverride ...int) Roles {
	n := v.Len()
	if n == 0 {
		return Roles{}
	}
	if pick == nil {
		pick = DefaultPicker
	}

	index := pick(n)
	if len(indexOverride) > 0 {
		index = indexOverride[0] % n
		if index < 0 {
			index += n
		}
	}

	at := func(pool []string) string {
		if len(pool) == 0 {
			return ""
		}
		return substitute(pool[index], tokens)
	}

	return Roles{
		Actor:         at(v.Actor),
		Actee:         at(v.Actee),
		Observer:      at(v.Observer),
		ActeeObserver: at(v.ActeeObserver),
	}
}

// substitute replaces every token in one pass.
//
// One Replacer rather than sequential replacements, so a value that happens to
// contain a token spelling cannot be substituted again by a later pass. The
// keys are sorted longest-first so the result never depends on Go's randomised
// map iteration order, which would otherwise make output nondeterministic if
// one token spelling were ever a prefix of another.
func substitute(s string, tokens map[string]string) string {
	if len(tokens) == 0 {
		return s
	}
	keys := make([]string, 0, len(tokens))
	for k := range tokens {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	pairs := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		pairs = append(pairs, k, tokens[k])
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// ValidateVariants checks that a pool set can be coordinated: every non-empty
// role holds the same count, that count is at least minVariants, and no
// variant is blank.
//
// The minimum is a PARAMETER because the stores genuinely differ. Defence
// demands 5 and ships 10 to 14; casting ships 3. Applying defence's number to
// casting would fail boot on shipped data without improving a single line of
// text. M4 is where these unify, if they should.
func ValidateVariants(v Variants, minVariants int) error {
	named := []struct {
		name string
		pool []string
	}{
		{"actor", v.Actor},
		{"actee", v.Actee},
		{"observer", v.Observer},
		{"acteeObserver", v.ActeeObserver},
	}

	n := 0
	for _, role := range named {
		if len(role.pool) == 0 {
			continue
		}
		if n == 0 {
			n = len(role.pool)
		} else if len(role.pool) != n {
			return fmt.Errorf("role %q holds %d variants but a sibling role holds %d; variant N of each role must describe the SAME moment", role.name, len(role.pool), n)
		}
		for i, text := range role.pool {
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("role %q variant %d is empty", role.name, i)
			}
		}
	}

	if n == 0 {
		return fmt.Errorf("no role holds any variants")
	}
	if n < minVariants {
		return fmt.Errorf("every role holds %d variants, need at least %d", n, minVariants)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/narration/ -run 'TestRender|TestValidateVariants' -v 2>&1 | grep -E "^(--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all PASS.

- [ ] **Step 5: Prove the coordination probe can fail**

This is required. A null probe that has never been seen red is not evidence.

Temporarily change `at` inside `Render` to pick per role:

```go
	at := func(pool []string) string {
		if len(pool) == 0 {
			return ""
		}
		return substitute(pool[pick(len(pool))], tokens)
	}
	_ = index
```

Then:

```bash
go vet ./internal/narration/ && echo "SABOTAGE COMPILES"
go test ./internal/narration/ -run TestRenderCoordinatesOneIndexAcrossRoles 2>&1 | head -5
```

Expected: `SABOTAGE COMPILES` printed **and** the test FAILS naming differing indices. A sabotage that does not compile proves nothing; if `go vet` complains, fix the sabotage until it compiles, then re-run. Revert it afterwards and confirm green.

- [ ] **Step 6: Commit**

```bash
git add internal/narration/render.go internal/narration/render_test.go
git commit -m "feat(narration): one core that coordinates a variant index across roles

Variant N of every role describes the same moment, so a renderer that
picks per role narrates a different event to each audience. That defect
shipped twice before this core existed: melee defence (#112) and taunt
(#115). Render makes it unrepresentable.

The core owns exactly three things: coordinate one index, substitute
tokens, validate a pool set. Bands, skill-tier unions and cooldown
filters stay in the stores, which is what leaves M4 a parameter flip.

Bug-compatible with DefenseOptions.RenderTriad including the detail that
an index override still consumes a pick, because DefaultPicker routes
through global util.Rand and skipping the draw would shift every
subsequent one."
```

---

## Task 3: Migrate defence onto the core

**Files:**
- Modify: `internal/items/defensive_messages.go:133-165` (`RenderTriad`)

The public surface does not change. `RenderDefenseMessage`, `GetDefenseMessage` and `DefenseMessageTriad` keep their signatures, so no caller outside `internal/items` is touched.

- [ ] **Step 1: Record the pre-migration golden hash**

```bash
git hash-object internal/narration/testdata/stores/defense_messages.golden
```

Write the hash down. It must be identical after the migration.

- [ ] **Step 2: Rewrite RenderTriad as an adapter**

Replace the body of `DefenseOptions.RenderTriad` (`internal/items/defensive_messages.go:133`), keeping the signature exactly as it is:

```go
// RenderTriad renders one coordinated defender/attacker/room triad from an
// already-selected band. ALL THREE ROLES COME FROM THE SAME VARIANT INDEX:
// the authored pools pair up by index, so picking per role produces three
// descriptions of three different events.
//
// A nil picker means production behaviour (narration.DefaultPicker).
//
// The coordination itself now lives in narration.Render. This function is the
// adapter: it maps the AUTHORED role names onto the core's role vocabulary,
// and that mapping is the one line in this file most worth reading carefully.
// An attacker ACTS and a defender is ACTED UPON, so toattacker is the Actor
// and todefender is the Actee. Swapping them would invert every defence
// message in the game and is exactly what defense_messages.golden exists to
// catch, because that file keys its rows by the authored name.
func (o DefenseOptions) RenderTriad(tokenReplacements map[TokenName]string, pick narration.Picker, indexOverride ...int) DefenseMessageTriad {
	roles := narration.Render(
		narration.Variants{
			Actor:    messageStrings(o.Together.ToAttacker),
			Actee:    messageStrings(o.Together.ToDefender),
			Observer: messageStrings(o.Together.ToRoom),
		},
		tokenStrings(tokenReplacements),
		pick,
		indexOverride...,
	)

	return DefenseMessageTriad{
		ToAttacker: ItemMessage(roles.Actor),
		ToDefender: ItemMessage(roles.Actee),
		ToRoom:     ItemMessage(roles.Observer),
	}
}

// messageStrings converts an authored pool to the core's plain-string form.
func messageStrings(pool MessageOptions) []string {
	if len(pool) == 0 {
		return nil
	}
	out := make([]string, len(pool))
	for i, m := range pool {
		out[i] = string(m)
	}
	return out
}

// tokenStrings converts a token map to the core's plain-string form.
func tokenStrings(tokens map[TokenName]string) map[string]string {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]string, len(tokens))
	for name, value := range tokens {
		out[string(name)] = value
	}
	return out
}
```

- [ ] **Step 3: Verify the goldens are byte-identical**

```bash
go test ./internal/narration/ -run TestSnapshotStores 2>&1 | tail -5
git status --short internal/narration/testdata/stores/
git hash-object internal/narration/testdata/stores/defense_messages.golden
```

Expected: `ok`, **no modified goldens**, and the same hash as Step 1. A red golden here means the migration is wrong. **Do not run `-update`.**

- [ ] **Step 4: Run the items and combat tests**

```bash
go test ./internal/items/ ./internal/combat/ 2>&1 | tail -4
```

Expected: `ok` for both.

- [ ] **Step 5: Prove the golden can catch a role swap**

Temporarily swap `Actor` and `Actee` in the `narration.Variants` literal:

```go
			Actor:    messageStrings(o.Together.ToDefender),
			Actee:    messageStrings(o.Together.ToAttacker),
```

```bash
go vet ./internal/items/ && echo "SABOTAGE COMPILES"
go test ./internal/narration/ -run 'TestSnapshotStores/defense_messages' 2>&1 | head -8
```

Expected: `SABOTAGE COMPILES` **and** a golden mismatch whose first differing line is a `todefender` row now holding the attacker's sentence. Revert and re-verify green. **This is the single most important verification in the plan**: it proves the net actually covers the mistake the refactor makes easy.

- [ ] **Step 6: Commit**

```bash
git add internal/items/defensive_messages.go
git commit -m "refactor(items): move defence narration onto the narration core

RenderTriad becomes an adapter. The coordination, token substitution and
index-override semantics move to narration.Render; what stays here is the
mapping from authored role names onto the core's vocabulary, which is the
part worth reading: toattacker is the Actor because an attacker acts, and
todefender is the Actee because a defender is acted upon.

defense_messages.golden is byte-identical, which is the proof this
changed no behaviour. Verified that swapping Actor and Actee in the
adapter turns that golden red, so the net covers the mistake this
refactor makes easiest."
```

---

## Task 4: Migrate itemvoices onto the core

**Files:**
- Modify: `internal/itemvoices/itemvoices.go` (`LineWith`)

- [ ] **Step 1: Record the golden hash**

```bash
git hash-object internal/narration/testdata/stores/itemvoices.golden
```

- [ ] **Step 2: Rewrite LineWith onto the core**

```go
// LineWith is Line with an explicit picker, for the snapshot harness. A nil
// picker means production behaviour: narration.DefaultPicker, which routes
// through util.Rand, the engine's single randomness seam.
//
// This is the DEGENERATE case for the narration core and it is worth stating
// plainly: an item voice has ONE role (the item speaks), no band, and no
// tokens. It renders through the same seam as the defence triad, which is the
// evidence that roles and bands are genuinely optional there rather than
// something a single-role store has to work around.
func (v *VoiceSpec) LineWith(pick narration.Picker, event string) string {
	return narration.Render(
		narration.Variants{Actor: v.Lines[event]},
		nil,
		pick,
	).Actor
}
```

- [ ] **Step 3: Verify byte-identical and green**

```bash
go test ./internal/itemvoices/ ./internal/narration/ 2>&1 | tail -4
git status --short internal/narration/testdata/stores/
git hash-object internal/narration/testdata/stores/itemvoices.golden
```

Expected: `ok`, no modified goldens, same hash as Step 1.

- [ ] **Step 4: Commit**

```bash
git add internal/itemvoices/itemvoices.go
git commit -m "refactor(itemvoices): render item voices through the narration core

The degenerate case: one role, no band, no tokens, rendered through the
same seam as the defence triad. That is the evidence roles and bands are
genuinely optional in the core rather than something a single-role store
has to work around. Golden byte-identical."
```

---

## Task 5: Migrate casting, add its validator, delete its Go fallback

Casting is not in `fileloader` (bare `os.ReadFile` under a `sync.Once`), so it has never had a validator, and it carries a hardcoded Go fallback that silently substitutes unauthored text. Owner rulings: **minimum 3**, **boot-fail by panic**, **fallback deleted**.

**Files:**
- Modify: `internal/spells/casting_messages.go`
- Create: `internal/spells/casting_messages_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/spells/casting_messages_test.go`:

```go
package spells

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

func TestValidateCastingMessagesAcceptsShippedShape(t *testing.T) {
	cm := &CastingMessages{
		AlreadyCasting:       []string{"a", "b", "c"},
		CastStarted:          []string{"a", "b", "c"},
		CastContinuing:       []string{"a", "b", "c", "d"},
		ConcentrationSlipped: []string{"a", "b", "c"},
	}
	if err := cm.Validate(); err != nil {
		t.Fatalf("the shipped shape (3/3/3/4) must validate: %v", err)
	}
}

func TestValidateCastingMessagesRejectsShortPool(t *testing.T) {
	cm := &CastingMessages{
		AlreadyCasting:       []string{"a", "b"},
		CastStarted:          []string{"a", "b", "c"},
		CastContinuing:       []string{"a", "b", "c"},
		ConcentrationSlipped: []string{"a", "b", "c"},
	}
	err := cm.Validate()
	if err == nil {
		t.Fatal("a pool below the minimum must not validate")
	}
	if !strings.Contains(err.Error(), "already_casting") {
		t.Errorf("error should name the offending pool, got %v", err)
	}
}

func TestValidateCastingMessagesRejectsEmptyPool(t *testing.T) {
	cm := &CastingMessages{
		AlreadyCasting:       []string{"a", "b", "c"},
		CastStarted:          nil,
		CastContinuing:       []string{"a", "b", "c"},
		ConcentrationSlipped: []string{"a", "b", "c"},
	}
	if err := cm.Validate(); err == nil {
		t.Fatal("a missing pool must not validate")
	}
}

func TestGetCastMessageRendersThroughTheCore(t *testing.T) {
	prev := castingMessages
	castingMessages = &CastingMessages{
		AlreadyCasting:       []string{"already {spell}"},
		CastStarted:          []string{"started {spell} 0", "started {spell} 1"},
		CastContinuing:       []string{"continuing {spell}"},
		ConcentrationSlipped: []string{"slipped {spell}"},
	}
	castingMessagesLoaded = true
	t.Cleanup(func() { castingMessages = prev })

	if got := GetCastMessage("cast_started", "Firebolt", narration.SequencePicker()); got != "started Firebolt 0" {
		t.Errorf("cast_started = %q", got)
	}
	always1 := func(n int) int { return 1 }
	if got := GetCastMessage("cast_started", "Firebolt", always1); got != "started Firebolt 1" {
		t.Errorf("cast_started at index 1 = %q", got)
	}
}

func TestGetCastMessageUnknownCategoryFallsBack(t *testing.T) {
	prev := castingMessages
	castingMessages = &CastingMessages{
		AlreadyCasting:       []string{"a"},
		CastStarted:          []string{"b"},
		CastContinuing:       []string{"c"},
		ConcentrationSlipped: []string{"d"},
	}
	castingMessagesLoaded = true
	t.Cleanup(func() { castingMessages = prev })

	got := GetCastMessage("bogus-category", "Firebolt", narration.SequencePicker())
	if got != "Something stirs with Firebolt." {
		t.Errorf("unknown category should keep its literal fallback sentence, got %q", got)
	}
}
```

- [ ] **Step 2: Run and watch it fail**

```bash
go test ./internal/spells/ -run 'TestValidateCastingMessages|TestGetCastMessage' 2>&1 | head -5
```

Expected: FAIL to build, `cm.Validate undefined`.

- [ ] **Step 3: Rewrite the loader, validator and renderer**

In `internal/spells/casting_messages.go`, replace everything from `var (` through the end of `GetCastMessage` with:

```go
var (
	castingMessages       *CastingMessages
	castingMessagesOnce   sync.Once
	castingMessagesLoaded bool
)

// minCastingVariants is casting's floor.
//
// THREE, not defence's five. casting-messages.yaml ships pools of 3, 3, 3 and
// 4, so defence's minimum would fail boot on shipped data without improving a
// single line of text. The validator's job is catching a regression (a pool
// emptied, a key renamed), not setting a content quality bar. The real content
// problem here, that cast_started fires on EVERY cast and has 3 variants
// against defence's 10 to 14 per pool, is filed as M6 content work.
const minCastingVariants = 3

// Validate enforces that every pool exists and is deep enough to be worth
// randomising. This store had no validator at all until 2026-09-09, because it
// is not in fileloader and therefore had no Validate() hook to implement.
func (cm *CastingMessages) Validate() error {
	pools := []struct {
		name string
		pool []string
	}{
		{"already_casting", cm.AlreadyCasting},
		{"cast_started", cm.CastStarted},
		{"cast_continuing", cm.CastContinuing},
		{"concentration_slipped", cm.ConcentrationSlipped},
	}
	for _, p := range pools {
		if err := narration.ValidateVariants(narration.Variants{Actor: p.pool}, minCastingVariants); err != nil {
			return fmt.Errorf("casting-messages %s: %w", p.name, err)
		}
	}
	return nil
}

// loadCastingMessages loads the YAML file once and caches the result.
//
// It PANICS on a missing, unparseable or invalid file, matching how defence
// and combat-messages behave (internal/items/itemspec.go:788, :795).
//
// There used to be a defaultCastingMessages() fallback here that silently
// substituted hardcoded Go text. It was deleted on 2026-09-09: a fallback that
// shadows shipped data means a YAML typo changes what players read and nobody
// finds out, which is the same hazard as reading a balance number from a Go
// default instead of config.yaml. Boot-fail and a silent fallback cannot both
// be the policy.
func loadCastingMessages() *CastingMessages {
	castingMessagesOnce.Do(func() {
		path := string(configs.GetFilePathsConfig().DataFiles) + `/casting-messages.yaml`
		data, err := os.ReadFile(path)
		if err != nil {
			panic(errors.Wrap(err, "reading "+path))
		}
		var cm CastingMessages
		if err := yaml.Unmarshal(data, &cm); err != nil {
			panic(errors.Wrap(err, "parsing "+path))
		}
		if err := cm.Validate(); err != nil {
			panic(errors.Wrap(err, "validating "+path))
		}
		castingMessages = &cm
		castingMessagesLoaded = true
	})
	return castingMessages
}

// GetCastMessage picks a message from the named category, substituting
// {spell} with spellName.
//
// category must be one of: "already_casting", "cast_started",
// "cast_continuing", "concentration_slipped".
//
// spellName is the player-facing DISPLAY name (spellInfo.Name), never the
// spellid. Passing the id leaks an internal identifier into player output,
// which is exactly what the round loop used to do.
//
// This is a DEGENERATE case for the narration core: one role (the caster), no
// band, one token. It renders through the same seam as the defence triad.
//
// The optional picker exists for the snapshot harness. Production passes none
// and gets narration.DefaultPicker.
func GetCastMessage(category, spellName string, picker ...narration.Picker) string {
	cm := castingMessages
	if !castingMessagesLoaded {
		cm = loadCastingMessages()
	}

	var pool []string
	switch category {
	case "already_casting":
		pool = cm.AlreadyCasting
	case "cast_started":
		pool = cm.CastStarted
	case "cast_continuing":
		pool = cm.CastContinuing
	case "concentration_slipped":
		pool = cm.ConcentrationSlipped
	}

	if len(pool) == 0 {
		return "Something stirs with " + spellName + "."
	}

	var pick narration.Picker
	if len(picker) > 0 {
		pick = picker[0]
	}

	return narration.Render(
		narration.Variants{Actor: pool},
		map[string]string{"{spell}": spellName},
		pick,
	).Actor
}
```

Update the import block to add `"github.com/pkg/errors"` and drop nothing else. Delete the entire `defaultCastingMessages` function.

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/spells/ 2>&1 | tail -3
```

Expected: `ok`.

- [ ] **Step 5: Verify the casting golden is byte-identical**

```bash
go test ./internal/narration/ -run TestSnapshotStores 2>&1 | tail -3
git status --short internal/narration/testdata/stores/
```

Expected: `ok` and no modified goldens.

- [ ] **Step 6: Confirm the shipped data actually passes the new validator**

A validator that has never been run against real data is a guess.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Exit code 124 is the success case. Then clean up:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-boot-check || (powershell -Command "Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check"; git worktree prune)
```

- [ ] **Step 7: Commit**

```bash
git add internal/spells/casting_messages.go internal/spells/casting_messages_test.go
git commit -m "refactor(spells): validate casting messages and render through the core

Casting is not in fileloader, so it never had a validator. It now has
one with a minimum of 3, because it ships pools of 3/3/3/4 and defence's
minimum of 5 would fail boot on shipped data without improving a line of
text. Bad data now panics, matching defence and combat-messages.

defaultCastingMessages is DELETED. A Go fallback that shadows shipped
data means a YAML typo silently changes what players read, which is the
same hazard as reading a balance number from a Go default rather than
config.yaml. Boot-fail and a silent fallback cannot both be the policy.

Golden byte-identical; boot verified against real data."
```

---

## Task 6: Documentation

**Files:**
- Modify: `internal/narration/context.md`
- Modify: `internal/itemvoices/context.md`
- Modify: `internal/spells/context.md`

- [ ] **Step 1: Verify every symbol before documenting it**

The project convention is that a `context.md` must never name a symbol its package does not define. `internal/messaging/context.md` carried four phantom symbols for months.

```bash
grep -E '^(func|type|const|var)\s' internal/narration/*.go | grep -v _test
```

- [ ] **Step 2: Update `internal/narration/context.md`**

Replace the "Purpose", "Files" and "Public API" sections to cover the core. The Purpose section currently says the package "does not render text" and "deliberately does not deliver anything"; the first half is now false and must change, the second half is still true and must stay.

```markdown
## Purpose

Owns two things, both on the RENDERING side of narration:

1. The seam that CHOOSES a line from a pool (`Picker`), so a snapshot run can
   be deterministic without touching global randomness.
2. The core that renders one event for its audiences from a SINGLE coordinated
   variant index (`Render`), so the room never sees a different event than the
   participants.

It does not deliver anything. Delivery is `internal/messaging`, which this
package must never import (see Gotchas).

## Files

| File | Holds |
|---|---|
| `picker.go` | `Picker`, `DefaultPicker`, `SequencePicker`. |
| `render.go` | `Selector`, `Variants`, `Roles`, `Render`, `ValidateVariants`. The core. |
| `picker_test.go` | Unit tests for the two pickers. |
| `render_test.go` | Unit tests for the core, including the coordinated-index probe. |
| `snapshot_test.go` | The M1 harness: golden snapshots of the message stores, raw and post-pipeline. |
| `testdata/stores/` | Those goldens. |

## Public API

```go
type Picker func(n int) int
func DefaultPicker(n int) int
func SequencePicker() Picker

type Selector string
type Variants struct{ Actor, Actee, Observer, ActeeObserver []string }
type Roles    struct{ Actor, Actee, Observer, ActeeObserver string }

func Render(v Variants, tokens map[string]string, pick Picker, indexOverride ...int) Roles
func ValidateVariants(v Variants, minVariants int) error
```

## The assembly rule

**The store assembles the pools and computes the selector. The core only
coordinates the index and substitutes tokens.**

The core deliberately knows nothing about bands, skill tiers or cooldowns.
`items.GetForSkillLevelWith` UNIONS beginner+expert+master by skill, and
`grapplemessaging.PickTemplate` FILTERS recently-used templates out; both hand
`Render` an already-assembled `[]string`. Pulling either into the core would
turn M4's parameter flip into a core rewrite.

## Gotchas

**`Render` takes ONE index for ALL roles, and that is the whole point.** Variant
N of each role describes the same moment. Picking per role gives every audience
a coherent-looking line describing a different event. That defect shipped twice
before this core existed: melee defence (PR #112) and taunt (PR #115).

**An index override still consumes a pick.** `Render` calls `pick(n)` and then
discards the draw when an override is supplied. This is bug-compatibility with
the defence store's original behaviour, and it matters because `DefaultPicker`
routes through `util.Rand`, which is global engine randomness: returning early
would shift every subsequent draw in the process.

**Unequal role pools render NOTHING.** `Variants.Len()` returns 0 when the
non-empty roles disagree, because index N cannot mean the same moment in a
5-entry pool and a 3-entry one. Use `ValidateVariants` at load time so this is
caught by a boot failure rather than by silence in play.

**`SequencePicker` returns a CLOSURE, and is not a seed.** Seeding global
randomness would be process-wide, and this repo runs every test in a package in
ONE binary, so a seeded snapshot's output would depend on which tests ran
before it.

**This package can never import `internal/messaging`.** `internal/items`
imports `narration`, and `messaging` reaches `items` through `characters`, so
the edge would close a cycle. Rendering-side selection lives low in the graph;
delivery lives high.
```

- [ ] **Step 3: Update `internal/itemvoices/context.md`**

Add `LineWith` to its public API section and note that `Line` delegates to it, and that the store renders through `narration.Render` as the single-role degenerate case.

- [ ] **Step 3b: Update `internal/spells/context.md`**

Three statements in it are now stale or newly true, and each must be corrected:

1. `defaultCastingMessages` no longer exists. Remove any mention of a hardcoded
   fallback for a missing or unparseable `casting-messages.yaml`.
2. `CastingMessages` now has a `Validate() error` method with a minimum of 3
   variants per pool. Document the minimum and say why it is 3 rather than
   defence's 5 (the file ships 3/3/3/4).
3. A malformed or missing `casting-messages.yaml` now **panics at startup**,
   matching defence and combat-messages, rather than silently substituting Go
   text. This is the single operator-visible behaviour change in the slice and
   is the thing most likely to surprise someone later.

- [ ] **Step 4: Run the context.md audit**

```bash
python tools/context_md_audit.py 2>&1 | grep -iE "narration|itemvoices|spells" | head -10
```

Expected: no findings for these three packages. The tool has known false positives elsewhere; only these three matter here.

- [ ] **Step 5: Commit**

```bash
git add internal/narration/context.md internal/itemvoices/context.md
git commit -m "docs: document the narration core and the assembly rule

narration/context.md said the package 'does not render text', which is
no longer true. Records the two gotchas most likely to be undone by a
later change: one index for all roles, and an index override still
consuming a pick."
```

---

## Task 7: Full verification

- [ ] **Step 1: Format and vet**

```bash
gofmt -l internal/ modules/
go vet ./... 2>&1 | head -10
```

Expected: gofmt prints nothing.

- [ ] **Step 2: Full suite**

```bash
go test ./... 2>&1 | grep -vE "^ok|no test files" | head -30
```

Expected: no failures. If `internal/playtestrun` fails here but passes standalone with `-count=1`, that is known contention under full-suite load, not a regression: check `go list -deps ./internal/playtestrun/` before hunting.

- [ ] **Step 3: Confirm all seven goldens are untouched**

```bash
git status --short internal/narration/testdata/stores/
git diff --stat master -- internal/narration/testdata/stores/
```

Expected: **only `itemvoices.golden` appears, as an addition.** Any modification to the other six means the migration changed behaviour and must be investigated, not re-recorded.

- [ ] **Step 4: Update PATCH_NOTES only if something player-visible changed**

M3.1 and M3.2 change no player-facing text; the byte-identical goldens are the proof. **Do not invent a patch note.** The one player-visible change is a negative: a malformed `casting-messages.yaml` now stops the server instead of silently serving different text. That is an operator-facing change, not a player-facing one, so it belongs in the commit message and this plan, not in PATCH_NOTES.

- [ ] **Step 5: Push and open the PR**

```bash
git push -u origin <branch>
gh pr create --repo pruuk/DOGMud --base master --head <branch> --fill
```

⚠️ `gh` defaults to the fork PARENT. **Every** `gh` command needs `--repo pruuk/DOGMud`. Read the URL it prints back and confirm it says `pruuk/DOGMud`.

```bash
gh run list --repo pruuk/DOGMud --branch <branch> --limit 10
```

`gh pr checks --watch` can report "no checks reported" before jobs register, so confirm against `gh run list` rather than trusting it.

---

## What this slice deliberately does NOT do

Stated so a reviewer does not read them as omissions:

- **Migrate taunt, grapple, buffs, spells, quests, crafting, conversations, combat-messages or weather.** Those are M3 items 3 to 9.
- **Populate `ActeeObserver`.** Nothing can until `combat-messages` migrates at item 8. It ships unused on purpose, so `Roles` is not reshaped later when the most stores depend on it.
- **Give `messaging.Trio` a fourth field.** Its AST guard requires every literal to name every role, so adding one would rewrite the role literals in all 25 files M2 migrated, for a slot nothing can fill yet. Item-8 prerequisite.
- **Unify the defence band split.** `GetDefenseMessage` bands on `zScore` while `RenderDefenseMessage` bands on crit plus margin. Unifying them CHANGES WHICH BAND FIRES, which is M4's flip, not a refactor.
- **Deepen casting's pools.** `cast_started` fires on every cast with 3 variants against defence's 10 to 14. Authored content, M6.
- **Route `mobcommands/taunt.go` through the store.** It would light up 28 currently unreachable `todefender` lines, but it changes what mobs say, so it carries the playtest gate. M3 item 3.
