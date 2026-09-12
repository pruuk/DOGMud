# Follow-up slices D and E, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Purge Affliction reaches a named companion; an unknown buff flag fails the build and the boot; Stone Stomach really grants poison immunity; Savant's Infusion costs what it is worth.

**Architecture:** Slice D changes only the purge dispatch and hook in `internal/hooks`, building its `messaging.Audience` directly for either target shape. Slice E adds `buffs.AllFlags` plus a load-time check and a root test, a `PoisonImmunity` flag honoured at the two buff primitives and at `Character.AddCondition`, and one item price.

**Tech Stack:** Go, `internal/hooks`, `internal/buffs`, `internal/characters`, root guard tests, YAML world data, the playtest harness.

Spec: `docs/superpowers/specs/2026-09-12-followup-slices-d-e-purge-target-and-flag-guard-design.md`

Tasks 1 and 2 are independent (disjoint files) and may run in parallel.

---

## Read this first

- Test binaries load no world YAML; seed specs with `buffs.SeedBuffsForTest`. The hooks fixture `seedAllRegistries()` (`internal/hooks/hooks_test.go:37`) seeds users 1 "Aliceia" and 2 "Bobrick" and mob instance 100 "Skeleton" in room 1; `drainPlain(userId)` returns tag-stripped lines; `countContaining(lines, needle)`; `darken(t, 1)`.
- `resolveSpell(u, activity.CastingData{...}, spell, room)` is how `internal/hooks/selfcast_wording_test.go:215-230` drives the purge case; `CastingData` has `TargetUserIds` and `TargetMobInstanceIds`.
- `mobDisplayName(mob, room, viewerUserId)` renders a mob's name for combat text; `mobs.GetInstance(id)`.
- A failing test must be seen failing. No em dashes. Named `git add` paths only.

## File structure

| File | Change |
|---|---|
| `internal/hooks/spell_resolution.go:272-281` | Purge dispatch takes the first mob target when no player target |
| `internal/hooks/spell_purgeaffliction.go` | Hook accepts a player or a mob target; audience built directly |
| `internal/hooks/purge_target_test.go` | Create: companion target purged and narrated; caster untouched |
| `internal/buffs/buffspec.go` | `PoisonImmunity` flag; `AllFlags`; load-time flag check |
| `internal/buffs/flags_test.go` | Create: registry completeness (parses the constants); unknown-flag panic |
| `internal/buffs/buffs.go` | `AddBuff` / `AddBuffScaled` refuse a `poison` spec when the holder has `poison-immunity` |
| `internal/buffs/immunity_test.go` | Create |
| `internal/characters/conditions.go` | `AddCondition` refuses `ConditionPoisoned` under immunity |
| `internal/characters/conditions_immunity_test.go` | Create |
| `buff_flag_guard_test.go` (root) | Create: every flag in the dogmud buff files is declared |
| `_datafiles/world/dogmud/items/consumables-30000/30054-savants_infusion.yaml` | `value: 100` |
| `internal/buffs/context.md`, `internal/hooks/context.md`, `docs/PATCH_NOTES.md` | Docs |
| `tools/playtest/goals/2026-09-12-slices-d-e-purge-and-stone-stomach.yaml` | Create |

---

### Task 1 (slice D): Purge Affliction reaches a companion

**Files:** `internal/hooks/spell_resolution.go`, `internal/hooks/spell_purgeaffliction.go`, create `internal/hooks/purge_target_test.go`.

- [ ] **Step 1: Failing tests.** Create `internal/hooks/purge_target_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/activity"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const purgeTestPoisonBuffId = 7201

func seedPurgeTestPoison() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		purgeTestPoisonBuffId: {BuffId: purgeTestPoisonBuffId, Name: "Test Venom", RoundInterval: 1, TriggerCount: 5,
			Flags: []buffs.Flag{buffs.Poison}, StartUserText: "venom", EndUserText: "gone"},
	})
}

// The 5a playtest cast Purge Affliction at a charmed companion and the CASTER
// was purged: the dispatch only knew player targets. A named mob target is
// purged and narrated, and the caster is left alone.
func TestPurgeAffliction_NamedMobTargetIsPurgedNotTheCaster(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPurgeTestPoison()
	defer restore()
	room := rooms.LoadRoom(1)
	caster := users.GetByUserId(1)
	mob := mobs.GetInstance(100)
	require.True(t, caster.Character.Buffs.AddBuff(purgeTestPoisonBuffId, false))
	require.True(t, mob.Character.Buffs.AddBuff(purgeTestPoisonBuffId, false))
	mob.Character.AddCondition(characters.ConditionPoisoned, 5, 1, "test")
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "purge-affliction", Name: "Purge Affliction", Type: spells.HelpSingle}
	resolveSpell(caster, activity.CastingData{SpellId: "purge-affliction", TargetMobInstanceIds: []int{100}}, spell, room)

	assert.False(t, mob.Character.HasBuff(purgeTestPoisonBuffId), "the named mob is purged")
	assert.False(t, mob.Character.HasCondition(characters.ConditionPoisoned))
	assert.True(t, caster.Character.HasBuff(purgeTestPoisonBuffId), "the caster keeps their own poison")
	casterLines := drainPlain(1)
	assert.Equal(t, 1, countContaining(casterLines, "You direct purging energy towards Skeleton."))
	assert.Equal(t, 0, countContaining(casterLines, "from your body"))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia directs purging energy towards Skeleton."))
}

func TestPurgeAffliction_NamedMobTargetInTheDarkIsHiddenFromTheRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPurgeTestPoison()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	caster := users.GetByUserId(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "purge-affliction", Name: "Purge Affliction", Type: spells.HelpSingle}
	resolveSpell(caster, activity.CastingData{SpellId: "purge-affliction", TargetMobInstanceIds: []int{100}}, spell, room)

	assert.Equal(t, 0, countContaining(drainPlain(2), "purging"), "an unsighted observer reads nothing")
}
```
If `HasCondition` is not the accessor's name, read `internal/characters/conditions.go` and use the real one. If `resolveSpell` refuses a mob target for a help spell before reaching the dispatch (an admission rule), read `spell_resolution.go:100-140` (where `TargetMobInstanceIds` is filtered to allies) and seed the mob as the caster's charmed companion the way `selfcast_wording_test.go` or `charm_in_combat_test.go` does; say what was needed.

- [ ] **Step 2: Run red.** `go test ./internal/hooks/ -run TestPurgeAffliction -v`. Expected: the first test FAILS (mob still poisoned, caster purged, caster reads "from your body").

- [ ] **Step 3: Dispatch.** In `spell_resolution.go:272-281`:

```go
		case "purge-affliction":
			switch {
			case len(cs.TargetUserIds) > 0:
				if targetUser := users.GetByUserId(cs.TargetUserIds[0]); targetUser != nil {
					resolvePurgeAffliction(user, room, purgeTarget{char: targetUser.Character, user: targetUser, name: targetUser.Character.Name})
				}
			case len(cs.TargetMobInstanceIds) > 0:
				if tMob := mobs.GetInstance(cs.TargetMobInstanceIds[0]); tMob != nil {
					resolvePurgeAffliction(user, room, purgeTarget{char: &tMob.Character, name: tMob.Character.Name, display: mobDisplayName(tMob, room, user.UserId)})
				}
			default:
				resolvePurgeAffliction(user, room, purgeTarget{char: user.Character, user: user, name: user.Character.Name}) // self-cast
			}
			return true
```

- [ ] **Step 4: Hook.** Rewrite `spell_purgeaffliction.go`:

```go
// purgeTarget is whoever Purge Affliction was aimed at: a player (user set),
// a mob such as a charmed companion (user nil, display set), or the caster.
type purgeTarget struct {
	char    *characters.Character
	user    *users.UserRecord // nil for a mob
	name    string            // plain name, for the seam to hide
	display string            // rendered mob name; empty for a player
}

func (p purgeTarget) token() string {
	if p.display != "" {
		return p.display
	}
	return fmt.Sprintf(`<ansi fg="username">%s</ansi>`, p.name)
}

// resolvePurgeAffliction narrates the purge and cancels poison on the target.
// Self-cast keeps its one-line wording. A mob target has no client, so its
// Actee recipient is nil and only the caster and the room read anything; names
// are hidden per reader by the seam.
func resolvePurgeAffliction(user *users.UserRecord, room *rooms.Room, target purgeTarget) {
	if user == nil || target.char == nil {
		mudlog.Error("resolvePurgeAffliction", "error", "nil user or target")
		return
	}
	if target.char == user.Character {
		user.SendText(messaging.CategorySpellVital, `<ansi fg="green">You purge the afflictions from your body.</ansi>`)
		if room != nil {
			sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> purges their afflictions.`, user.Character.Name), user.UserId)
		}
	} else {
		var actee messaging.Recipient
		acteeId := 0
		if target.user != nil {
			actee = target.user
			acteeId = target.user.UserId
		}
		aud := messaging.Audience{
			Actor: user, ActorId: user.UserId, ActorName: user.Character.Name,
			Actee: actee, ActeeId: acteeId, ActeeName: target.name,
		}
		if room != nil {
			aud.Room = room
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">You direct purging energy towards %s.</ansi>`, target.token())),
			Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi> purges the afflictions from your body.</ansi>`, user.Character.Name)),
			Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> directs purging energy towards %s.`, user.Character.Name, target.token())),
		}, aud)
	}
	target.char.CancelBuffsWithFlag(buffs.Poison)
	target.char.RemoveCondition(characters.ConditionPoisoned)
}
```
Keep the imports tidy. The existing self-cast tests (`selfcast_wording_test.go:215`, `hooks_test.go:2395`) must stay green; the M2 routing golden may need re-recording only if the guard says so (read its header; re-record in the same commit and say why).

- [ ] **Step 5: Green and guards.** `go test ./internal/hooks/ -run "TestPurgeAffliction|TestSelfCastPurge|TestApplyPlayerEffect_Purge" -v`; `go test ./internal/hooks/`; the root guards `go test . -run "TestNarrationSitesMatchViewpointAudit|TestM2RoutingIsFrozen|TestEveryTrioLiteralNamesAllThreeRoles|TestM2LiteralsAreFrozen|TestEveryTextSurfaceIsRegistered"`; gofmt.

- [ ] **Step 6: Commit.**
```bash
git add internal/hooks/spell_resolution.go internal/hooks/spell_purgeaffliction.go internal/hooks/purge_target_test.go
git commit -m "fix(spells): Purge Affliction reaches a named companion" -m "The dispatch only knew player targets, so a cast at a charmed companion purged the caster. A mob target is now purged and narrated through the seam; the caster keeps their own afflictions." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 2 (slice E): the flag guard, poison immunity, the price

**Files:** `internal/buffs/buffspec.go`, `internal/buffs/buffs.go`, create `internal/buffs/flags_test.go` and `internal/buffs/immunity_test.go`, `internal/characters/conditions.go`, create `internal/characters/conditions_immunity_test.go`, create `buff_flag_guard_test.go` (root), `30054-savants_infusion.yaml`.

- [ ] **Step 1: Failing tests, registry.** Create `internal/buffs/flags_test.go`:

```go
package buffs

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every Flag constant declared in buffspec.go must be in AllFlags, or the
// load-time guard would reject a buff that uses a perfectly good flag.
func TestAllFlagsNamesEveryDeclaredConstant(t *testing.T) {
	src, err := os.ReadFile("buffspec.go")
	require.NoError(t, err)
	re := regexp.MustCompile("Flag = `([a-z-]+)`")
	declared := map[Flag]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		declared[Flag(m[1])] = true
	}
	require.NotEmpty(t, declared)
	listed := map[Flag]bool{}
	for _, f := range AllFlags {
		listed[f] = true
	}
	for f := range declared {
		assert.True(t, listed[f], "declared flag %q is missing from AllFlags", f)
	}
	for f := range listed {
		assert.True(t, declared[f], "AllFlags lists %q, which no constant declares", f)
	}
}

func TestUnknownFlagIsRejectedAtLoad(t *testing.T) {
	assert.NoError(t, (&BuffSpec{BuffId: 1, Name: "Fine", Flags: []Flag{Poison, NightVision}}).ValidateFlags())
	err := (&BuffSpec{BuffId: 65, Name: "Cat's Eye Draught", Flags: []Flag{"night-vision"}}).ValidateFlags()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "night-vision")
	assert.Contains(t, err.Error(), "65")
}
```

- [ ] **Step 2: Red.** `go test ./internal/buffs/ -run "TestAllFlags|TestUnknownFlag" -v`: compile FAIL (undefined `AllFlags`, `ValidateFlags`).

- [ ] **Step 3: Implement the registry.** In `buffspec.go`, after the `Flag` constants: add `PoisonImmunity Flag = \`poison-immunity\`` with a comment ("while held, poison-flagged buffs and the poisoned condition are refused"), then

```go
// AllFlags is every flag the engine understands. LoadDataFiles rejects a buff
// whose flags include anything else, exactly as spelled: the Cat's Eye
// Draught shipped with `night-vision` for `nightvision` and did nothing for
// weeks. TestAllFlagsNamesEveryDeclaredConstant keeps this list honest.
var AllFlags = []Flag{ /* every constant, one per line */ }

// ValidateFlags reports the first flag this spec carries that the engine does
// not declare. Compared exactly; nothing is normalised.
func (b *BuffSpec) ValidateFlags() error {
	for _, f := range b.Flags {
		if !slices.Contains(AllFlags, f) {
			return fmt.Errorf("buffId %d (%s) carries unknown flag %q; see buffs.AllFlags", b.BuffId, b.Name, f)
		}
	}
	return nil
}
```
In `LoadDataFiles`, inside the existing loop over `tmpBuffs`, add `if err := b.ValidateFlags(); err != nil { panic(err) }` (the species guard panics the same way). Green: the two tests, then `go test ./internal/buffs/`.

- [ ] **Step 4: Failing tests, immunity.** Create `internal/buffs/immunity_test.go`:

```go
package buffs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoisonImmunityRefusesPoisonBuffs(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		64:   {BuffId: 64, Name: "Stone Stomach", TriggerCount: 5, RoundInterval: 1, Flags: []Flag{PoisonImmunity}},
		39:   {BuffId: 39, Name: "Venom", TriggerCount: 5, RoundInterval: 1, Flags: []Flag{Poison}},
		1000: {BuffId: 1000, Name: "Harmless", TriggerCount: 5, RoundInterval: 1},
	})
	defer restore()
	bs := &Buffs{}
	bs.Validate()
	require.True(t, bs.AddBuff(64, false))
	assert.False(t, bs.AddBuff(39, false), "a poison buff is refused while immune")
	assert.False(t, bs.HasBuff(39))
	assert.False(t, bs.AddBuffScaled(39, 0.5), "the scaled primitive refuses too")
	assert.True(t, bs.AddBuff(1000, false), "a non-poison buff still lands")

	unprotected := &Buffs{}
	unprotected.Validate()
	assert.True(t, unprotected.AddBuff(39, false), "without immunity poison lands")
}
```
Read how other `internal/buffs` tests construct an empty `Buffs` (`Validate()` or a constructor) and mirror it. Red: `go test ./internal/buffs/ -run TestPoisonImmunity -v` fails on the refusal assertion.

- [ ] **Step 5: Implement at the primitives.** At the top of both `Buffs.AddBuff` and `Buffs.AddBuffScaled`, after the spec lookup succeeds:

```go
		// Poison immunity (Stone Stomach): a poison-flagged buff is refused
		// while the holder is immune. Checked here so every application path,
		// event or direct, honours it. Silent: the immunity's own start line
		// already told the player.
		if slices.Contains(buffInfo.Flags, Poison) && bs.HasFlag(PoisonImmunity, false) {
			return false
		}
```
Green.

- [ ] **Step 6: Condition.** Create `internal/characters/conditions_immunity_test.go`: seed buff 64 with `PoisonImmunity` via `buffs.SeedBuffsForTest`, `c := &Character{}` initialised the way `conditions_test.go` (or `taunt_hold_test.go`) does, `c.Buffs.AddBuff(64,false)`, then `c.AddCondition(ConditionPoisoned, 5, 1, "test")` and assert the character does not have it, while `ConditionBlinded` still lands. Red, then in `AddCondition` add at the top: `if typ == ConditionPoisoned && c.Buffs.HasFlag(buffs.PoisonImmunity, false) { return }` with a one-line comment. Check `characters` already imports `buffs` (it does for `HasBuffFlag`). Green: `go test ./internal/characters/`.

- [ ] **Step 7: Root guard.** Create `buff_flag_guard_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"gopkg.in/yaml.v3"
)

// Slice E: every flag a dogmud buff carries must be one the engine declares,
// spelled exactly. An unknown flag used to load silently and do nothing.
func TestEveryDogmudBuffFlagIsDeclared(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "buffs", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no buff files: %v", err)
	}
	sort.Strings(files)
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			BuffId int      `yaml:"buffid"`
			Name   string   `yaml:"name"`
			Flags  []string `yaml:"flags"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		spec := &buffs.BuffSpec{BuffId: b.BuffId, Name: b.Name}
		for _, fl := range b.Flags {
			spec.Flags = append(spec.Flags, buffs.Flag(fl))
		}
		if err := spec.ValidateFlags(); err != nil {
			problems = append(problems, filepath.Base(f)+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d unknown buff flags:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}
```
Run it: with `PoisonImmunity` declared it passes. Prove it can fail: temporarily change `64-stone_stomach.yaml`'s flag to `poison-immunty`, run, confirm FAIL naming the file, restore the line exactly. Paste the red line.

- [ ] **Step 8: Price.** `30054-savants_infusion.yaml`: `value: 60` becomes `value: 100`. Nothing else in the file.

- [ ] **Step 9: Gates.** `go build ./...`; `go test ./internal/buffs/ ./internal/characters/ ./internal/hooks/`; `go test . -run "TestEveryDogmudBuffFlagIsDeclared|TestEveryDogmudBuffHasAuthoredNotices|TestPlayerBuffsTravelTheEventPath|TestEveryTextSurfaceIsRegistered"`; gofmt; `golangci-lint run --new-from-rev=master ./...` 0 issues.

- [ ] **Step 10: Commits** (two):
```bash
git add internal/buffs/buffspec.go internal/buffs/flags_test.go buff_flag_guard_test.go
git commit -m "feat(buffs): an unknown buff flag fails the boot and the build" -m "AllFlags lists every declared flag; LoadDataFiles panics on a spec carrying anything else, a root test walks the dogmud buff files, and a unit test keeps the list complete. Compared exactly: the Cat's Eye Draught shipped with a misspelled flag and did nothing." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
git add internal/buffs/buffs.go internal/buffs/immunity_test.go internal/characters/conditions.go internal/characters/conditions_immunity_test.go _datafiles/world/dogmud/items/consumables-30000/30054-savants_infusion.yaml
git commit -m "feat(buffs): poison immunity is real, and Savant's Infusion costs what it is worth" -m "Stone Stomach's poison-immunity flag was read by nothing. Both buff primitives now refuse a poison-flagged buff and AddCondition refuses the poisoned condition while it is held. Savant's Infusion moves to the catalyst price tier since it is the stronger of the learning pair." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 3: Docs, playtest lane, ship

- [ ] `internal/buffs/context.md`: `AllFlags`, `ValidateFlags`, the load-time panic, `PoisonImmunity` and where it is honoured. `internal/hooks/context.md`: the purge hook takes a `purgeTarget` (player, mob or self). `docs/PATCH_NOTES.md`: one entry, 80 columns, no numbers, no dashes: Purge Affliction now works on a companion you name; Stone Stomach now truly keeps poison out of you; Savant's Infusion costs more, as the stronger of the two learning tonics.
- [ ] Playtest lane `tools/playtest/goals/2026-09-12-slices-d-e-purge-and-stone-stomach.yaml`: veteran, a lit quiet room, grant a Stone Stomach potion (30046) and a toxic flask or any poisoning item (find one whose effect carries the `poison` flag; `30059-toxic_flask.yaml` if it self-poisons on use, else a poison-applying drink), plus a charmed companion (the veteran profile has golems). Goals: poison the companion or yourself, cast `purge <companion>`, quote the lines and confirm the companion's `conditions`/look shows it clean and yours does not change; then drink Stone Stomach, expose yourself to poison, quote that nothing lands and no poison line appears. Run per `dogmud-playtesting`, extract findings, commit the goals file.
- [ ] Gates per `dogmud-shipping` (full suite standalone for `internal/playtestrun` if it times out under load, isolated boot check), push, PR on `pruuk/DOGMud`, merge on green.

## Self-review
- Spec defect 1 -> Task 1; defects 2, 3, 4 -> Task 2; docs and lane -> Task 3.
- Names consistent: `purgeTarget`, `resolvePurgeAffliction(user, room, target)`, `AllFlags`, `ValidateFlags`, `PoisonImmunity`, `TestEveryDogmudBuffFlagIsDeclared`.
