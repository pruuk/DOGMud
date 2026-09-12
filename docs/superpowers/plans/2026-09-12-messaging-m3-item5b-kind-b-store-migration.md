# Messaging M3 item 5b: Kind B store migration, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the buff, spell and quest text stores onto `internal/narration` as a byte-identical refactor, proven by goldens built before the migration.

**Architecture:** `narration.FirstPicker` lets a one-variant pool render without a random draw. `textutil` becomes a thin adapter (`Tokens`, `Pool`, `Narrate`) and `SubstituteTokens` runs over the core. Each store gets a phase selector, `Narration(phase) narration.Variants` and `Narrate(phase, ctx) narration.Roles`; the 12 `SendPhaseText` sites, the quest bridge, the reward hook and the three silent-start appliers render through those doors and deliver inline exactly as today. `spelltext.go` is deleted at the end. Two root guards pin the shape.

**Tech Stack:** Go 1.2x, `go test`, the golden harness in `internal/narration/snapshot_test.go`, the root guards in the repo root package, the playtest harness (`go run ./cmd/playtestrun scenario`).

Spec: `docs/superpowers/specs/2026-09-12-messaging-m3-item5b-kind-b-store-migration-design.md`. Read it first; every design decision and its reason is there. Branch: `feature/messaging-m3-item5b-kind-b-migration` (already exists, spec committed at `a43dca778`).

## Rules for this plan

- **Never run `go test ... -update` after Task 0.** The goldens are the proof. A red golden after Task 0 means the refactor changed behaviour; fix the code, not the golden. The rule expires when 5b merges: these goldens snapshot live world data, so a later content PR that adds a buff, spell or quest re-records them deliberately, in its own commit.
- **The goldens see rendered strings, never delivery.** Category, channel (`SendTextVisual` versus `Room.SendText`, the lit variant for a light buff) and the exclusion id at every site can all be wrong with all ten goldens green. Tasks 6 to 8 preserve those by reading the old site; the viewpoint registry and the lane A playtest are the net for them.
- **Run every test command standalone**, never `| tail`. `go test ... | tail -1` masks the exit code (a push went out red once).
- **Buff, spell and quest YAML is CRLF.** A bare `$` in grep matches nothing there; use `\s*$`.
- **One commit per task**, named paths only (`git add <file> <file>`, never `-A` or `.`). Commit messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.
- **Every `gh` command carries `--repo pruuk/DOGMud`.**
- `go test ./...` may red `internal/playtestrun` under CPU load; run that package standalone before believing it.
- The Go build cache can fill C:; if a build fails on disk, `go clean -cache`.
- Before every commit: `gofmt -l ./internal ./cmd . ` must print nothing.

## File map

| File | Responsibility |
|---|---|
| `internal/narration/picker.go` | gains `FirstPicker` |
| `internal/narration/picker_test.go` | `FirstPicker` tests |
| `internal/narration/snapshot_test.go` | gains three builders and three subtests; loads the three stores |
| `internal/narration/testdata/stores/{buffs,spells,quests}.golden` | new goldens |
| `internal/textutil/tokens.go` | `Tokens()`; `SubstituteTokens` over the core |
| `internal/textutil/narrate.go` (new) | `Pool`, `Narrate` |
| `internal/textutil/narrate_test.go` (new) | adapter tests |
| `internal/textutil/spelltext.go` | DELETED in Task 9 |
| `internal/buffs/narration.go` (new) | `Phase`, `Narration`, `Narrate`, `AuthoredStartLine` |
| `internal/buffs/narration_test.go` (new) | door tests |
| `internal/buffs/buffspec.go` | `Validate` runs `ValidateVariants` per phase |
| `internal/spells/narration.go` (new), `narration_test.go` (new), `spells.go` | same for spells |
| `internal/quests/narration.go` (new), `narration_test.go` (new) | action and reward doors; the both-set rule and per-line validation |
| `internal/quests/quests.go` | `Validate` calls `validateNarration` |
| `internal/questengine/actions.go`, `bridge.go`, `actions_test.go` | `Narrate(v)` replaces `SendText`/`RoomText` |
| `internal/hooks/Buff_ApplyBuffs.go`, `NewRound_UserRoundTick.go`, `NewTurn_PruneBuffs.go`, `NewRound_MobRoundTick.go`, `Position_Messaging.go`, `spell_resolution.go`, `NewRound_DoCombat_helpers.go`, `Quest_HandleQuestUpdate.go` | sites |
| `internal/actions/sleep.go`, `internal/justice/arrest.go`, `internal/mobcommands/aid.go`, `internal/mobcommands/cast.go`, `internal/usercommands/skill.cast.go` | sites |
| `narration_render_callers_guard_test.go` (new, repo root) | guard 1 |
| `store_text_fields_guard_test.go` (new, repo root) | guard 2 |
| `messaging_surface_guard_test.go` | viewpoint registry entries |
| six `context.md`, `docs/README.md`, the spec | docs |

---

### Task 0: Goldens first, from pre-migration code

The net must exist before anything moves. These builders read today's fields and call today's functions; Task 9 rewrites them to call the store doors, and the files must not change by a byte.

**Files:**
- Modify: `internal/narration/snapshot_test.go`
- Create: `internal/narration/testdata/stores/buffs.golden`, `spells.golden`, `quests.golden`

- [ ] **Step 1: Add the imports and load the three stores**

In the import block of `snapshot_test.go` add:

```go
	"slices"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/textutil"
```

In `setupRealStores`, after `spells.LoadCastingMessages()`:

```go
	// Kind B stores (M3 item 5b). Their loaders read the same configured data
	// path, so the golden sees exactly what a booted dogmud world sees.
	buffs.LoadDataFiles()
	spells.LoadSpellFiles()
	quests.LoadDataFiles()
```

- [ ] **Step 2: Add the stand-ins and the three builders**

Append to `snapshot_test.go`:

```go
// ---------------------------------------------------------------------
// Kind B stores (M3 item 5b): buffs, spells, quests. Single strings per
// lifecycle phase, no pool, so no picker is involved: the golden freezes the
// substitution and the notice logic, keyed by the AUTHORED key name so a
// swapped role shows up as a changed row.
// ---------------------------------------------------------------------

// kindBSource is the stand-in name set. The source is a player and the target
// a mob, so the two tags differ and a swap of {source} for {target} would
// change the golden.
var kindBSource = textutil.TokenContext{
	SourceName:      `<ansi fg="username">Aliceia</ansi>`,
	SourcePlainName: "Aliceia",
	TargetName:      `<ansi fg="mobname">Targetticus</ansi>`,
	TargetPlainName: "Targetticus",
}

// kindBNoTarget is the same source with no target, which is how every buff
// site and the quest bridge render: they never know a target.
var kindBNoTarget = textutil.TokenContext{
	SourceName:      kindBSource.SourceName,
	SourcePlainName: kindBSource.SourcePlainName,
}

// Store 8: buffs (internal/buffs, six *_user_text / *_room_text fields)
func buildBuffsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# buffs store snapshot (internal/buffs)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code. The *_user_text rows record what the\n")
	fmt.Fprintf(&b, "# HOLDER is sent: for start and end that is StartUserNotice / EndUserNotice (authored\n")
	fmt.Fprintf(&b, "# line, else the generic fallback, else nothing for a secret buff). A row exists only\n")
	fmt.Fprintf(&b, "# when the sent line is non-empty. authored_start_line is the raw start_user_text of a\n")
	fmt.Fprintf(&b, "# silent-start buff, as its applier (sleep, arrest, stun, broken limb) sends it.\n")
	fmt.Fprintf(&b, "# dimensions: buff id x authored key; source only, buffs never know a target\n\n")

	ids := buffs.GetAllBuffIds()
	sort.Ints(ids)
	if len(ids) == 0 {
		t.Fatal("no buffs loaded; setupRealStores must call buffs.LoadDataFiles()")
	}
	for _, id := range ids {
		spec := buffs.GetBuffSpec(id)
		rows := []struct{ key, text string }{
			{"start_user_text", spec.StartUserNotice()},
			{"start_room_text", spec.StartRoomText},
			{"trigger_user_text", spec.TriggerUserText},
			{"trigger_room_text", spec.TriggerRoomText},
			{"end_user_text", spec.EndUserNotice()},
			{"end_room_text", spec.EndRoomText},
		}
		for _, r := range rows {
			if line := textutil.SubstituteTokens(r.text, kindBNoTarget); line != "" {
				fmt.Fprintf(&b, "buff|%d|%s => %s\n", id, r.key, line)
			}
		}
		if slices.Contains(spec.Flags, buffs.SilentStart) && spec.StartUserText != "" {
			fmt.Fprintf(&b, "buff|%d|authored_start_line => %s\n", id, spec.StartUserText)
		}
	}
	return b.String()
}

// Store 9: spells (internal/spells, six cast/wait/magic x user/room fields)
func buildSpellsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# spells store snapshot (internal/spells)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code: textutil.SubstituteTokens over each raw\n")
	fmt.Fprintf(&b, "# field with a source AND a target. A |notarget row follows any line whose rendering\n")
	fmt.Fprintf(&b, "# changes when the target is absent, freezing the empty substitution.\n")
	fmt.Fprintf(&b, "# dimensions: spell id x authored key [x notarget]\n\n")

	all := spells.GetAllSpells()
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatal("no spells loaded; setupRealStores must call spells.LoadSpellFiles()")
	}
	for _, id := range ids {
		s := all[id]
		rows := []struct{ key, text string }{
			{"cast_user_text", s.CastUserText},
			{"cast_room_text", s.CastRoomText},
			{"wait_user_text", s.WaitUserText},
			{"wait_room_text", s.WaitRoomText},
			{"magic_user_text", s.MagicUserText},
			{"magic_room_text", s.MagicRoomText},
		}
		for _, r := range rows {
			line := textutil.SubstituteTokens(r.text, kindBSource)
			if line == "" {
				continue
			}
			fmt.Fprintf(&b, "spell|%s|%s => %s\n", id, r.key, line)
			if noTarget := textutil.SubstituteTokens(r.text, kindBNoTarget); noTarget != line {
				fmt.Fprintf(&b, "spell|%s|%s|notarget => %s\n", id, r.key, noTarget)
			}
		}
	}
	return b.String()
}

// Store 10: quests (internal/quests: reward playermessage/roommessage, and the
// send_text / room_text actions, nested sequences included)
func buildQuestsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# quests store snapshot (internal/quests)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code, sending what each site sends today:\n")
	fmt.Fprintf(&b, "# rewards playermessage/roommessage and action send_text RAW (no substitution),\n")
	fmt.Fprintf(&b, "# action room_text through textutil.SubstituteTokens with the player as {source}.\n")
	fmt.Fprintf(&b, "# dimensions: quest id x rewards | trigger<i>|action<j>[|sequence|action<k>...] x key\n\n")

	all := quests.GetAllQuests()
	sort.Slice(all, func(i, j int) bool { return all[i].QuestId < all[j].QuestId })
	if len(all) == 0 {
		t.Fatal("no quests loaded; setupRealStores must call quests.LoadDataFiles()")
	}

	var walk func(where string, actions []quests.ActionDef)
	walk = func(where string, actions []quests.ActionDef) {
		for j, a := range actions {
			aw := fmt.Sprintf("%s|action%d", where, j)
			if a.SendText != "" {
				fmt.Fprintf(&b, "%s|send_text => %s\n", aw, a.SendText)
			}
			if a.RoomText != "" {
				fmt.Fprintf(&b, "%s|room_text => %s\n", aw, textutil.SubstituteTokens(a.RoomText, kindBNoTarget))
			}
			if a.Sequence != nil {
				walk(aw+"|sequence", a.Sequence.OnComplete)
			}
		}
	}
	for _, q := range all {
		if q.Rewards.PlayerMessage != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|playermessage => %s\n", q.QuestId, q.Rewards.PlayerMessage)
		}
		if q.Rewards.RoomMessage != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|roommessage => %s\n", q.QuestId, q.Rewards.RoomMessage)
		}
		for i, tr := range q.Triggers {
			walk(fmt.Sprintf("quest|%d|trigger%d", q.QuestId, i), tr.Actions)
		}
	}
	return b.String()
}
```

- [ ] **Step 3: Register the three subtests**

In `TestSnapshotStores`, after the `itemvoices` subtest and before `post_pipeline`:

```go
	t.Run("buffs", func(t *testing.T) {
		checkGolden(t, "buffs.golden", buildBuffsGolden(t))
	})
	t.Run("spells", func(t *testing.T) {
		checkGolden(t, "spells.golden", buildSpellsGolden(t))
	})
	t.Run("quests", func(t *testing.T) {
		checkGolden(t, "quests.golden", buildQuestsGolden(t))
	})
```

- [ ] **Step 4: Prove the builders are capable of failing**

Run: `go test ./internal/narration/ -run 'TestSnapshotStores/(buffs|spells|quests)' -count=1`
Expected: FAIL three times with `read golden ... buffs.golden: ... no such file` (and spells, quests). If a builder panics instead, fix the loader call, not the builder.

- [ ] **Step 5: Record the goldens ONCE**

Run: `go test ./internal/narration/ -run TestSnapshotStores -update -count=1`
Expected: `ok`. This is the ONLY `-update` in this plan.

Then sanity-check the content:

```bash
wc -l internal/narration/testdata/stores/buffs.golden internal/narration/testdata/stores/spells.golden internal/narration/testdata/stores/quests.golden
grep -c "authored_start_line" internal/narration/testdata/stores/buffs.golden
grep -c "|notarget" internal/narration/testdata/stores/spells.golden
grep -c "|room_text =>" internal/narration/testdata/stores/quests.golden
git check-attr eol internal/narration/testdata/stores/buffs.golden
```
Expected: buffs has roughly 250 rows plus header; `authored_start_line` DATA rows exactly 5 (15, 83, 84, 88, 89; 79 and 80 are silent-start with no text; the grep also counts the header sentence, so subtract it); `|notarget` DATA rows exactly 2 (charm, repair-pulse; the header sentence matches too); `|room_text =>` exactly 22; `eol: lf`. If `|notarget` is not 2 or `room_text` is not 22, the builder is wrong; fix it and re-record (this is still Task 0).

Also confirm the seven existing goldens did not change: `git status --short internal/narration/testdata/stores/` must list ONLY the three new files.

- [ ] **Step 6: Run the whole harness and commit**

Run: `go test ./internal/narration/ -count=1`
Expected: `ok`.

```bash
git add internal/narration/snapshot_test.go internal/narration/testdata/stores/buffs.golden internal/narration/testdata/stores/spells.golden internal/narration/testdata/stores/quests.golden
git commit -m "test(narration): golden-snapshot the buff, spell and quest stores before migrating them"
```

---

### Task 1: `narration.FirstPicker`

**Files:**
- Modify: `internal/narration/picker.go`
- Modify: `internal/narration/picker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `picker_test.go`:

```go
// FirstPicker is for a single-variant store. It must return 0 for every n and
// must not be DefaultPicker in disguise: DefaultPicker(1) still calls
// util.Rand(1), which still calls rand.Intn, so routing ~750 single-string
// fields through it would add one global draw per narrated phase.
func TestFirstPickerAlwaysReturnsZero(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 100} {
		if got := FirstPicker(n); got != 0 {
			t.Fatalf("FirstPicker(%d) = %d, want 0", n, got)
		}
	}
}

func TestRenderWithFirstPickerTakesTheOnlyVariant(t *testing.T) {
	roles := Render(Variants{Actee: []string{"You feel {x}."}, Observer: []string{"A glow."}},
		map[string]string{"{x}": "warm"}, FirstPicker)
	if roles.Actee != "You feel warm." || roles.Observer != "A glow." || roles.Actor != "" {
		t.Fatalf("unexpected roles: %+v", roles)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/narration/ -run 'TestFirstPicker|TestRenderWithFirstPicker' -count=1`
Expected: FAIL to compile, `undefined: FirstPicker`.

- [ ] **Step 3: Implement**

Append to `picker.go`:

```go
// FirstPicker always returns 0 and never touches util.Rand. It is the picker
// for a single-variant store (buffs, spells, quests), where there is nothing
// to choose.
//
// It exists because Render always calls pick(n), DefaultPicker always calls
// util.Rand(n) for n >= 1, and util.Rand(1) still calls rand.Intn. Rendering a
// one-variant pool through DefaultPicker would therefore consume one GLOBAL
// random draw per narrated buff, spell or quest phase and shift every later
// combat roll. Render cannot special-case a pool of one instead: itemvoices
// never validates its pool sizes, so a one-line voice pool is legitimate and
// its draw count must not change either.
func FirstPicker(n int) int { return 0 }
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/narration/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/narration/picker.go internal/narration/picker_test.go
git commit -m "feat(narration): FirstPicker, the picker for a single-variant store"
```

---

### Task 2: The `textutil` adapter

**Files:**
- Modify: `internal/textutil/tokens.go`
- Create: `internal/textutil/narrate.go`
- Create: `internal/textutil/narrate_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/textutil/narrate_test.go`:

```go
package textutil

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

func TestTokensCarriesAllFourKeysEvenWhenEmpty(t *testing.T) {
	m := TokenContext{SourceName: "S", SourcePlainName: "Sp"}.Tokens()
	want := map[string]string{"{source}": "S", "{source_plain}": "Sp", "{target}": "", "{target_plain}": ""}
	if len(m) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(m), len(want), m)
	}
	for k, v := range want {
		got, ok := m[k]
		if !ok || got != v {
			t.Fatalf("key %q = %q (present %v), want %q", k, got, ok, v)
		}
	}
}

func TestPoolIsNilForEmptyAndOneVariantOtherwise(t *testing.T) {
	if got := Pool(""); got != nil {
		t.Fatalf("Pool(\"\") = %v, want nil", got)
	}
	if got := Pool("  "); len(got) != 1 || got[0] != "  " {
		t.Fatalf("Pool(whitespace) = %v, want the whitespace kept so a validator can refuse it", got)
	}
	if got := Pool("x"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("Pool(x) = %v", got)
	}
}

func TestNarrateSubstitutesEveryRoleFromTheOneVariant(t *testing.T) {
	ctx := TokenContext{SourceName: `<ansi fg="username">Kael</ansi>`, SourcePlainName: "Kael", TargetName: "Goblin", TargetPlainName: "Goblin"}
	roles := Narrate(narration.Variants{
		Actor:    Pool("You hex {target}."),
		Actee:    Pool("{source} hexes you."),
		Observer: Pool("{source_plain}'s hex lands on {target_plain}."),
	}, ctx)
	if roles.Actor != "You hex Goblin." {
		t.Fatalf("actor: %q", roles.Actor)
	}
	if roles.Actee != `<ansi fg="username">Kael</ansi> hexes you.` {
		t.Fatalf("actee: %q", roles.Actee)
	}
	if roles.Observer != "Kael's hex lands on Goblin." {
		t.Fatalf("observer: %q", roles.Observer)
	}
	if roles.ActeeObserver != "" {
		t.Fatalf("acteeObserver should be empty, got %q", roles.ActeeObserver)
	}
}

func TestNarrateRendersNothingForNoVariants(t *testing.T) {
	if roles := Narrate(narration.Variants{}, TokenContext{}); roles != (narration.Roles{}) {
		t.Fatalf("got %+v, want zero Roles", roles)
	}
}

func TestSubstituteTokensAndNarrateAgree(t *testing.T) {
	ctx := TokenContext{SourceName: "A", SourcePlainName: "a", TargetName: "B", TargetPlainName: "b"}
	for _, text := range []string{
		"{source} at {target}; {source_plain}/{target_plain}; {unknown} stays",
		"no tokens",
		"{source}{source}",
	} {
		if got, want := SubstituteTokens(text, ctx), Narrate(narration.Variants{Actor: Pool(text)}, ctx).Actor; got != want {
			t.Fatalf("%q: SubstituteTokens %q != Narrate %q", text, got, want)
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/textutil/ -count=1`
Expected: FAIL to compile, `undefined: Pool`, `Tokens`, `Narrate`.

- [ ] **Step 3: Implement the adapter**

Create `internal/textutil/narrate.go`:

```go
package textutil

import "github.com/GoMudEngine/GoMud/internal/narration"

// Pool is a single-variant pool: nil for an empty string, otherwise the one
// line. Whitespace is kept, not trimmed, so narration.ValidateVariants can
// refuse a whitespace-only authored line at load instead of a site sending it.
func Pool(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}

// Narrate renders one single-variant event for its audiences.
//
// It is the ONLY way the Kind B stores (buffs, spells, quests) reach
// narration.Render, and it always passes narration.FirstPicker: these stores
// hold one line per phase, so there is nothing to choose, and the default
// picker would consume a global random draw per narrated phase (see
// FirstPicker). The root guard narration_render_callers_guard_test.go pins
// both facts.
func Narrate(v narration.Variants, ctx TokenContext) narration.Roles {
	return narration.Render(v, ctx.Tokens(), narration.FirstPicker)
}
```

In `tokens.go`, add `Tokens` after the `TokenContext` type and rewrite `SubstituteTokens`:

```go
// Tokens is the vocabulary as the narration core takes it. All four keys are
// always present, so an absent target substitutes to an empty string, which
// is what SubstituteTokens has always done.
func (ctx TokenContext) Tokens() map[string]string {
	return map[string]string{
		`{source}`:       ctx.SourceName,
		`{target}`:       ctx.TargetName,
		`{source_plain}`: ctx.SourcePlainName,
		`{target_plain}`: ctx.TargetPlainName,
	}
}

// SubstituteTokens replaces known tokens in text with values from ctx.
// Unknown tokens are left as-is. Empty string input returns empty string.
//
// Since M3 item 5b it is a one-variant Narrate, so one engine substitutes
// for every store. The result is identical to the former four-pair
// strings.NewReplacer: no token is a prefix of another (each ends in "}"),
// so pair order cannot change the output.
func SubstituteTokens(text string, ctx TokenContext) string {
	if text == "" {
		return ""
	}
	return Narrate(narration.Variants{Actor: Pool(text)}, ctx).Actor
}
```

In `tokens.go`'s import block, add `"github.com/GoMudEngine/GoMud/internal/narration"` and remove `"strings"`, which nothing else in the file uses (`regexp` stays for `ValidateTokens`).

- [ ] **Step 4: Run to verify they pass, and that the goldens still hold**

Run: `go test ./internal/textutil/ ./internal/narration/ -count=1`
Expected: both `ok`. The `SubstituteTokens` rewrite is exercised by the three new goldens, which were recorded with the old engine.

- [ ] **Step 5: Commit**

```bash
git add internal/textutil/tokens.go internal/textutil/narrate.go internal/textutil/narrate_test.go
git commit -m "feat(textutil): Tokens, Pool and Narrate, the adapter the Kind B stores render through"
```

---

### Task 3: The buffs door

**Files:**
- Create: `internal/buffs/narration.go`
- Create: `internal/buffs/narration_test.go`
- Modify: `internal/buffs/buffspec.go:272-279` (Validate)

- [ ] **Step 1: Write the failing tests**

Create `internal/buffs/narration_test.go`:

```go
package buffs

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func glowSpec() *BuffSpec {
	return &BuffSpec{BuffId: 500, Name: "Glow", StartRoomText: "A glow surrounds {source}.", EndUserText: "The glow fades.", TriggerUserText: "You shimmer."}
}

func TestNarrationStartPutsTheHolderInActeeAndUsesTheNotice(t *testing.T) {
	v := glowSpec().Narration(PhaseStart)
	if len(v.Actor) != 0 {
		t.Fatalf("Actor must stay empty (reserved for the caster), got %v", v.Actor)
	}
	if len(v.Actee) != 1 || v.Actee[0] != "Glow takes effect." {
		t.Fatalf("Actee should be the generic notice, got %v", v.Actee)
	}
	if len(v.Observer) != 1 || v.Observer[0] != "A glow surrounds {source}." {
		t.Fatalf("Observer should be the raw room line, got %v", v.Observer)
	}
}

func TestNarrationTriggerAndEnd(t *testing.T) {
	s := glowSpec()
	if v := s.Narration(PhaseTrigger); len(v.Actee) != 1 || v.Actee[0] != "You shimmer." || len(v.Observer) != 0 {
		t.Fatalf("trigger: %+v", v)
	}
	if v := s.Narration(PhaseEnd); len(v.Actee) != 1 || v.Actee[0] != "The glow fades." || len(v.Observer) != 0 {
		t.Fatalf("end: %+v", v)
	}
}

func TestNarrationSecretBuffHasNoHolderLine(t *testing.T) {
	s := glowSpec()
	s.Secret = true
	if v := s.Narration(PhaseStart); len(v.Actee) != 0 {
		t.Fatalf("a secret buff must not narrate to its holder, got %v", v.Actee)
	}
}

func TestNarrateSubstitutesTheHolderName(t *testing.T) {
	roles := glowSpec().Narrate(PhaseStart, textutil.TokenContext{SourceName: "Aliceia", SourcePlainName: "Aliceia"})
	if roles.Actee != "Glow takes effect." || roles.Observer != "A glow surrounds Aliceia." || roles.Actor != "" {
		t.Fatalf("roles: %+v", roles)
	}
}

func TestNarrateAPhaseWithNoTextRendersNothing(t *testing.T) {
	s := &BuffSpec{BuffId: 501, Name: "Quiet", Secret: true}
	if v := s.Narration(PhaseStart); v.Len() != 0 {
		t.Fatalf("expected no variants, got %+v", v)
	}
	if roles := s.Narrate(PhaseStart, textutil.TokenContext{}); roles != (narration.Roles{}) {
		t.Fatalf("expected zero roles, got %+v", roles)
	}
}

func TestAuthoredStartLineIgnoresTheSilentStartRule(t *testing.T) {
	s := &BuffSpec{BuffId: 502, Name: "Sleeping", Flags: []Flag{SilentStart}, StartUserText: "You lie down, {source_plain}."}
	if got := s.StartUserNotice(); got != "" {
		t.Fatalf("notice should be silent for silent-start, got %q", got)
	}
	if got := s.AuthoredStartLine(textutil.TokenContext{SourcePlainName: "Aliceia"}); got != "You lie down, Aliceia." {
		t.Fatalf("AuthoredStartLine = %q", got)
	}
}

func TestValidateRefusesAWhitespaceOnlyLine(t *testing.T) {
	s := glowSpec()
	s.TriggerRoomText = "   "
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "trigger") {
		t.Fatalf("expected a trigger-phase validation error, got %v", err)
	}
	s.TriggerRoomText = ""
	if err := s.Validate(); err != nil {
		t.Fatalf("a spec with ordinary text must validate, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/buffs/ -run 'TestNarrat|TestAuthoredStartLine|TestValidateRefusesAWhitespace' -count=1`
Expected: FAIL to compile, `undefined: PhaseStart` etc.

- [ ] **Step 3: Implement the door**

Create `internal/buffs/narration.go`:

```go
package buffs

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Phase selects which of a buff's three narrated moments to render. A buff
// holds one line per phase and audience, so the phase IS the selector: there
// is no pool and nothing to pick.
type Phase uint8

const (
	PhaseStart Phase = iota
	PhaseTrigger
	PhaseEnd
)

// Narration assembles the variants for one phase.
//
// The holder's line is the ACTEE: the buff happens to them. The room's line is
// the Observer. Actor is empty and reserved for the caster, which M6 authors
// once events.Buff carries a caster (owner ruling, 2026-09-12). Start and End
// go through StartUserNotice / EndUserNotice, so the secret, hidden,
// silent-start and generic-fallback rules stay in their one door.
func (b *BuffSpec) Narration(p Phase) narration.Variants {
	var holder, room string
	switch p {
	case PhaseStart:
		holder, room = b.StartUserNotice(), b.StartRoomText
	case PhaseTrigger:
		holder, room = b.TriggerUserText, b.TriggerRoomText
	case PhaseEnd:
		holder, room = b.EndUserNotice(), b.EndRoomText
	}
	return narration.Variants{Actee: textutil.Pool(holder), Observer: textutil.Pool(room)}
}

// Narrate renders one phase for its audiences with the holder as {source}.
func (b *BuffSpec) Narrate(p Phase, ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(b.Narration(p), ctx)
}

// AuthoredStartLine renders start_user_text as written, ignoring the notice
// rules. It is the door for the applier of a silent-start buff, which narrates
// the start itself because the buff never travels the event that would:
// sleep (15), arrest (88), stun (84) and broken limb (83).
func (b *BuffSpec) AuthoredStartLine(ctx textutil.TokenContext) string {
	return textutil.SubstituteTokens(b.StartUserText, ctx)
}

// validateNarration refuses a phase whose authored text cannot be rendered:
// a whitespace-only line, which ValidateVariants reports as an empty variant.
// It checks the RAW fields, not the notices, so a silent-start buff's hidden
// start line is checked too.
func (b *BuffSpec) validateNarration() error {
	phases := []struct{ name, user, room string }{
		{"start", b.StartUserText, b.StartRoomText},
		{"trigger", b.TriggerUserText, b.TriggerRoomText},
		{"end", b.EndUserText, b.EndRoomText},
	}
	for _, ph := range phases {
		if ph.user == "" && ph.room == "" {
			continue
		}
		v := narration.Variants{Actee: textutil.Pool(ph.user), Observer: textutil.Pool(ph.room)}
		if err := narration.ValidateVariants(v, 1); err != nil {
			return fmt.Errorf("buffId %d (%s) %s text: %w", b.BuffId, b.Name, ph.name, err)
		}
	}
	return nil
}
```

In `buffspec.go`'s `Validate`, directly after the token-warning loop (line 279, before `// Validate tick fields`):

```go
	// A whitespace-only line would be sent as-is; refuse it at load.
	if err := b.validateNarration(); err != nil {
		return err
	}
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/buffs/ -count=1`
Expected: `ok`. Then `go test ./internal/narration/ -count=1` (still `ok`; nothing renders through the door yet).

- [ ] **Step 5: Commit**

```bash
git add internal/buffs/narration.go internal/buffs/narration_test.go internal/buffs/buffspec.go
git commit -m "feat(buffs): Phase, Narration, Narrate and AuthoredStartLine, the store's narration door"
```

---

### Task 4: The spells door

**Files:**
- Create: `internal/spells/narration.go`
- Create: `internal/spells/narration_test.go`
- Modify: `internal/spells/spells.go:300-307` (Validate)

- [ ] **Step 1: Write the failing tests**

Create `internal/spells/narration_test.go`:

```go
package spells

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func boltSpec() *SpellData {
	// PrimaryStat is required by Validate (U9 made it load-bearing).
	return &SpellData{SpellId: "bolt", Name: "Bolt", PrimaryStat: "willpower", CastUserText: "You gather a bolt.", CastRoomText: "{source} gathers a bolt at {target}.", WaitUserText: "You hold the bolt."}
}

func TestNarrationCastPutsTheCasterInActor(t *testing.T) {
	v := boltSpec().Narration(PhaseCast)
	if len(v.Actor) != 1 || v.Actor[0] != "You gather a bolt." {
		t.Fatalf("Actor: %v", v.Actor)
	}
	if len(v.Observer) != 1 || v.Observer[0] != "{source} gathers a bolt at {target}." {
		t.Fatalf("Observer: %v", v.Observer)
	}
	if len(v.Actee) != 0 {
		t.Fatalf("Actee must be empty (M6 authors it), got %v", v.Actee)
	}
}

func TestNarrationWaitAndMagic(t *testing.T) {
	s := boltSpec()
	if v := s.Narration(PhaseWait); len(v.Actor) != 1 || v.Actor[0] != "You hold the bolt." || len(v.Observer) != 0 {
		t.Fatalf("wait: %+v", v)
	}
	if v := s.Narration(PhaseMagic); v.Len() != 0 {
		t.Fatalf("magic has no text, got %+v", v)
	}
}

func TestNarrateSubstitutesSourceAndTarget(t *testing.T) {
	roles := boltSpec().Narrate(PhaseCast, textutil.TokenContext{SourceName: "Kael", TargetName: "Goblin"})
	if roles.Actor != "You gather a bolt." || roles.Observer != "Kael gathers a bolt at Goblin." {
		t.Fatalf("roles: %+v", roles)
	}
	if roles := boltSpec().Narrate(PhaseMagic, textutil.TokenContext{}); roles != (narration.Roles{}) {
		t.Fatalf("magic should render nothing, got %+v", roles)
	}
}

func TestValidateRefusesAWhitespaceOnlyLine(t *testing.T) {
	s := boltSpec()
	s.WaitRoomText = " "
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "wait") {
		t.Fatalf("expected a wait-phase validation error, got %v", err)
	}
	s.WaitRoomText = ""
	if err := s.Validate(); err != nil {
		t.Fatalf("ordinary text must validate, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/spells/ -run 'TestNarrat|TestValidateRefusesAWhitespace' -count=1`
Expected: FAIL to compile, `undefined: PhaseCast`.

- [ ] **Step 3: Implement**

Create `internal/spells/narration.go`:

```go
package spells

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Phase selects which of a spell's three narrated moments to render: the cast
// command, each round of the channel, and the moment the magic resolves. One
// line per phase and audience, so the phase IS the selector.
type Phase uint8

const (
	PhaseCast Phase = iota
	PhaseWait
	PhaseMagic
)

// Narration assembles the variants for one phase: the caster's line is the
// Actor, the room's line the Observer. Actee is empty; the target's own line
// is authored in M6.
func (s *SpellData) Narration(p Phase) narration.Variants {
	var caster, room string
	switch p {
	case PhaseCast:
		caster, room = s.CastUserText, s.CastRoomText
	case PhaseWait:
		caster, room = s.WaitUserText, s.WaitRoomText
	case PhaseMagic:
		caster, room = s.MagicUserText, s.MagicRoomText
	}
	return narration.Variants{Actor: textutil.Pool(caster), Observer: textutil.Pool(room)}
}

// Narrate renders one phase with the caster as {source} and the first target,
// if any, as {target}.
func (s *SpellData) Narrate(p Phase, ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(s.Narration(p), ctx)
}

// validateNarration refuses a phase whose authored text is whitespace only.
func (s *SpellData) validateNarration() error {
	phases := []struct{ name string; p Phase }{{"cast", PhaseCast}, {"wait", PhaseWait}, {"magic", PhaseMagic}}
	for _, ph := range phases {
		v := s.Narration(ph.p)
		if len(v.Actor) == 0 && len(v.Observer) == 0 {
			continue
		}
		if err := narration.ValidateVariants(v, 1); err != nil {
			return fmt.Errorf("spell %s %s text: %w", s.SpellId, ph.name, err)
		}
	}
	return nil
}
```

In `spells.go`'s `Validate`, directly after the token-warning loop (before `// Validate summon fields`):

```go
	if err := s.validateNarration(); err != nil {
		return err
	}
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/spells/ ./internal/narration/ -count=1`
Expected: both `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/spells/narration.go internal/spells/narration_test.go internal/spells/spells.go
git commit -m "feat(spells): Phase, Narration and Narrate, the store's narration door"
```

---

### Task 5: The quests door and the both-set rule

**Files:**
- Create: `internal/quests/narration.go`
- Create: `internal/quests/narration_test.go`
- Modify: `internal/quests/quests.go:172-176` (Validate)

- [ ] **Step 1: Write the failing tests**

Create `internal/quests/narration_test.go`:

```go
package quests

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/textutil"
)

func TestActionNarrationSendTextIsActorAndRoomTextIsObserver(t *testing.T) {
	v := ActionDef{SendText: "You pocket the disc."}.Narration()
	if len(v.Actor) != 1 || v.Actor[0] != "You pocket the disc." || len(v.Observer) != 0 {
		t.Fatalf("send_text: %+v", v)
	}
	v = ActionDef{RoomText: "{source} pockets a disc."}.Narration()
	if len(v.Observer) != 1 || v.Observer[0] != "{source} pockets a disc." || len(v.Actor) != 0 {
		t.Fatalf("room_text: %+v", v)
	}
	if v := (ActionDef{Grant: "1-end"}).Narration(); v.Len() != 0 {
		t.Fatalf("a non-text action narrates nothing, got %+v", v)
	}
}

func TestActionNarrateSubstitutesThePlayer(t *testing.T) {
	roles := ActionDef{RoomText: "{source} pockets a disc."}.Narrate(textutil.TokenContext{SourceName: "Aliceia"})
	if roles.Observer != "Aliceia pockets a disc." || roles.Actor != "" {
		t.Fatalf("roles: %+v", roles)
	}
}

func TestRewardNarration(t *testing.T) {
	r := QuestReward{PlayerMessage: "The clerk thanks you.", RoomMessage: "The clerk thanks {source}."}
	roles := r.Narrate(textutil.TokenContext{SourceName: "Aliceia"})
	if roles.Actor != "The clerk thanks you." || roles.Observer != "The clerk thanks Aliceia." {
		t.Fatalf("roles: %+v", roles)
	}
}

// validQuest is the smallest quest Validate accepts, with one text action.
func validQuest(a ActionDef) *Quest {
	return &Quest{
		QuestId: 9001, Name: "Probe",
		Steps:    []QuestStep{{Id: "start"}},
		Triggers: []TriggerDef{{Event: "command", Actions: []ActionDef{a}}},
	}
}

func TestValidateRefusesAnActionThatSetsBothTexts(t *testing.T) {
	err := validQuest(ActionDef{SendText: "You see it.", RoomText: "{source} sees it."}).Validate()
	if err == nil || !strings.Contains(err.Error(), "both send_text and room_text") {
		t.Fatalf("expected the both-set refusal, got %v", err)
	}
}

func TestValidateRefusesWhitespaceOnlyQuestText(t *testing.T) {
	err := validQuest(ActionDef{SendText: "  "}).Validate()
	if err == nil || !strings.Contains(err.Error(), "send_text") {
		t.Fatalf("expected a send_text refusal, got %v", err)
	}
	q := validQuest(ActionDef{SendText: "You see it."})
	q.Rewards.RoomMessage = " "
	err = q.Validate()
	if err == nil || !strings.Contains(err.Error(), "rewards") {
		t.Fatalf("expected a rewards refusal, got %v", err)
	}
	if err := validQuest(ActionDef{SendText: "You see it."}).Validate(); err != nil {
		t.Fatalf("ordinary text must validate, got %v", err)
	}
}
```

(`QuestStep` and `TriggerDef` are the struct names in `internal/quests/quests.go:61,63,69`.)

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/quests/ -run 'TestActionNarrat|TestRewardNarration|TestValidateRefuses' -count=1`
Expected: FAIL to compile, `ActionDef{...}.Narration undefined`.

- [ ] **Step 3: Implement**

Create `internal/quests/narration.go`:

```go
package quests

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Narration assembles a text action's variants: send_text is the Actor line
// (the triggering player), room_text the Observer line. An action sets one or
// the other; Validate refuses both.
func (a ActionDef) Narration() narration.Variants {
	return narration.Variants{Actor: textutil.Pool(a.SendText), Observer: textutil.Pool(a.RoomText)}
}

// Narrate renders a text action with the triggering player as {source}.
// A quest has no target, so {target} renders empty; RoomTextProblems refuses
// it in room_text for that reason.
func (a ActionDef) Narrate(ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(a.Narration(), ctx)
}

// Narration assembles a reward's variants: playermessage is the Actor line,
// roommessage the Observer line.
func (r QuestReward) Narration() narration.Variants {
	return narration.Variants{Actor: textutil.Pool(r.PlayerMessage), Observer: textutil.Pool(r.RoomMessage)}
}

// Narrate renders the reward lines with the completing player as {source}.
func (r QuestReward) Narrate(ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(r.Narration(), ctx)
}

// validateNarration refuses an action that sets both send_text and room_text
// (one narration per action; ExecuteAction used to drop the room line of such
// an action silently), and any text line that is whitespace only, in actions,
// nested sequence actions, and the rewards.
func (r *Quest) validateNarration() error {
	var problems []string
	var walk func(where string, actions []ActionDef)
	walk = func(where string, actions []ActionDef) {
		for j, a := range actions {
			aw := fmt.Sprintf("%s action %d", where, j)
			if a.SendText != "" && a.RoomText != "" {
				problems = append(problems, aw+": sets both send_text and room_text; an action narrates one line")
			}
			if a.SendText != "" || a.RoomText != "" {
				if err := narration.ValidateVariants(a.Narration(), 1); err != nil {
					key := "send_text"
					if a.SendText == "" {
						key = "room_text"
					}
					problems = append(problems, aw+" "+key+": "+err.Error())
				}
			}
			if a.Sequence != nil {
				walk(aw+" sequence on_complete", a.Sequence.OnComplete)
			}
		}
	}
	for i, t := range r.Triggers {
		walk(fmt.Sprintf("trigger %d", i), t.Actions)
	}
	if r.Rewards.PlayerMessage != "" || r.Rewards.RoomMessage != "" {
		if err := narration.ValidateVariants(r.Rewards.Narration(), 1); err != nil {
			problems = append(problems, "rewards: "+err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("quest %d (%s) narration problems:\n%s", r.QuestId, r.Name, strings.Join(problems, "\n"))
	}
	return nil
}
```

In `quests.go`'s `Validate`, after the `validateRoomText` call:

```go
	// One line per text action, and no whitespace-only line; see narration.go.
	if err := r.validateNarration(); err != nil {
		return err
	}
```

- [ ] **Step 4: Run to verify they pass, and that shipped data still loads**

Run: `go test ./internal/quests/ ./internal/questengine/ ./internal/narration/ -count=1`
Expected: all `ok` (the narration harness loads every dogmud quest through `Validate`, so the both-set rule is proven to refuse nothing shipped).

- [ ] **Step 5: Commit**

```bash
git add internal/quests/narration.go internal/quests/narration_test.go internal/quests/quests.go
git commit -m "feat(quests): action and reward narration doors; refuse an action that sets both texts"
```

---

### Task 6: The buff sites

Four hook files and three silent-start appliers render through the door. Every delivery keeps today's category, channel and exclusion. No field of the spec is read outside `internal/buffs` after this task.

**Files:**
- Modify: `internal/hooks/Buff_ApplyBuffs.go:93-148`
- Modify: `internal/hooks/NewRound_UserRoundTick.go:277-294`
- Modify: `internal/hooks/NewTurn_PruneBuffs.go:42-64,105-126`
- Modify: `internal/hooks/NewRound_MobRoundTick.go:264-278`
- Modify: `internal/hooks/Position_Messaging.go:377-392`
- Modify: `internal/actions/sleep.go:71-78`
- Modify: `internal/justice/arrest.go:403-411`

- [ ] **Step 1: Buff_ApplyBuffs.go**

Replace the block from `startUser := buffInfo.StartUserNotice()` through the closing brace of `if charName != "" { ... }` with:

```go
	startText := buffInfo.Narration(buffs.PhaseStart)
	holderCanRead := evt.UserId != 0 && len(startText.Actee) > 0
	if !wasAlreadyActive && (holderCanRead || len(startText.Observer) > 0) {
		var charName, charPlainName string
		var holder *users.UserRecord
		var roomId, excludeId int

		if evt.UserId != 0 {
			if u := users.GetByUserId(evt.UserId); u != nil {
				charName = u.Character.GetCharacterName(true)
				charPlainName = u.Character.GetCharacterName(false)
				roomId = u.Character.RoomId
				excludeId = u.UserId
				holder = u
			}
		} else if evt.MobInstanceId != 0 {
			if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
				// The mob tag, not the player one. GetCharacterName(true) tags
				// every name `username`, so a mob holder rendered in the player
				// colour. mobDisplayName is what the spell code already uses.
				charName = m.Character.GetCharacterName(true)
				if r := rooms.LoadRoom(m.Character.RoomId); r != nil {
					charName = mobDisplayName(m, r, 0)
				}
				charPlainName = m.Character.GetCharacterName(false)
				roomId = m.Character.RoomId
			}
		}

		if charName != "" {
			roles := buffInfo.Narrate(buffs.PhaseStart, textutil.TokenContext{
				SourceName:      charName,
				SourcePlainName: charPlainName,
			})
			// The holder is the ACTEE: the buff happens to them. A mob holder
			// has no client, so its line is rendered and dropped.
			if roles.Actee != "" && holder != nil {
				holder.SendText(messaging.CategoryBuffApply, roles.Actee)
			}
			// Visual, not audio. Start text describes what the room SEES
			// ("A warm glow surrounds Alice"), and Room.SendText is never
			// sight-gated, so it reached blind and unsighted observers. M2
			// fixed the same defect for cast_room_text.
			if roles.Observer != "" {
				if r := rooms.LoadRoom(roomId); r != nil {
					r.SendTextVisual(messaging.CategoryBuffApply, roles.Observer, excludeId)
				}
			}
		}
	}
```

Keep the comment block above it ("Send the start notice ...") as it is.

- [ ] **Step 2: NewRound_UserRoundTick.go**

Replace from `// Send YAML trigger text (if defined).` through the closing brace of that `if trigBuffSpec != nil ...` block with:

```go
						// Send YAML trigger text (if defined).
						trigBuffSpec := buffs.GetBuffSpec(buff.BuffId)
						if trigBuffSpec != nil && trigBuffSpec.Narration(buffs.PhaseTrigger).Len() > 0 {
							roles := trigBuffSpec.Narrate(buffs.PhaseTrigger, textutil.TokenContext{
								SourceName:      user.Character.GetCharacterName(true),
								SourcePlainName: user.Character.GetCharacterName(false),
							})
							if roles.Actee != "" {
								user.SendText(messaging.CategoryBuffApply, roles.Actee)
							}
							if roles.Observer != "" {
								if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
									r.SendTextVisual(messaging.CategoryBuffApply, roles.Observer, user.UserId) // visual: see Buff_ApplyBuffs.go start text
								}
							}
						}
```

- [ ] **Step 3: NewTurn_PruneBuffs.go, player branch**

Replace from `endBuffSpec := buffs.GetBuffSpec(buffInfo.BuffId)` through the closing brace of `if endBuffSpec != nil && (endUser != "" || ...) { ... }` with:

```go
						endBuffSpec := buffs.GetBuffSpec(buffInfo.BuffId)
						if endBuffSpec != nil && endBuffSpec.Narration(buffs.PhaseEnd).Len() > 0 {
							roles := endBuffSpec.Narrate(buffs.PhaseEnd, textutil.TokenContext{
								SourceName:      user.Character.GetCharacterName(true),
								SourcePlainName: user.Character.GetCharacterName(false),
							})
							if roles.Actee != "" {
								user.SendText(messaging.CategoryBuffExpire, roles.Actee)
							}
							if roles.Observer != "" {
								if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
									sendBuffEndRoomText(r, endBuffSpec, roles.Observer, user.UserId)
								}
							}
						}
```

Keep the "Send the end notice" comment above it.

- [ ] **Step 4: NewTurn_PruneBuffs.go, mob branch**

Replace from `endBuffSpec := buffs.GetBuffSpec(buffInfo.BuffId)` (the second one) through the closing brace of `if endBuffSpec != nil && endBuffSpec.EndRoomText != "" { ... }` with:

```go
				endBuffSpec := buffs.GetBuffSpec(buffInfo.BuffId)
				if endBuffSpec != nil && len(endBuffSpec.Narration(buffs.PhaseEnd).Observer) > 0 {
					// The mob tag, not the player one: see Buff_ApplyBuffs.go.
					// Visual, not audio, for the same reason as start text. The
					// holder line is rendered and dropped: a mob has no client.
					sourceName := mob.Character.GetCharacterName(true)
					if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
						sourceName = mobDisplayName(mob, r, 0)
					}
					roles := endBuffSpec.Narrate(buffs.PhaseEnd, textutil.TokenContext{
						SourceName:      sourceName,
						SourcePlainName: mob.Character.GetCharacterName(false),
					})
					if roles.Observer != "" {
						if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
							sendBuffEndRoomText(r, endBuffSpec, roles.Observer)
						}
					}
				}
```

- [ ] **Step 5: NewRound_MobRoundTick.go**

Replace from `if !buff.Expired() {` through its closing brace with:

```go
			if !buff.Expired() {
				if trigSpec := buffs.GetBuffSpec(buff.BuffId); trigSpec != nil && len(trigSpec.Narration(buffs.PhaseTrigger).Observer) > 0 {
					if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
						roles := trigSpec.Narrate(buffs.PhaseTrigger, textutil.TokenContext{
							SourceName:      mobDisplayName(mob, room, 0),
							SourcePlainName: mob.Character.GetCharacterName(false),
						})
						if roles.Observer != "" {
							room.SendTextVisual(messaging.CategoryBuffApply, roles.Observer)
						}
					}
				}
			}
```

Keep the comment above it ("Trigger text. The player round tick has always sent it ...").

- [ ] **Step 6: The three silent-start appliers**

`internal/hooks/Position_Messaging.go`, body of `sendSilentStartText` from `spec := buffs.GetBuffSpec(buffId)` to the end:

```go
	spec := buffs.GetBuffSpec(buffId)
	if spec == nil {
		return
	}
	line := spec.AuthoredStartLine(textutil.TokenContext{
		SourceName:      u.Character.GetCharacterName(true),
		SourcePlainName: u.Character.GetCharacterName(false),
	})
	if line == "" {
		return
	}
	u.SendText(messaging.CategoryBuffApply, line)
```

Update its doc comment: "Reads the authored start line through `AuthoredStartLine`, not `StartUserNotice()`, which is empty by design for a silent-start buff." Add `"github.com/GoMudEngine/GoMud/internal/textutil"` to the file's imports.

`internal/actions/sleep.go`, the `if actor.IsPlayer() { ... }` block:

```go
	if actor.IsPlayer() {
		if spec := buffs.GetBuffSpec(15); spec != nil {
			name := actor.GetName()
			if line := spec.AuthoredStartLine(textutil.TokenContext{SourceName: name, SourcePlainName: name}); line != "" {
				actor.SendText(messaging.CategoryBuffApply, line)
			}
		}
	}
```

Update the comment above it to name `AuthoredStartLine`. Add the `textutil` import.

`internal/justice/arrest.go`, inside `if u := users.GetByUserId(userId); u != nil {`:

```go
		if spec := buffs.GetBuffSpec(jailedBuffId); spec != nil {
			line := spec.AuthoredStartLine(textutil.TokenContext{
				SourceName:      u.Character.GetCharacterName(true),
				SourcePlainName: u.Character.GetCharacterName(false),
			})
			if line != "" {
				u.SendText(messaging.CategoryBuffApply, line)
			}
		}
```

Add the `textutil` import.

- [ ] **Step 7: Build, run the hook tests, the goldens, and the buff-apply guard**

Run: `go build ./... && go vet ./internal/hooks/ ./internal/actions/ ./internal/justice/`
Expected: clean.

Run: `go test ./internal/hooks/ -count=1`
Expected: `ok`. The 20 buff narration tests in `buff_room_text_test.go` and `buff_notice_test.go` assert delivered lines and are the behaviour proof for these sites.

Run: `go test ./internal/narration/ -count=1`
Expected: `ok` (goldens untouched; nothing in the builders changed yet).

Run: `go test . -run 'TestBuffApplyPath|TestBuffNotice' -count=1`
Expected: `ok`. If the allowlist keyed on a LINE NUMBER in `sleep.go` or `arrest.go` moved (`internal/actions/sleep.go|60`, `internal/justice/arrest.go|395`, `|634`), the guard names the new line; update the key to the new line number in `buff_apply_path_guard_test.go` in this same commit.

Also run: `gofmt -l ./internal`
Expected: nothing printed.

- [ ] **Step 8: Commit**

```bash
git add internal/hooks/Buff_ApplyBuffs.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewTurn_PruneBuffs.go internal/hooks/NewRound_MobRoundTick.go internal/hooks/Position_Messaging.go internal/actions/sleep.go internal/justice/arrest.go
git commit -m "refactor(buffs): every buff phase narrates through the store door"
```
(add `buff_apply_path_guard_test.go` to the `git add` if its line keys moved.)

---

### Task 7: The spell sites

**Files:**
- Modify: `internal/hooks/spell_resolution.go:205-233`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:601-617,745-761`
- Modify: `internal/mobcommands/aid.go:60-77`
- Modify: `internal/mobcommands/cast.go:118-144`
- Modify: `internal/usercommands/skill.cast.go:314-349`

- [ ] **Step 1: spell_resolution.go, magic text**

Replace from `if spellData != nil && (spellData.MagicUserText != "" || spellData.MagicRoomText != "") {` through its closing brace with:

```go
	if spellData != nil && spellData.Narration(spells.PhaseMagic).Len() > 0 {
		tCtx := textutil.TokenContext{
			SourceName:      user.Character.GetCharacterName(true),
			SourcePlainName: user.Character.GetCharacterName(false),
		}
		if len(cs.TargetUserIds) > 0 {
			if tUser := users.GetByUserId(cs.TargetUserIds[0]); tUser != nil {
				tCtx.TargetName = tUser.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tUser.Character.GetCharacterName(false)
			}
		} else if len(cs.TargetMobInstanceIds) > 0 {
			if tMob := mobs.GetInstance(cs.TargetMobInstanceIds[0]); tMob != nil {
				tCtx.TargetName = tMob.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tMob.Character.GetCharacterName(false)
			}
		}
		roles := spellData.Narrate(spells.PhaseMagic, tCtx)
		if roles.Actor != "" {
			user.SendText(spellSchoolCategory(spellData), roles.Actor)
		}
		// Audio channel, as before this refactor: filed, not changed here.
		if roles.Observer != "" {
			if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
				r.SendText(spellSchoolCategory(spellData), roles.Observer, user.UserId)
			}
		}
	}
```

(`spells` and `textutil` are already imported by every file in this task.)

- [ ] **Step 2: NewRound_DoCombat_helpers.go, both wait-text blocks**

`case result.CastComplete:` block, replace from `if spellData != nil && (spellData.WaitUserText != "" || spellData.WaitRoomText != "") {` through its closing brace with:

```go
		if spellData != nil && spellData.Narration(spells.PhaseWait).Len() > 0 {
			roles := spellData.Narrate(spells.PhaseWait, textutil.TokenContext{
				SourceName:      user.Character.GetCharacterName(true),
				SourcePlainName: user.Character.GetCharacterName(false),
			})
			if roles.Actor != "" {
				user.SendText(messaging.CategorySpellFold, roles.Actor)
			}
			// Audio channel, as before this refactor: filed, not changed here.
			if roles.Observer != "" {
				if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
					r.SendText(messaging.CategorySpellFold, roles.Observer, user.UserId)
				}
			}
		}
```

`case result.StillCasting:` block, replace from `if waitSpellInfo != nil && (waitSpellInfo.WaitUserText != "" || ...) {` through its closing brace with the same code using `waitSpellInfo` in place of `spellData`.

- [ ] **Step 3: mobcommands/aid.go**

Replace from `if spellInfo != nil && (spellInfo.CastUserText != "" || spellInfo.CastRoomText != "") {` through its closing brace with:

```go
	if spellInfo != nil && spellInfo.Narration(spells.PhaseCast).Len() > 0 {
		castRoom := rooms.LoadRoom(mob.Character.RoomId)
		roles := spellInfo.Narrate(spells.PhaseCast, textutil.TokenContext{
			SourceName:      mob.Character.GetCharacterName(true),
			SourcePlainName: mob.Character.GetCharacterName(false),
			TargetName:      p.Character.GetCharacterName(true),
			TargetPlainName: p.Character.GetCharacterName(false),
		})
		// A mob caster has no client: its own line is rendered and dropped.
		if roles.Observer != "" && castRoom != nil {
			castRoom.SendTextVisual(messaging.CategorySpellVital, roles.Observer)
		}
	}
```

- [ ] **Step 4: mobcommands/cast.go**

Replace from `if spellInfo.CastUserText != "" || spellInfo.CastRoomText != "" {` through its closing brace with:

```go
	if spellInfo.Narration(spells.PhaseCast).Len() > 0 {
		castRoom := rooms.LoadRoom(mob.Character.RoomId)
		tCtx := textutil.TokenContext{
			SourceName:      mob.Character.GetCharacterName(true),
			SourcePlainName: mob.Character.GetCharacterName(false),
		}
		if len(result.TargetUserIds) > 0 {
			if tUser := users.GetByUserId(result.TargetUserIds[0]); tUser != nil {
				tCtx.TargetName = tUser.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tUser.Character.GetCharacterName(false)
			}
		} else if len(result.TargetMobInstanceIds) > 0 {
			if tMob := mobs.GetInstance(result.TargetMobInstanceIds[0]); tMob != nil {
				tCtx.TargetName = tMob.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tMob.Character.GetCharacterName(false)
			}
		}
		roles := spellInfo.Narrate(spells.PhaseCast, tCtx)
		// A mob caster has no client: its own line is rendered and dropped.
		if roles.Observer != "" && castRoom != nil {
			castRoom.SendTextVisual(messaging.CategorySpellFold, roles.Observer)
		}
	}
```

- [ ] **Step 5: usercommands/skill.cast.go**

Replace from `if spellInfo.CastUserText != "" || spellInfo.CastRoomText != "" {` through its closing brace with:

```go
	if spellInfo.Narration(spells.PhaseCast).Len() > 0 {
		castRoom := rooms.LoadRoom(user.Character.RoomId)
		tCtx := textutil.TokenContext{
			SourceName:      user.Character.GetCharacterName(true),
			SourcePlainName: user.Character.GetCharacterName(false),
		}
		if len(result.TargetUserIds) > 0 {
			if tUser := users.GetByUserId(result.TargetUserIds[0]); tUser != nil {
				tCtx.TargetName = tUser.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tUser.Character.GetCharacterName(false)
			}
		} else if len(result.TargetMobInstanceIds) > 0 {
			if tMob := mobs.GetInstance(result.TargetMobInstanceIds[0]); tMob != nil {
				tCtx.TargetName = tMob.Character.GetCharacterName(true)
				tCtx.TargetPlainName = tMob.Character.GetCharacterName(false)
			}
		}
		roles := spellInfo.Narrate(spells.PhaseCast, tCtx)
		if roles.Actor != "" {
			user.SendText(messaging.CategorySpellFold, roles.Actor)
		}
		// SendTextVisual, not SendText: a cast_room_text describes what the
		// room SEES ("a fierce glow building"), and the audio channel is never
		// sight-gated, so this reached blind observers with the caster name
		// and the visual detail. Found by the 2026-09-08 darkness playtest.
		if roles.Observer != "" && castRoom != nil {
			castRoom.SendTextVisual(messaging.CategorySpellFold, roles.Observer, user.UserId)
		}
	}
```

Keep the `// 12b. Send YAML cast text (if defined).` comment.

- [ ] **Step 6: Build and test**

Run: `go build ./... && gofmt -l ./internal`
Expected: clean, nothing printed.

Run: `go test ./internal/hooks/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/narration/ -count=1`
Expected: all `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/spell_resolution.go internal/hooks/NewRound_DoCombat_helpers.go internal/mobcommands/aid.go internal/mobcommands/cast.go internal/usercommands/skill.cast.go
git commit -m "refactor(spells): every spell phase narrates through the store door"
```

---

### Task 8: The quest sites

**Files:**
- Modify: `internal/questengine/actions.go:26-27,71-77`
- Modify: `internal/questengine/bridge.go:201-233`
- Modify: `internal/questengine/actions_test.go:65-68`
- Modify: `internal/hooks/Quest_HandleQuestUpdate.go:260-269`

- [ ] **Step 1: Update the mock first, so the interface change is test-driven**

In `actions_test.go` replace the `SendText` and `RoomText` methods with:

```go
// Narrate records the authored lines by role, so existing assertions on
// sentTexts and roomTexts keep reading the send_text and room_text an action
// carried.
func (m *mockActionContext) Narrate(v narration.Variants) {
	m.sentTexts = append(m.sentTexts, v.Actor...)
	m.roomTexts = append(m.roomTexts, v.Observer...)
}
```

Add `"github.com/GoMudEngine/GoMud/internal/narration"` to its imports and remove `messaging`: the deleted `SendText` signature was the file's only use of it.

Run: `go test ./internal/questengine/ -count=1`
Expected: FAIL to compile: the mock no longer satisfies `ActionContext` (`missing method SendText`).

- [ ] **Step 2: Change the interface and the dispatch**

In `actions.go`, replace the two interface lines

```go
	SendText(cat messaging.Category, text string)
	RoomText(text string)
```

with

```go
	// Narrate delivers a text action: the Actor line to the triggering player,
	// the Observer line to the room. The implementation renders the tokens.
	Narrate(v narration.Variants)
```

and replace the two dispatch branches

```go
	if a.SendText != "" {
		ctx.SendText(messaging.CategoryNPCDialogue, a.SendText)
		return nil
	}
	if a.RoomText != "" {
		ctx.RoomText(a.RoomText)
		return nil
	}
```

with

```go
	if a.SendText != "" || a.RoomText != "" {
		ctx.Narrate(a.Narration())
		return nil
	}
```

Fix imports: add `narration`; remove `messaging`, whose only two uses in `actions.go` were the interface line and the dispatch branch just deleted.

- [ ] **Step 3: The bridge**

In `bridge.go`, delete the `SendText` and `RoomText` methods (lines 201-233 today) and add:

```go
// Narrate delivers a text action to its audiences.
//
// Both lines are rendered with the triggering player's TAGGED name as
// {source}, and the room line goes out on the VISUAL channel; both matter.
// The room line describes something the room watches the player do
// ("{source} unlocks the strongbox"), so an observer who cannot see must not
// receive it, and Room.SendText is never sight-gated. And the name must carry
// its `username` tag, because messaging.Anonymize strips only tagged names.
// quests.Quest.Validate keeps every room_text naming {source}.
//
// send_text is substituted too since M3 item 5b. No shipped send_text carries
// a token, so nothing changed on the day; a future line naming {source} now
// renders the name instead of the literal, the defect quest 77 showed.
func (b *GameBridge) Narrate(v narration.Variants) {
	roles := textutil.Narrate(v, textutil.TokenContext{
		SourceName:      b.user.Character.GetCharacterName(true),
		SourcePlainName: b.user.Character.GetCharacterName(false),
	})
	if roles.Actor != "" {
		b.user.SendText(messaging.CategoryNPCDialogue, roles.Actor)
	}
	if roles.Observer != "" {
		room := rooms.LoadRoom(b.roomId)
		if room == nil {
			mudlog.Error("GameBridge.Narrate", "error", fmt.Sprintf("room %d not found", b.roomId))
			return
		}
		room.SendTextVisual(messaging.CategoryNPCDialogue, roles.Observer, b.user.UserId)
	}
}
```

Add the `narration` import.

- [ ] **Step 4: The reward hook**

In `Quest_HandleQuestUpdate.go`, replace

```go
		// Message to player?
		if len(questInfo.Rewards.PlayerMessage) > 0 {
			questUser.SendText(messaging.CategorySystem, questInfo.Rewards.PlayerMessage)
		}
		// Message to room?
		if len(questInfo.Rewards.RoomMessage) > 0 {
			if room := rooms.LoadRoom(questUser.Character.RoomId); room != nil {
				sendVisualRoomText(room, messaging.CategoryEmote, questInfo.Rewards.RoomMessage, questUser.UserId)
			}
		}
```

with

```go
		// Reward messages, through the quest store's door. Shipped rewards
		// carry no token, so substitution changes nothing today.
		rewardLines := questInfo.Rewards.Narrate(textutil.TokenContext{
			SourceName:      questUser.Character.GetCharacterName(true),
			SourcePlainName: questUser.Character.GetCharacterName(false),
		})
		if rewardLines.Actor != "" {
			questUser.SendText(messaging.CategorySystem, rewardLines.Actor)
		}
		if rewardLines.Observer != "" {
			if room := rooms.LoadRoom(questUser.Character.RoomId); room != nil {
				sendVisualRoomText(room, messaging.CategoryEmote, rewardLines.Observer, questUser.UserId)
			}
		}
```

Add `"github.com/GoMudEngine/GoMud/internal/textutil"` to the imports.

- [ ] **Step 5: Build and test**

Run: `go build ./... && gofmt -l ./internal`
Expected: clean.

Run: `go test ./internal/questengine/ ./internal/quests/ ./internal/hooks/ ./internal/narration/ -count=1`
Expected: all `ok`. `engine_test.go:183` and `actions_test.go:179` still assert on `sentTexts`, now fed by `Narrate`.

- [ ] **Step 6: Commit**

```bash
git add internal/questengine/actions.go internal/questengine/bridge.go internal/questengine/actions_test.go internal/hooks/Quest_HandleQuestUpdate.go
git commit -m "refactor(quests): one Narrate on the action context; rewards narrate through the store door"
```

---

### Task 9: Delete the old path, switch the goldens to the doors, prove byte-identity

**Files:**
- Delete: `internal/textutil/spelltext.go`
- Modify: `internal/narration/snapshot_test.go` (the three builders)

- [ ] **Step 1: Delete `spelltext.go`**

```bash
git rm internal/textutil/spelltext.go
go build ./...
```
Expected: builds clean. If anything still references `SendPhaseText` or `SendTextConfig`, a site was missed in Task 6, 7 or 8; go back and migrate it the same way.

- [ ] **Step 2: Rewrite the three builders to read through the doors**

In `buildBuffsGolden`, replace the per-buff body (from `rows := ...` to the end of the `for _, id` loop) with:

```go
		spec := buffs.GetBuffSpec(id)
		phases := []struct {
			p                buffs.Phase
			userKey, roomKey string
		}{
			{buffs.PhaseStart, "start_user_text", "start_room_text"},
			{buffs.PhaseTrigger, "trigger_user_text", "trigger_room_text"},
			{buffs.PhaseEnd, "end_user_text", "end_room_text"},
		}
		for _, ph := range phases {
			roles := spec.Narrate(ph.p, kindBNoTarget)
			if roles.Actee != "" {
				fmt.Fprintf(&b, "buff|%d|%s => %s\n", id, ph.userKey, roles.Actee)
			}
			if roles.Observer != "" {
				fmt.Fprintf(&b, "buff|%d|%s => %s\n", id, ph.roomKey, roles.Observer)
			}
		}
		if slices.Contains(spec.Flags, buffs.SilentStart) {
			if line := spec.AuthoredStartLine(kindBNoTarget); line != "" {
				fmt.Fprintf(&b, "buff|%d|authored_start_line => %s\n", id, line)
			}
		}
```

Do NOT change any `fmt.Fprintf(&b, "# ...")` header line: the headers are part of the golden bytes, so rewording one would fail the byte-identity proof this task exists for. Put the note about the switch in a Go comment above the builder instead. (Corrected 2026-09-12 during execution; the first draft of this step said to reword the header.)

In `buildSpellsGolden`, replace the per-spell body with:

```go
		s := all[id]
		phases := []struct {
			p                spells.Phase
			userKey, roomKey string
		}{
			{spells.PhaseCast, "cast_user_text", "cast_room_text"},
			{spells.PhaseWait, "wait_user_text", "wait_room_text"},
			{spells.PhaseMagic, "magic_user_text", "magic_room_text"},
		}
		for _, ph := range phases {
			with := s.Narrate(ph.p, kindBSource)
			without := s.Narrate(ph.p, kindBNoTarget)
			if with.Actor != "" {
				fmt.Fprintf(&b, "spell|%s|%s => %s\n", id, ph.userKey, with.Actor)
				if without.Actor != with.Actor {
					fmt.Fprintf(&b, "spell|%s|%s|notarget => %s\n", id, ph.userKey, without.Actor)
				}
			}
			if with.Observer != "" {
				fmt.Fprintf(&b, "spell|%s|%s => %s\n", id, ph.roomKey, with.Observer)
				if without.Observer != with.Observer {
					fmt.Fprintf(&b, "spell|%s|%s|notarget => %s\n", id, ph.roomKey, without.Observer)
				}
			}
		}
```

In `buildQuestsGolden`, replace the walk body and the rewards lines with:

```go
	walk = func(where string, actions []quests.ActionDef) {
		for j, a := range actions {
			aw := fmt.Sprintf("%s|action%d", where, j)
			roles := a.Narrate(kindBNoTarget)
			if roles.Actor != "" {
				fmt.Fprintf(&b, "%s|send_text => %s\n", aw, roles.Actor)
			}
			if roles.Observer != "" {
				fmt.Fprintf(&b, "%s|room_text => %s\n", aw, roles.Observer)
			}
			if a.Sequence != nil {
				walk(aw+"|sequence", a.Sequence.OnComplete)
			}
		}
	}
	for _, q := range all {
		reward := q.Rewards.Narrate(kindBNoTarget)
		if reward.Actor != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|playermessage => %s\n", q.QuestId, reward.Actor)
		}
		if reward.Observer != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|roommessage => %s\n", q.QuestId, reward.Observer)
		}
		for i, tr := range q.Triggers {
			walk(fmt.Sprintf("quest|%d|trigger%d", q.QuestId, i), tr.Actions)
		}
	}
```

- [ ] **Step 3: The proof**

Run: `go test ./internal/narration/ -run TestSnapshotStores -count=1`
Expected: `ok`, and `git status --short internal/narration/testdata/` prints NOTHING. Ten goldens, all byte-identical, with the builders now reading through the doors. If any golden is red, the door or a site changed behaviour: read the first differing row the failure prints and fix the CODE. Do not run `-update`.

- [ ] **Step 4: Sabotage probe 1, proven red**

In `internal/buffs/narration.go`, temporarily swap the two roles in `Narration`:

```go
	return narration.Variants{Actee: textutil.Pool(room), Observer: textutil.Pool(holder)}
```

Run: `go vet ./internal/buffs/ && go test ./internal/narration/ -run TestSnapshotStores/buffs -count=1`
Expected: `vet` clean (the sabotage compiles), then FAIL naming a `buff|...|start_user_text` row whose text is a room line. Revert the swap, re-run, `ok`. Record in the commit message that the probe was verified red.

- [ ] **Step 5: Commit**

```bash
git add internal/narration/snapshot_test.go
git commit -m "refactor(textutil): delete SendPhaseText; the Kind B goldens read through the store doors and are byte-identical"
```
(`spelltext.go`'s removal is already staged by `git rm`.)

---

### Task 10: Root guards and the viewpoint registry

**Files:**
- Create: `narration_render_callers_guard_test.go` (repo root, `package main`)
- Create: `store_text_fields_guard_test.go` (repo root, `package main`)
- Modify: `messaging_surface_guard_test.go` (registry entries)

- [ ] **Step 1: Guard 1, Render callers**

Create `narration_render_callers_guard_test.go`:

```go
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// narrationRenderCallers is every production file allowed to call
// narration.Render, with why. The Kind B stores (buffs, spells, quests) are
// deliberately ABSENT: they reach the core only through textutil.Narrate,
// which always passes narration.FirstPicker. A store calling Render itself
// could pass the default picker and consume a global random draw per
// narrated phase (see narration.FirstPicker), which no golden can see.
var narrationRenderCallers = map[string]string{
	"internal/combat/taunt_messages.go":      "Kind A: the taunt store's coordinated triad",
	"internal/grapplemessaging/render.go":    "Kind A: the grapple store's coordinated triad",
	"internal/items/defensive_messages.go":   "Kind A: the defence store's coordinated triad",
	"internal/itemvoices/itemvoices.go":      "Kind A: sentient item voices, single role",
	"internal/spells/casting_messages.go":    "Kind A: the caster-only casting pools",
	"internal/textutil/narrate.go":           "the ONE door for the Kind B stores; must pass narration.FirstPicker",
}

var narrationRenderCallRE = regexp.MustCompile(`narration\.Render\(`)

// The textutil door must hand Render FirstPicker, and must never name the
// default picker at all.
var textutilFirstPickerRE = regexp.MustCompile(`(?s)narration\.Render\([^;]*?narration\.FirstPicker\)`)

func TestNarrationRenderIsCalledOnlyByRegisteredStores(t *testing.T) {
	found := map[string]bool{}
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if narrationRenderCallRE.Match(src) {
				found[filepath.ToSlash(path)] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no narration.Render call found anywhere; the walk is broken, not the code")
	}

	var unregistered, stale []string
	for f := range found {
		if _, ok := narrationRenderCallers[f]; !ok {
			unregistered = append(unregistered, f)
		}
	}
	for f := range narrationRenderCallers {
		if !found[f] {
			stale = append(stale, f)
		}
	}
	sort.Strings(unregistered)
	sort.Strings(stale)
	if len(unregistered) > 0 {
		t.Errorf("narration.Render is called from unregistered file(s):\n  %s\n\nA Kind B store (buffs, spells, quests) must render through textutil.Narrate, never Render directly. A new Kind A store registers here with a reason.", strings.Join(unregistered, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("registered Render caller(s) no longer call it:\n  %s\n\nRemove the entry only after confirming the store did not lose its rendering.", strings.Join(stale, "\n  "))
	}

	door, err := os.ReadFile("internal/textutil/narrate.go")
	if err != nil {
		t.Fatalf("read the textutil door: %v", err)
	}
	if !textutilFirstPickerRE.Match(door) {
		t.Errorf("internal/textutil/narrate.go must call narration.Render with narration.FirstPicker as the picker; a single-variant store must not consume a random draw")
	}
	if strings.Contains(string(door), "DefaultPicker") {
		t.Errorf("internal/textutil/narrate.go names DefaultPicker; the Kind B door must never draw")
	}
}
```

- [ ] **Step 2: Guard 2, text fields read only by their store**

Create `store_text_fields_guard_test.go`:

```go
package main

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The Kind B stores own their text. Every buff, spell and quest-reward line is
// rendered through the store's Narrate door, so a site never reads a field
// and never decides for itself which audience a line is for. This guard keeps
// it that way: outside the owning package, no production file may name the
// fields. Quest ACTION fields (ActionDef.SendText / RoomText) are exempt:
// questengine.ExecuteAction tests them to dispatch, and the spellings are too
// common (user.SendText) to match safely.
var storeTextFieldOwners = []struct {
	owner   string
	pattern *regexp.Regexp
	what    string
}{
	{"internal/buffs", regexp.MustCompile(`\.(StartUserText|StartRoomText|TriggerUserText|TriggerRoomText|EndUserText|EndRoomText)\b`), "buff text fields"},
	{"internal/spells", regexp.MustCompile(`\.(CastUserText|CastRoomText|WaitUserText|WaitRoomText|MagicUserText|MagicRoomText)\b`), "spell text fields"},
	{"internal/quests", regexp.MustCompile(`Rewards\.(PlayerMessage|RoomMessage)\b`), "quest reward messages"},
}

func TestStoreTextFieldsAreReadOnlyByTheirStore(t *testing.T) {
	type hit struct{ file string; line int; text string }
	for _, owner := range storeTextFieldOwners {
		var outside []hit
		inside := 0
		for _, root := range messagingSurfaceGoRoots {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				src, rerr := os.ReadFile(path)
				if rerr != nil {
					return rerr
				}
				rel := filepath.ToSlash(path)
				sc := bufio.NewScanner(bytes.NewReader(src))
				sc.Buffer(make([]byte, 1024*1024), 1024*1024)
				n := 0
				for sc.Scan() {
					n++
					if !owner.pattern.MatchString(sc.Text()) {
						continue
					}
					if strings.HasPrefix(rel, owner.owner+"/") {
						inside++
					} else {
						outside = append(outside, hit{rel, n, strings.TrimSpace(sc.Text())})
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walk %s: %v", root, err)
			}
		}
		// The pattern must be proven capable of matching, or an empty
		// "outside" list proves nothing.
		if inside == 0 {
			t.Errorf("%s: pattern matched nothing inside %s; the guard is blind, not the tree clean", owner.what, owner.owner)
		}
		sort.Slice(outside, func(i, j int) bool {
			if outside[i].file != outside[j].file {
				return outside[i].file < outside[j].file
			}
			return outside[i].line < outside[j].line
		})
		for _, h := range outside {
			t.Errorf("%s read outside %s: %s:%d: %s\n  render through the store's Narrate (or AuthoredStartLine) instead", owner.what, owner.owner, h.file, h.line, h.text)
		}
	}
}
```

- [ ] **Step 3: Run both guards**

Run: `go test . -run 'TestNarrationRenderIsCalledOnlyByRegisteredStores|TestStoreTextFieldsAreReadOnlyByTheirStore' -count=1`
Expected: `ok`. If guard 2 reports a reader outside a store, it is a site this plan missed: migrate it the way Task 6 or 7 did, in this commit.

- [ ] **Step 4: Sabotage probe 2, proven red**

In `internal/textutil/narrate.go`, temporarily change `narration.FirstPicker` to `narration.DefaultPicker`. Run `go vet ./internal/textutil/` (must compile) then the guard command above. Expected: FAIL with both the FirstPicker and the DefaultPicker messages. Revert; re-run; `ok`.

Second probe for guard 2: in `internal/hooks/Buff_ApplyBuffs.go` temporarily add `_ = buffInfo.StartRoomText` on a line of its own. `go vet ./internal/hooks/` then the guard: FAIL naming `internal/hooks/Buff_ApplyBuffs.go:<line>`. Revert; re-run; `ok`.

- [ ] **Step 5: The viewpoint registry**

Run: `go test . -run TestNarrationSitesMatchViewpointAudit -count=1`

Expected: FAIL listing newly visible candidate sites (each line prints the key, the source line and the viewpoint booleans), and possibly STALE keys for sites whose literal changed. For each unregistered key add an entry to `narrationViewpointRegistry` in `messaging_surface_guard_test.go`, copying the key exactly as printed and the booleans it printed, with `verdictCorrect` and one of these reasons:

| Site | Reason |
|---|---|
| `hooks/Buff_ApplyBuffs.go` | "buff start narration through the store door (M3 item 5b): the holder reads the actee line and the room the observer line; there is no separate actor until M6 gives the event a caster" |
| `hooks/NewRound_UserRoundTick.go` (trigger) | "buff trigger narration through the store door: holder plus room; no separate party exists" |
| `hooks/NewTurn_PruneBuffs.go` | "buff end narration through the store door: holder plus room; no separate party exists" |
| `hooks/spell_resolution.go` (magic text) | "authored magic_user_text/magic_room_text through the store door: caster plus room; the target is reached by the effect's own narration below" |
| `hooks/NewRound_DoCombat_helpers.go` (wait text, two sites) | "authored wait text through the store door: caster plus room; a channelling round has no actee" |
| `usercommands/skill.cast.go` (cast text) | "authored cast text through the store door: caster plus room; the target is reached by the room line and by resolution" |
| `hooks/Quest_HandleQuestUpdate.go` (rewards) | "quest reward lines through the store door: the completing player plus the room; a reward has no other party" |
| `questengine/bridge.go` (Narrate) | "quest action lines through the store door: the triggering player plus the room; quest YAML has no actee key" |
| `hooks/Position_Messaging.go`, `justice/arrest.go`, `actions/sleep.go` | keep or update the existing reason (sleep's is registered at line 1151 today); the line is now `AuthoredStartLine`, the verdict is unchanged |

For a STALE key (the old sleep entry, whose literal changed), replace it with the new key the guard prints, keeping its reason. Mob-side sites (`NewRound_MobRoundTick.go`, `mobcommands/aid.go`, `mobcommands/cast.go`, the mob branch of `NewTurn_PruneBuffs.go`) send only a room line and should not appear; if one does, register it with "mob holder or mob caster: no client, room line only".

Re-run until `ok`.

- [ ] **Step 6: Full root package and commit**

Run: `go test . -count=1`
Expected: `ok`.

```bash
git add narration_render_callers_guard_test.go store_text_fields_guard_test.go messaging_surface_guard_test.go
git commit -m "test: root guards pin the Kind B render door and the store-only field reads; register the newly visible sites"
```

---

### Task 11: Documentation

**Files:**
- Modify: `internal/narration/context.md`, `internal/textutil/context.md`, `internal/buffs/context.md`, `internal/spells/context.md`, `internal/quests/context.md`, `internal/questengine/context.md`
- Modify: `docs/README.md`, the spec

- [ ] **Step 1: `internal/narration/context.md`**

In the Public API block add after `SequencePicker`:

```go
func FirstPicker(n int) int
```

Add a Gotcha at the end of the Gotchas section:

```markdown
**A single-variant store renders with `FirstPicker`, never the default.**
`Render` always calls `pick(n)`, `DefaultPicker` always calls `util.Rand`, and
`util.Rand(1)` still calls `rand.Intn`, so a one-line pool rendered through the
default picker consumes a global random draw and shifts every later combat
roll. `Render` cannot special-case `n == 1` because itemvoices never validates
its pool sizes and legitimately holds one-line pools whose draw count must not
change. The buff, spell and quest stores reach `Render` only through
`textutil.Narrate`, which passes `FirstPicker`; the root guard
`narration_render_callers_guard_test.go` pins both facts.
```

In Consumers, add: "`internal/textutil` (the door for the buff, spell and quest stores, which do not call `Render` themselves)."

- [ ] **Step 2: `internal/textutil/context.md`**, replace the whole file:

```markdown
# Text Utility Context

## Purpose

`internal/textutil` is the thin adapter between the single-string narration
stores (buffs, spells, quests: one authored line per lifecycle phase and
audience) and the rendering core in `internal/narration`. It owns the
four-token vocabulary those stores are written in and the one function they
render through.

## Files

- **tokens.go**: `TokenContext`, `Tokens`, `SubstituteTokens`, `ValidateTokens`.
- **narrate.go**: `Pool`, `Narrate`.

## API

```go
type TokenContext struct { SourceName, SourcePlainName, TargetName, TargetPlainName string }

func (ctx TokenContext) Tokens() map[string]string     // all four keys, always
func Pool(text string) []string                        // nil for "", else one variant
func Narrate(v narration.Variants, ctx TokenContext) narration.Roles
func SubstituteTokens(text string, ctx TokenContext) string
func ValidateTokens(text string) []string
```

A store assembles its `narration.Variants` (which line is Actor, Actee,
Observer) and calls `Narrate`; the site delivers each role on its own channel.
`SubstituteTokens` is a one-variant `Narrate`, kept for the dialogue store,
which the messaging arc migrates in its item 7.

## Gotchas

- **`Narrate` always passes `narration.FirstPicker`.** A single-variant store
  has nothing to choose, and the default picker would consume a global random
  draw per narrated phase (`util.Rand(1)` still draws). The root guard
  `narration_render_callers_guard_test.go` fails the build if this file names
  the default picker or if a store calls `narration.Render` itself.
- **`Tokens` always carries all four keys**, so an absent target renders as an
  empty string. That is what `SubstituteTokens` has always done; a line that
  names `{target}` with no target has a hole in it.
- **A misspelled token substitutes to nothing and does not error.**
  `ValidateTokens` exists to catch that at load; buffs and spells only WARN on
  it today, quests fail (`internal/quests/roomtext.go`).
- **`Pool` keeps whitespace.** A whitespace-only authored line must reach
  `narration.ValidateVariants` and be refused at load, not trimmed into
  silence.
- **`SendPhaseText` is gone** (deleted 2026-09-12, messaging M3 item 5b). Sites
  render through the store's `Narrate` door and deliver inline; delivery
  through `messaging.SendTrio` is the arc's M4.

## Dependencies

`internal/narration` only.

## Consumers

`internal/buffs`, `internal/spells`, `internal/quests` (their `Narrate` doors and
validators), `internal/questengine` (the bridge), the hook, command and action
sites that build a `TokenContext`, and `internal/behaviortree` (dialogue, via
`SubstituteTokens`).
```

- [ ] **Step 3: `internal/buffs/context.md`**

Add a new subsection after "The player-side notice (slice C, `notice.go`)":

```markdown
### The narration door (M3 item 5b, `narration.go`)

- `Phase` (`PhaseStart`, `PhaseTrigger`, `PhaseEnd`) selects the moment.
- `Narration(p Phase) narration.Variants`: the holder's line is the **Actee**
  (the buff happens to them; owner ruling 2026-09-12) and the room's line the
  Observer. Actor is empty and reserved for the caster, which M6 authors once
  `events.Buff` carries one. Start and End go through `StartUserNotice` /
  `EndUserNotice`, so the notice rules stay in their one door.
- `Narrate(p Phase, ctx textutil.TokenContext) narration.Roles` renders it. This
  is what `Buff_ApplyBuffs`, both round ticks and `NewTurn_PruneBuffs` call;
  they deliver each role themselves on today's category and channel.
- `AuthoredStartLine(ctx) string` renders `start_user_text` as written, ignoring
  the notice rules: the door for a silent-start buff's applier (sleep 15,
  arrest 88, stun 84, broken limb 83).
- `Validate` runs `narration.ValidateVariants` over every authored phase, so a
  whitespace-only line fails the load.
- **No file outside this package reads the six text fields.** The root guard
  `store_text_fields_guard_test.go` fails the build on one.
```

Also fix the sentence in the notice subsection that says "`actions.Sleep` reads buff 15's `StartUserText` directly and sends it" to "`actions.Sleep` sends buff 15's line through `AuthoredStartLine`".

- [ ] **Step 4: `internal/spells/context.md`**

Add after the casting-messages section:

```markdown
### The narration door (M3 item 5b, `narration.go`)

`Phase` (`PhaseCast`, `PhaseWait`, `PhaseMagic`); `Narration(p)` puts
`*_user_text` in Actor and `*_room_text` in Observer (Actee is authored in M6);
`Narrate(p, ctx)` renders through `textutil.Narrate`. The cast command (player
and mob), the two wait-text sites in `NewRound_DoCombat_helpers.go`, the magic
text in `spell_resolution.go` and the mob `aid` command all render through it
and deliver on their own channel. `Validate` refuses a whitespace-only line.
No file outside this package reads the six text fields (root guard
`store_text_fields_guard_test.go`). The two shipped `wait_room_text` lines still
go out on the audio channel; that is filed, not a property of the door.
```

- [ ] **Step 5: `internal/quests/context.md` and `internal/questengine/context.md`**

In `quests/context.md`, after the Architecture section's core components, add:

```markdown
### Narration (M3 item 5b, `narration.go`)

`ActionDef.Narration()` (send_text = Actor, room_text = Observer) and
`QuestReward.Narration()` (playermessage = Actor, roommessage = Observer), each
with a `Narrate(ctx)` that renders through `textutil.Narrate`. `Validate`
refuses an action that sets both texts (one narration per action) and any
whitespace-only line, alongside the `room_text` rule in `roomtext.go`.
```

In `questengine/context.md`, replace the paragraph beginning "`RoomText` substitutes `{source}`" with:

```markdown
`ActionContext.Narrate(v narration.Variants)` is the one door for a text action
(since 2026-09-12; it replaced `SendText` and `RoomText`). `ExecuteAction` calls
it with `a.Narration()`; `GameBridge.Narrate` renders with the triggering
player's **tagged** name as `{source}`, sends the Actor line to the player and
the Observer line on the **visual** channel, the way the behaviour tree's own
`room_text` does. The tag matters: `messaging.Anonymize` strips only tagged
names. `send_text` is substituted too; no shipped line carries a token.
```

Also update the file list line for `actions.go` and `bridge.go` if it names `SendText`/`RoomText`.

- [ ] **Step 6: The spec**

(The `docs/README.md` rows for the spec and this plan already exist; new files created by this plan are Go source and goldens, which the README does not list.)

In the spec's "The adapter" section, add `Pool(text string) []string` to the API block with the comment "nil for an empty string, else one variant; whitespace kept so the validator can refuse it".

- [ ] **Step 7: Audit and commit**

Run: `python tools/context_md_audit.py`
Expected: no phantom symbols reported for narration, textutil, buffs, spells, quests, questengine.

```bash
git add internal/narration/context.md internal/textutil/context.md internal/buffs/context.md internal/spells/context.md internal/quests/context.md internal/questengine/context.md docs/superpowers/specs/2026-09-12-messaging-m3-item5b-kind-b-store-migration-design.md
git commit -m "docs: the Kind B narration doors, the textutil adapter and FirstPicker"
```

---

### Task 12: Full verification, playtest lane, PR

- [ ] **Step 1: The suite**

Run: `gofmt -l ./internal ./cmd .` (nothing printed), then `go vet ./...`, then `go test ./... -count=1`.
Expected: every package `ok`. If `internal/playtestrun` is the only red, run `go test ./internal/playtestrun/ -count=1` standalone; it reds under CPU load and passes alone.

- [ ] **Step 2: The boot check**

Follow `dogmud-shipping`'s detached-worktree boot check. Expected: the server boots, `buffs.LoadDataFiles`, `spells.LoadSpellFiles` and `quests.LoadDataFiles` log their counts, no panic from `validateNarration` on either world.

- [ ] **Step 3: The playtest lane**

Load the `dogmud-playtesting` skill, then run 5a's lit lane against this branch's HEAD:

```bash
go run ./cmd/playtestrun scenario --checkout "C:/Users/Calabe Davis/workspace/DOGMud" --scenario tools/playtest/scenarios/m3-item5a-lit.yaml
```

Read the run id from `tools/playtest/.run/<run_id>/session.json` (a piped `tail` hides it), then read BOTH bridges' `events.jsonl` directly. The check is equality against the 5a lane A record, verbatim:

| Reader | Expected line |
|---|---|
| witness, quest | `Veteran Pathfinder works something free of the cracked floor and pockets it.` |
| actor, quest | the quest's send_text and `You receive a Inert Disc.` (the a/an is pre-existing) |
| witness, glow | `Chrysalis Glow settles over Veteran Pathfinder.` then `A warm glow surrounds Veteran Pathfinder.` |
| actor, glow | `Your Chrysalis Glow takes effect.` then `A warm glow surrounds you.` |
| witness, purge | `Veteran Pathfinder's Cleansing Wave purges the toxins from your body.` and room `Cleansing Wave cleanses Veteran Pathfinder of afflictions.` |
| actor, purge | own purge line exactly once |
| witness self-heal | `A warm glow of healing magic envelops you. Your wounds begin to mend.` once; actor reads `Ordel Quist channels restorative magic.` |

Any difference is a defect in this branch (the lines are goldens of a kind). Tear down with `docker rm -f dogmud-playtest-<run_id>-server-1`; `playtestrun stop` exits 0 and does nothing. Extract the findings to memory; the report is gitignored.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feature/messaging-m3-item5b-kind-b-migration
gh pr create --repo pruuk/DOGMud --title "Messaging M3 item 5b: buffs, spells and quests onto the narration core" --body-file - <<'EOF'
Byte-identical migration of the three Kind B stores onto `internal/narration`.

- `narration.FirstPicker`: a single-variant store renders without a random draw (`util.Rand(1)` still draws).
- `textutil` is the adapter: `Tokens`, `Pool`, `Narrate`; `SubstituteTokens` runs over the core; `SendPhaseText` deleted.
- buffs, spells, quests each gain `Phase` / `Narration` / `Narrate` doors; holder line is the Actee (owner ruling), caster slot reserved for M6.
- 12 sites, the quest bridge (`ActionContext.Narrate`), the reward hook and the three silent-start appliers render through the doors and deliver exactly as before.
- Three goldens recorded from pre-migration code; all ten goldens byte-identical after the migration. Two sabotage probes verified red. Two new root guards.
- Playtest: 5a lane A rerun, every line verbatim.

Spec: docs/superpowers/specs/2026-09-12-messaging-m3-item5b-kind-b-store-migration-design.md

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

Merge on green per `dogmud-shipping`. The owner deploys; do not.
