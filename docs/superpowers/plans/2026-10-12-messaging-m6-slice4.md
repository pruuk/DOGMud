# Messaging M6 slice 4 (spell target cast line) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a creature starts a targeted spell, the player it is aimed at reads their own `cast_actee` line instead of the room's line.

**Architecture:** `SpellData` gains `cast_actee`, surfaced as the `Actee` variant of `Narration(PhaseCast)`. One new helper, `actions.NarrateCastStart`, builds a `messaging.Trio` and `messaging.Audience` and calls `messaging.SendTrio`; the player cast command, the mob cast command and mob `aid` all call it in place of their hand-sent lines. 35 spell files gain the line, and two root guards pin which spells carry it and keep dashes out of `spells/`.

**Tech Stack:** Go, YAML world data, root guard tests (`package main`).

**Spec:** `docs/superpowers/specs/2026-10-12-messaging-m6-slice4-design.md` (owner approved 2026-10-12, rulings R1 to R4).

---

## Facts verified against source (master `eb0d57624`, 2026-10-12)

| Fact | Where |
|---|---|
| Six spell text fields, yaml `cast_actor`, `cast_observer`, `wait_actor`, `wait_observer`, `magic_actor`, `magic_observer` | `internal/spells/spells.go:88-93` |
| `Narration(p)` returns `narration.Variants{Actor, Observer}`; `Narrate` renders via `textutil.Narrate` (`narration.FirstPicker`, deterministic) | `internal/spells/narration.go:24-40`, `internal/textutil/narrate.go:25-30` |
| `validateNarration` skips a phase when `len(v.Actor) == 0 && len(v.Observer) == 0` | `internal/spells/narration.go:50-52` |
| `TestNarrationCastPutsTheCasterInActor` asserts `len(v.Actee) == 0` | `internal/spells/narration_test.go:30-32` |
| Root guard forbids reading the six fields outside `internal/spells` by regex `\.(CastUserText\|...\|MagicRoomText)\b` | `store_text_fields_guard_test.go:32` |
| Player cast start block (step "12b") | `internal/usercommands/skill.cast.go:310-339` |
| Mob cast start block | `internal/mobcommands/cast.go:118-140` |
| Mob `aid` cast start block (spell `aidskill`, only in `world/default`) | `internal/mobcommands/aid.go:60-73` |
| All three files already import `internal/actions` | each file's import block |
| Neither `cast.go` nor `skill.cast.go` cancels `conditions.Hidden`; `aid.go` does at lines 48 and 75 | grep |
| `messaging.Audience` fields: `Actor Recipient`, `ActorId`, `ActorName`, `Actee Recipient`, `ActeeId`, `ActeeName`, `Room Broadcaster`, `RemoteRoom`; leave a nil recipient UNSET (a typed nil pointer panics) | `internal/messaging/trio.go:83-102` |
| `SendTrio`: Actee line hidden by `HideNames(text, ActorName, Room.ParticipantSight(ActeeId))`; Observer via `SendTextVisualHidingNames` excluding `ActorId`, `ActeeId` | `internal/messaging/trio.go:118-145` |
| `messaging.UnseenFigure(SightNone)` = `<ansi fg="combat-anon">something</ansi>`; `messaging.Normalize` capitalises the first letter of every delivered line | `internal/messaging/hidenames.go:39-41`, `internal/messaging/normalize.go:91-94` |
| `Character.IsHidden()` reads the Awareness FSM; `Character.Perceives(other)` is true unless other is hidden and the reader lacks `SeeHidden` | `internal/characters/character.go:892,966` |
| `actions.UserActor` has field `User *users.UserRecord`; `actions.MobActor` has field `Mob *mobs.Mob`; constructors `NewUserActorInRoom(u, room)`, `NewMobActorInRoom(m, room)` | `internal/actions/actor_user.go:23-30`, `actor_mob.go:23-30` |
| Actions test fixtures: `users.NewTestUser(id, user, name, charId)`, `users.SeedUsersForTest`, `events.DrainQueuedMessagesForTest(uid)`, `rooms.SeedBiomesForTest`, `rooms.LampPtr`, `rooms.SkyLightPtr`, `mobs.SetInstanceForTest`, hidden-mob idiom (`awareness.NewMachine`, `TransitionToConcealing`, `ResolveConcealment(true, ...)`) | `internal/actions/drink_room_line_sight_test.go:42-80`, `internal/actions/search_stealth_test.go:15-38` |
| `messaging.SightDecision` values `SightFull`, `SightShapes`, `SightNone`; infrared condition fixture gives shapes in a dark cave | `internal/actions/drink_room_line_sight_test.go:49-60` |
| 40 dogmud spells have `targeting: single`; excluded from authoring: `fold-anchor`, `fold-recall`, `summon-hive-swarm`, `summon-steppe-spirit`, `heal` | `_datafiles/world/dogmud/spells/` |
| `proseDashDirs` lacks `spells`; the guard skips comment lines; `core-drain.yaml:7` description has ` -- ` | `prose_dash_guard_test.go:20-28,62-64` |
| Root guard tests are `package main` and resolve paths relative to the repo root (`shippedWorldRoot = "_datafiles/world/dogmud"`) | `shipped_narration_data_guard_test.go:1,43` |
| `fileloader.LoadAllFlatFiles[string, *spells.SpellData](dir)` loads the spell store | `shipped_narration_data_guard_test.go:186,699` |

## File structure

- Modify `internal/spells/spells.go`: the `CastActeeText` field.
- Modify `internal/spells/narration.go`: Actee in the cast phase, validation.
- Modify `internal/spells/narration_test.go`: the flipped and new tests.
- Create `internal/actions/cast_start_lines.go`: `NarrateCastStart`, the one delivery body.
- Create `internal/actions/cast_start_lines_test.go`: delivery tests.
- Modify `internal/usercommands/skill.cast.go`, `internal/mobcommands/cast.go`, `internal/mobcommands/aid.go`: call the helper.
- Modify `store_text_fields_guard_test.go`, `prose_dash_guard_test.go`; create `spell_cast_actee_guard_test.go`.
- Modify 35 files under `_datafiles/world/dogmud/spells/` and `core-drain.yaml`.
- Docs: `internal/spells/context.md`, `internal/actions/context.md`, the ledger, `docs/PATCH_NOTES.md`, `docs/README.md`.

## Working rules for every task

- Work in a worktree on branch `feat/m6-slice4` cut from `origin/master` (`git worktree add C:/tmp/dogmud-m6s4 -b feat/m6-slice4 origin/master`). Record `C:/tmp/dogmud-m6s4` in the session handoff so the tmp sweep keeps it.
- Every text edit goes through the Edit tool. Never `sed -i`, heredoc appends, Python read-modify-write, or `gofmt -w` on a file you did not just write; run `gofmt -l` to check.
- `git add` named paths only. Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- `grep -c` exits 1 on zero matches: run "expect zero" checks standalone.

---

### Task 1: `cast_actee` in the spell data

**Files:**
- Modify: `internal/spells/spells.go:88-93`
- Modify: `internal/spells/narration.go:21-62`
- Modify: `internal/spells/narration_test.go`
- Modify: `store_text_fields_guard_test.go:32`

- [ ] **Step 1: Write the failing tests**

In `internal/spells/narration_test.go`, change `boltSpec` to author the actee line:

```go
		CastUserText: "You gather a bolt.", CastRoomText: "{actor} gathers a bolt at {actee}.", WaitUserText: "You hold the bolt.",
		CastActeeText: "{actor} gathers a bolt at you.",
```

Replace the empty-Actee assertion in `TestNarrationCastPutsTheCasterInActor`:

```go
	if len(v.Actee) != 1 || v.Actee[0] != "{actor} gathers a bolt at you." {
		t.Fatalf("Actee: %v", v.Actee)
	}
```

Extend `TestNarrateSubstitutesSourceAndTarget` after its first check:

```go
	if roles.Actee != "Kael gathers a bolt at you." {
		t.Fatalf("actee: %q", roles.Actee)
	}
```

Add two tests at the end of the file:

```go
// The target's own line is a cast-phase line only: wait and magic stay
// caster and room (messaging M6 slice 4).
func TestNarrationActeeIsCastPhaseOnly(t *testing.T) {
	s := boltSpec()
	if v := s.Narration(PhaseWait); len(v.Actee) != 0 {
		t.Fatalf("wait carries an actee: %v", v.Actee)
	}
}

// A whitespace-only cast_actee is refused even when the phase has no other
// line, so it cannot slip past the "phase not authored" skip.
func TestValidateRefusesAWhitespaceOnlyActeeLine(t *testing.T) {
	s := boltSpec()
	s.CastUserText, s.CastRoomText, s.CastActeeText = "", "", " "
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "cast") {
		t.Fatalf("expected a cast-phase validation error, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/spells/ -run 'Narrat|Validate' -count=1`
Expected: build failure, `unknown field CastActeeText in struct literal`.

- [ ] **Step 3: Add the field**

In `internal/spells/spells.go`, after `CastRoomText`:

```go
	CastUserText  string `yaml:"cast_actor,omitempty"`
	CastRoomText  string `yaml:"cast_observer,omitempty"`
	// CastActeeText is the target's own line when the cast begins (messaging
	// M6 slice 4): a player the spell is aimed at reads it instead of
	// cast_observer. A self-cast and a mob target read nothing from it.
	CastActeeText string `yaml:"cast_actee,omitempty"`
```

- [ ] **Step 4: Surface it and validate it**

In `internal/spells/narration.go`, replace the `Narration` doc comment and body:

```go
// Narration assembles the variants for one phase: the caster's line is the
// Actor, the room's line the Observer. The cast phase also carries the
// target's own line as the Actee (cast_actee, messaging M6 slice 4); wait
// and magic have none.
func (s *SpellData) Narration(p Phase) narration.Variants {
	var caster, target, room string
	switch p {
	case PhaseCast:
		caster, target, room = s.CastUserText, s.CastActeeText, s.CastRoomText
	case PhaseWait:
		caster, room = s.WaitUserText, s.WaitRoomText
	case PhaseMagic:
		caster, room = s.MagicUserText, s.MagicRoomText
	}
	return narration.Variants{Actor: textutil.Pool(caster), Actee: textutil.Pool(target), Observer: textutil.Pool(room)}
}
```

In `validateNarration`, change the skip:

```go
		if len(v.Actor) == 0 && len(v.Actee) == 0 && len(v.Observer) == 0 {
			continue
		}
```

Before the change, confirm `textutil.Pool("")` returns an empty slice (read `internal/textutil` for `func Pool`); if it returns `[]string{""}`, the existing Actor/Observer skip would already be broken, so it does not, but check before relying on it.

- [ ] **Step 5: Add the field to the text-field guard**

In `store_text_fields_guard_test.go:32`, the regex becomes:

```go
	{"internal/spells", regexp.MustCompile(`\.(CastUserText|CastActeeText|CastRoomText|WaitUserText|WaitRoomText|MagicUserText|MagicRoomText)\b`), "spell text fields", nil},
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/spells/ -count=1` then `go test . -run 'StoreTextFields|TextField' -count=1`
Expected: both `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/spells/spells.go internal/spells/narration.go internal/spells/narration_test.go store_text_fields_guard_test.go
git commit -m "feat(spells): cast_actee, the target's own cast-start line

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `actions.NarrateCastStart`

**Files:**
- Create: `internal/actions/cast_start_lines.go`
- Create: `internal/actions/cast_start_lines_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/cast_start_lines_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 slice 4: the player a spell is aimed at reads cast_actee
// instead of the room's line; everyone else reads cast_observer as before.

const (
	castLineCasterId   = 7351
	castLineTargetId   = 7352
	castLineBystanderId = 7353
	castLineMobId      = 98351
	castLineInfraredId = 7361
)

func castLineSpell(withActee bool) *spells.SpellData {
	s := &spells.SpellData{
		SpellId: "test-spike", Name: "Test Spike", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		CastUserText: "You focus a spike.",
		CastRoomText: "{actor} stares intently.",
	}
	if withActee {
		s.CastActeeText = "{actor} stares at you."
	}
	return s
}

// castLineScene stands a caster (7351), a target (7352) and a bystander
// (7353) in one room of the given biome. Infrared, when asked, goes to the
// target, so a dark cave leaves it at shapes.
func castLineScene(t *testing.T, biome string, targetInfrared bool) *rooms.Room {
	t.Helper()
	seedParityWorld(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		castLineInfraredId: {
			ConditionId: castLineInfraredId, Name: "Test Heat Eyes",
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
		},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		"city": {BiomeId: "city", Lamp: rooms.LampPtr(90)},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		castLineCasterId:    users.NewTestUser(castLineCasterId, "kael", "Kael", 97351),
		castLineTargetId:    users.NewTestUser(castLineTargetId, "mira", "Mira", 97352),
		castLineBystanderId: users.NewTestUser(castLineBystanderId, "tob", "Tob", 97353),
	}))
	if targetInfrared && !users.GetByUserId(castLineTargetId).Character.Conditions.AddCondition(castLineInfraredId, true) {
		t.Fatal("precondition: the target should now carry infrared")
	}
	room := &rooms.Room{RoomId: 7350, Biome: biome}
	for _, id := range []int{castLineCasterId, castLineTargetId, castLineBystanderId} {
		room.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	return room
}

func castLineMob(t *testing.T, room *rooms.Room, hidden bool) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: castLineMobId}
	m.Character.Name = "Shaman"
	m.Character.Conditions = conditions.New()
	m.Character.RoomId = room.RoomId
	if hidden {
		m.Character.Awareness = awareness.NewMachine()
		reason := state.TransitionReason{Trigger: "cast_start_lines_test"}
		if err := m.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
			t.Fatalf("concealing the test mob: %v", err)
		}
		m.Character.Awareness.ResolveConcealment(true, reason)
		if !m.Character.IsHidden() {
			t.Fatal("fixture mob is not hidden")
		}
	}
	mobs.SetInstanceForTest(castLineMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(castLineMobId, nil) })
	room.AddMob(castLineMobId)
	return m
}

func castLinesFor(uid int) []string { return drinkSightPlain(events.DrainQueuedMessagesForTest(uid)) }

func TestCastStart_TargetReadsActeeBystanderReadsObserver(t *testing.T) {
	room := castLineScene(t, "city", false)
	caster := users.GetByUserId(castLineCasterId)

	NarrateCastStart(NewUserActorInRoom(caster, room), castLineSpell(true),
		[]int{castLineTargetId}, nil, room, messaging.CategorySpellFold)

	requireOneLine(t, "caster", castLinesFor(castLineCasterId), "You focus a spike.")
	requireOneLine(t, "target", castLinesFor(castLineTargetId), "Kael stares at you.")
	requireOneLine(t, "bystander", castLinesFor(castLineBystanderId), "Kael stares intently.")
}

func TestCastStart_UnauthoredActeeLeavesTheTargetTheRoomLine(t *testing.T) {
	room := castLineScene(t, "city", false)
	caster := users.GetByUserId(castLineCasterId)

	NarrateCastStart(NewUserActorInRoom(caster, room), castLineSpell(false),
		[]int{castLineTargetId}, nil, room, messaging.CategorySpellFold)

	requireOneLine(t, "target", castLinesFor(castLineTargetId), "Kael stares intently.")
	requireOneLine(t, "bystander", castLinesFor(castLineBystanderId), "Kael stares intently.")
}

func TestCastStart_SelfCastSendsNoActee(t *testing.T) {
	room := castLineScene(t, "city", false)
	caster := users.GetByUserId(castLineCasterId)

	NarrateCastStart(NewUserActorInRoom(caster, room), castLineSpell(true),
		[]int{castLineCasterId}, nil, room, messaging.CategorySpellFold)

	requireOneLine(t, "caster", castLinesFor(castLineCasterId), "You focus a spike.")
	requireOneLine(t, "target-less room", castLinesFor(castLineTargetId), "Kael stares intently.")
}

// R1: a target who makes out shapes reads "a figure"; one who sees nothing
// still reads their line, with the caster as "Something".
func TestCastStart_TargetLineHidesTheCasterBySight(t *testing.T) {
	cases := []struct {
		name     string
		infrared bool
		want     string
	}{
		{"shapes", true, "A figure stares at you."},
		{"none", false, "Something stares at you."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := castLineScene(t, "cave", tc.infrared)
			caster := users.GetByUserId(castLineCasterId)

			NarrateCastStart(NewUserActorInRoom(caster, room), castLineSpell(true),
				[]int{castLineTargetId}, nil, room, messaging.CategorySpellFold)

			requireOneLine(t, "target", castLinesFor(castLineTargetId), tc.want)
			if got := castLinesFor(castLineBystanderId); len(got) != 0 {
				t.Errorf("a bystander who sees nothing read %q", got)
			}
		})
	}
}

func TestCastStart_MobCasterOnAPlayer(t *testing.T) {
	room := castLineScene(t, "city", false)
	m := castLineMob(t, room, false)

	NarrateCastStart(NewMobActorInRoom(m, room), castLineSpell(true),
		[]int{castLineTargetId}, nil, room, messaging.CategorySpellFold)

	got := castLinesFor(castLineTargetId)
	if len(got) != 1 || !strings.HasSuffix(got[0], "stares at you.") || !strings.Contains(got[0], "Shaman") {
		t.Fatalf("target read %q, want the shaman staring at them", got)
	}
	got = castLinesFor(castLineBystanderId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "stares intently.") {
		t.Fatalf("bystander read %q, want the room line", got)
	}
}

// A hidden mob caster is "Something" to every reader who does not perceive
// it, in the target's line and the room's, as its landing lines already read.
func TestCastStart_HiddenMobCasterIsSomething(t *testing.T) {
	room := castLineScene(t, "city", false)
	m := castLineMob(t, room, true)

	NarrateCastStart(NewMobActorInRoom(m, room), castLineSpell(true),
		[]int{castLineTargetId}, nil, room, messaging.CategorySpellFold)

	requireOneLine(t, "target", castLinesFor(castLineTargetId), "Something stares at you.")
	requireOneLine(t, "bystander", castLinesFor(castLineBystanderId), "Something stares intently.")
}

// A mob target has no client: the room reads the room line and nobody
// reads an actee line.
func TestCastStart_MobTargetReadsNothing(t *testing.T) {
	room := castLineScene(t, "city", false)
	m := castLineMob(t, room, false)
	caster := users.GetByUserId(castLineCasterId)

	NarrateCastStart(NewUserActorInRoom(caster, room), castLineSpell(true),
		nil, []int{m.InstanceId}, room, messaging.CategorySpellFold)

	requireOneLine(t, "target player who is not the target", castLinesFor(castLineTargetId), "Kael stares intently.")
}
```

`requireOneLine`, `drinkSightPlain` and `seedParityWorld` already exist in package `actions` test files (`drink_room_line_sight_test.go`, `drink_parity_test.go`).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/actions/ -run CastStart -count=1`
Expected: build failure, `undefined: NarrateCastStart`.

- [ ] **Step 3: Write the helper**

Create `internal/actions/cast_start_lines.go`:

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// NarrateCastStart sends a spell's cast-start lines, for every caster: the
// player casting reads cast_actor, the player the spell is aimed at reads
// cast_actee (messaging M6 slice 4), and the room reads cast_observer. It is
// the one body the player cast command, the mob cast command and mob aid
// share.
//
// The target line goes only to a player other than the caster, and only
// when the spell authors one; otherwise the target stays in the room and
// reads cast_observer, as every target did before the slice. SendTrio hides
// the caster from the target by the target's sight ("a figure", or
// "Something" in the dark: owner ruling R1).
//
// A hidden mob caster is "something" in the room line, and in the target's
// line unless the target perceives it, as its landing lines already read
// (hooks.roomName). Only the first target is named, as before.
func NarrateCastStart(caster Actor, spell *spells.SpellData, targetUserIds, targetMobIds []int,
	room *rooms.Room, cat messaging.Category) {

	if spell == nil || room == nil || spell.Narration(spells.PhaseCast).Len() == 0 {
		return
	}
	cc := caster.GetCharacter()
	actorName, actorPlain := cc.GetCharacterName(true), cc.GetCharacterName(false)
	hidden := false
	if ma, ok := caster.(*MobActor); ok && ma.Mob != nil {
		hidden = cc.IsHidden()
	}
	unseenName, unseenPlain := messaging.UnseenFigure(messaging.SightNone), messaging.UnseenNoun(messaging.SightNone)

	ctx := textutil.TokenContext{ActorName: actorName, ActorPlainName: actorPlain}
	if hidden {
		ctx.ActorName, ctx.ActorPlainName = unseenName, unseenPlain
	}
	var target *users.UserRecord
	if len(targetUserIds) > 0 {
		if tu := users.GetByUserId(targetUserIds[0]); tu != nil {
			ctx.ActeeName, ctx.ActeePlainName = tu.Character.GetCharacterName(true), tu.Character.GetCharacterName(false)
			if tu.Character != cc {
				target = tu
			}
		}
	} else if len(targetMobIds) > 0 {
		if tm := mobs.GetInstance(targetMobIds[0]); tm != nil {
			ctx.ActeeName, ctx.ActeePlainName = tm.Character.GetCharacterName(true), tm.Character.GetCharacterName(false)
		}
	}
	roles := spell.Narrate(spells.PhaseCast, ctx)

	aud := messaging.Audience{ActorName: actorName, ActeeName: ctx.ActeeName, Room: room}
	trio := messaging.Trio{Observer: messaging.Say(cat, roles.Observer)}
	if ua, ok := caster.(*UserActor); ok && ua.User != nil {
		aud.Actor, aud.ActorId = ua.User, ua.User.UserId
		trio.Actor = messaging.Say(cat, roles.Actor)
	}
	if target != nil && roles.Actee != "" {
		acteeLine := roles.Actee
		if hidden && target.Character.Perceives(cc) {
			// The target sees through the hiding: re-render its line with the
			// caster's real name. FirstPicker makes the re-render pick the
			// same variant.
			seen := ctx
			seen.ActorName, seen.ActorPlainName = actorName, actorPlain
			acteeLine = spell.Narrate(spells.PhaseCast, seen).Actee
		}
		aud.Actee, aud.ActeeId = target, target.UserId
		trio.Actee = messaging.Say(cat, acteeLine)
	}
	messaging.SendTrio(trio, aud)
}
```

Check before relying on them: `Actor.GetCharacter()` exists on the interface (read `internal/actions/actor.go`), and `textutil.TokenContext` has exactly the fields `ActorName`, `ActorPlainName`, `ActeeName`, `ActeePlainName` (read `internal/textutil`). If `tu.Character` is a value rather than a pointer, compare user ids instead: `caster.GetUserId() != tu.UserId`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/actions/ -run CastStart -count=1 -v`
Expected: all seven `PASS`. If the `MobCasterOnAPlayer` names differ in case, read the delivered line and fix the fixture, not the helper.

- [ ] **Step 5: Run the root Audience guard**

Run: `go test . -run TestEveryAudienceLiteralPairsIdsWithRecipients -count=1`
Expected: `ok`. The literal names `ActorName` and `ActeeName` and sets recipients by assignment, as `hooks.spellAudience` does.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/cast_start_lines.go internal/actions/cast_start_lines_test.go
git commit -m "feat(actions): NarrateCastStart, one body for cast-start lines

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Route the three cast-start sites through the helper

**Files:**
- Modify: `internal/usercommands/skill.cast.go:310-339`
- Modify: `internal/mobcommands/cast.go:118-140`
- Modify: `internal/mobcommands/aid.go:60-73`

- [ ] **Step 1: Player cast**

Replace the whole `// 12b. Send YAML cast text (if defined).` block (from that comment through the closing `}` of `if spellInfo.Narration(spells.PhaseCast).Len() > 0 {`) with:

```go
	// 12b. Cast-start lines: the caster's, the target's own (cast_actee,
	// messaging M6 slice 4) and the room's, through one body for every
	// caster. The room line is sight-gated: a cast_observer line describes
	// what the room SEES (the 2026-09-08 darkness playtest).
	if castRoom := rooms.LoadRoom(user.Character.RoomId); castRoom != nil {
		actions.NarrateCastStart(actions.NewUserActorInRoom(user, castRoom), spellInfo,
			result.TargetUserIds, result.TargetMobInstanceIds, castRoom, messaging.CategorySpellFold)
	}
```

- [ ] **Step 2: Mob cast**

Replace the `// Send YAML cast text (if defined).` block in `internal/mobcommands/cast.go` with:

```go
	// Cast-start lines through the one body every caster shares: the target
	// player reads cast_actee, the room cast_observer (messaging M6 slice 4).
	// A mob caster has no client, so it reads nothing.
	if castRoom := rooms.LoadRoom(mob.Character.RoomId); castRoom != nil {
		actions.NarrateCastStart(actions.NewMobActorInRoom(mob, castRoom), spellInfo,
			result.TargetUserIds, result.TargetMobInstanceIds, castRoom, messaging.CategorySpellFold)
	}
```

- [ ] **Step 3: Mob aid**

Replace the `// Send YAML cast text (if defined).` block in `internal/mobcommands/aid.go` with:

```go
	// Cast-start lines through the one body every caster shares (messaging M6
	// slice 4). The aided player is the target.
	if castRoom := rooms.LoadRoom(mob.Character.RoomId); castRoom != nil && spellInfo != nil {
		actions.NarrateCastStart(actions.NewMobActorInRoom(mob, castRoom), spellInfo,
			[]int{aidPlayerId}, nil, castRoom, messaging.CategorySpellVital)
	}
```

- [ ] **Step 4: Build and drop dead imports**

Run: `go build ./... && go vet ./internal/usercommands/ ./internal/mobcommands/`
Expected: clean. If `textutil` (or `mobs`, `users`) is now unused in any of the three files, the build names it; remove that import line with the Edit tool.

- [ ] **Step 5: Run the packages' tests**

Run: `go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/actions/ ./internal/spells/ -count=1`
Expected: all `ok`. A golden or line test that pinned the old room line for a target now sees the target excluded only when a `cast_actee` exists; test spells authored before this slice have none, so no existing assertion should move. If one does, read it before changing it.

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/skill.cast.go internal/mobcommands/cast.go internal/mobcommands/aid.go
git commit -m "refactor(cast): player, mob and aid cast starts share NarrateCastStart

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Spells join the dash guard

**Files:**
- Modify: `prose_dash_guard_test.go:20-28`
- Modify: `_datafiles/world/dogmud/spells/core-drain.yaml:7`

- [ ] **Step 1: Widen the guard**

Add `"spells",` after `"taunt-messages",` in `proseDashDirs`.

- [ ] **Step 2: Run it to see it fail**

Run: `go test . -run TestPlayerProseHasNoDashes -count=1`
Expected: FAIL naming `core-drain.yaml:7`. Fix every file it names, not only this one.

- [ ] **Step 3: Fix the description**

In `core-drain.yaml`, lines 7 and 8 become:

```yaml
  its own cold fire. Mob-castable only; this is the Guardian's
  signature recharge, not a spell any living caster could learn.
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test . -run TestPlayerProseHasNoDashes -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add prose_dash_guard_test.go _datafiles/world/dogmud/spells/core-drain.yaml
git commit -m "copy(spells): no dashes in spell text; the dash guard covers spells

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: The guard and the 35 lines

**Files:**
- Create: `spell_cast_actee_guard_test.go`
- Modify: 35 files in `_datafiles/world/dogmud/spells/`

- [ ] **Step 1: Write the guard**

Create `spell_cast_actee_guard_test.go`:

```go
package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// castActeeExempt are the single-target spells that author no cast_actee:
// four act on the caster or summon (they target no one), and heal authors no
// cast text at all (messaging M6 slice 4, owner ruling R4).
var castActeeExempt = map[string]bool{
	"fold-anchor":          true,
	"fold-recall":          true,
	"summon-hive-swarm":    true,
	"summon-steppe-spirit": true,
	"heal":                 true,
}

// TestCastActeeCoversEveryAimedSpell holds the line on who reads a cast-start
// line of their own: every single-target spell that can land on someone else
// authors cast_actee, and no other spell does (a self or area spell has no
// one target to address).
func TestCastActeeCoversEveryAimedSpell(t *testing.T) {
	loaded, err := fileloader.LoadAllFlatFiles[string, *spells.SpellData](shippedWorldRoot + "/spells")
	if err != nil {
		t.Fatalf("load spells: %v", err)
	}
	authored := 0
	for id, s := range loaded {
		want := s.Targeting == combatvocab.TargetSingle && !castActeeExempt[id]
		has := len(s.Narration(spells.PhaseCast).Actee) > 0
		switch {
		case want && !has:
			t.Errorf("spell %q is aimed at one target but authors no cast_actee", id)
		case !want && has:
			t.Errorf("spell %q authors cast_actee but has no one target to address", id)
		}
		if has {
			authored++
		}
	}
	for id := range castActeeExempt {
		if _, ok := loaded[id]; !ok {
			t.Errorf("exempt spell %q no longer exists; drop it from castActeeExempt", id)
		}
	}
	if authored == 0 {
		t.Fatal("no spell authors cast_actee: the guard is checking nothing")
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test . -run TestCastActeeCoversEveryAimedSpell -count=1`
Expected: FAIL, 35 "authors no cast_actee" errors and the "checking nothing" fatal.

- [ ] **Step 3: Author the lines**

Add one `cast_actee:` line to each file, directly after its `cast_actor` value (after the last line of a folded `cast_actor: >-` block), double-quoted, exactly as written:

| File | `cast_actee` |
|---|---|
| `blood-boil.yaml` | `"{actor} extends a trembling hand toward you, and your blood grows warm."` |
| `charm.yaml` | `"{actor} stares at you, and a fierce will presses against your mind."` |
| `chrysalis-cocoon.yaml` | `"{actor} gestures at you as tendrils of living Chrysalis curl close."` |
| `chrysalis-glow.yaml` | `"{actor} concentrates on you as faint motes of light gather around you."` |
| `chrysalis-haste.yaml` | `"{actor} weaves quickening energy toward you, and your pulse stirs."` |
| `chrysalis-pall.yaml` | `"{actor} concentrates on you as the air around you dims and thickens."` |
| `chrysalis-regeneration.yaml` | `"{actor} turns to you, a vivid green glow pulsing in their hands."` |
| `conviction-armor.yaml` | `"{actor} looks to you as conviction hardens in the air around you."` |
| `conviction-bulwark.yaml` | `"{actor} lifts their hands toward you as belief gathers at your side."` |
| `conviction-spike.yaml` | `"{actor} narrows their eyes at you, conviction sharpening to a point."` |
| `conviction-surge.yaml` | `"{actor} gathers conviction toward you, a fierce glow building."` |
| `conviction-ward.yaml` | `"{actor} spreads their arms toward you as a ward shimmers into being."` |
| `empathic-shroud.yaml` | `"{actor} traces patterns around you, and the air at your edges blurs."` |
| `heat-sight.yaml` | `"{actor} looks into your eyes, and a dull warmth gathers behind them."` |
| `iron-will.yaml` | `"{actor} focuses on you, a steely aura gathering at your temples."` |
| `kinetic-hurl.yaml` | `"{actor} gathers telekinetic force, the air between you distorting."` |
| `kinetic-shove.yaml` | `"{actor} thrusts their palm toward you, the air before you distorting."` |
| `mend-wounds.yaml` | `"{actor} reaches toward you as warm light gathers in their hands."` |
| `mind-fog.yaml` | `"{actor} stares at you with unfocused eyes, and a haze presses at your thoughts."` |
| `mind-spike.yaml` | `"{actor} stares at you, and a needle of pressure builds behind your eyes."` |
| `mutation-catalyst.yaml` | `"{actor} channels a sickly green energy toward you, crackling with potential."` |
| `nerve-disruption.yaml` | `"{actor} studies you with a sharp, clinical intensity, and your skin prickles."` |
| `neural-stun.yaml` | `"{actor} stares at you, brow furrowed, and a buzz rises along your nerves."` |
| `neural-toxin.yaml` | `"{actor} focuses on you with toxic intent, a sickly glow around their hands."` |
| `night-vision.yaml` | `"{actor} looks into your eyes, and the faint light here seems to gather in them."` |
| `psychic-anchor.yaml` | `"{actor} fixes you with a piercing stare, and a weight presses on your thoughts."` |
| `purge-affliction.yaml` | `"{actor} enters a meditative focus, Chrysalis energy pulsing softly toward you."` |
| `pyretic-surge.yaml` | `"{actor} holds out their hands toward you as the air ripples with heat."` |
| `repair-pulse.yaml` | `"{actor} crackles with hard blue-white arc-light, current arcing toward you."` |
| `sensory-overload.yaml` | `"{actor} grimaces at you, and light and noise swell at the edges of your senses."` |
| `sensory-veil.yaml` | `"{actor} traces a shimmering veil in the air toward your eyes."` |
| `skill-attunement.yaml` | `"{actor} touches their temple and focuses on you, and a calm clarity stirs in your mind."` |
| `synaptic-overload.yaml` | `"{actor} locks eyes with you, veins pulsing at their temples."` |
| `veil-sight.yaml` | `"{actor} narrows their eyes at you, and the edges of your sight shimmer."` |
| `vital-surge.yaml` | `"{actor} channels a warm, pulsing glow toward you."` |

Rules the lines already meet and an edit must keep: second person, `{actor}` named, the target's experience rather than the outcome, no dashes, no numbers, "their" for the caster, every value double-quoted (no bare colon can turn a value into a map).

- [ ] **Step 4: Run the guard, the store guards and the dash guard**

Run: `go test . -run 'CastActee|ShippedNarration|PlayerProseHasNoDashes|ProseTagSpacing' -count=1`
Expected: `ok`.

- [ ] **Step 5: YAML shape check against master**

Write `$TMP/shapecheck.py` with the Write tool (not inline Bash: the Bash tool collapses `\\`):

```python
import subprocess, sys, yaml
from pathlib import Path

def shape(v):
    if isinstance(v, dict):
        return {k: shape(x) for k, x in v.items()}
    if isinstance(v, list):
        return [shape(x) for x in v]
    return type(v).__name__

root = Path(sys.argv[1])
bad = 0
for p in sorted((root / "_datafiles/world/dogmud/spells").glob("*.yaml")):
    rel = p.relative_to(root).as_posix()
    old = subprocess.run(["git", "-C", str(root), "show", f"origin/master:{rel}"], capture_output=True, text=True)
    if old.returncode != 0:
        continue
    a, b = shape(yaml.safe_load(old.stdout)), shape(yaml.safe_load(p.read_text(encoding="utf-8")))
    b.pop("cast_actee", None)
    if a != b:
        bad += 1
        print("shape changed:", rel)
print("files with changed shape:", bad)
```

Run: `python -I "$TMP/shapecheck.py" "C:/tmp/dogmud-m6s4"`
Expected: `files with changed shape: 0`.

- [ ] **Step 6: Commit**

```bash
git add spell_cast_actee_guard_test.go _datafiles/world/dogmud/spells/*.yaml
git commit -m "copy(spells): cast_actee for the 35 spells aimed at one target

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(`_datafiles/world/dogmud/spells/*.yaml` is a named path glob of the 35 edited files plus nothing else changed in that directory; check `git status --short` first and list files by name if anything unexpected is modified there.)

---

### Task 6: Docs

**Files:**
- Modify: `internal/spells/context.md` (the six-keys paragraph near line 350)
- Modify: `internal/actions/context.md`
- Modify: `docs/superpowers/audits/messaging-m6-content-ledger.md` (row 9)
- Modify: `docs/PATCH_NOTES.md`
- Modify: `docs/README.md`

- [ ] **Step 1: `internal/spells/context.md`**

Change "No file outside this package reads the six text fields" to "the seven text fields", change "The six authored keys are" to "The seven authored keys are", insert `` `cast_actee`, `` after `` `cast_actor` / `cast_observer`, ``, and append to that paragraph:

```markdown
`cast_actee` (messaging M6 slice 4) is the target's own line when the cast
begins, returned as the `Actee` variant of `Narration(PhaseCast)`; wait and
magic have none. `actions.NarrateCastStart` delivers it to a player target
other than the caster, who then reads it instead of `cast_observer`. Every
single-target spell that can land on someone else authors one; root guard
`spell_cast_actee_guard_test.go` holds the list.
```

- [ ] **Step 2: `internal/actions/context.md`**

Add a short section (match the file's heading level):

```markdown
## Cast-start lines

`NarrateCastStart(caster, spell, targetUserIds, targetMobIds, room, cat)` is
the one body for a spell's cast-start lines, called by the player cast
command, the mob cast command and mob `aid`. It sends `cast_actor`,
`cast_actee` and `cast_observer` through `messaging.SendTrio`: the target's
line only to a player other than the caster when the spell authors one, the
caster hidden from the target by sight, and a hidden mob caster named
"something" to whoever does not perceive it.
```

- [ ] **Step 3: Ledger row 9**

Replace the Status cell of row 9 (`open, narrowed (triage 2026-10-12): ...`) with:

```
done (PR #___, M6 slice 4): the landing lines already existed as Go templates in every applier (`spell_effects.go`, `spell_help_effects.go`) and every heal spell carries a condition, so the real gap was the cast start; `cast_actee` now gives the target its own line there, authored for 35 spells
```

Fill `#___` once the PR exists (Task 7).

- [ ] **Step 4: PATCH_NOTES**

Add a bullet under the first `## Unreleased` heading, after the first bullet group:

```markdown
- When someone starts a spell aimed at you, you now read a line of your own
  telling you so, instead of only what the rest of the room sees.
```

- [ ] **Step 5: `docs/README.md` plan entry**

After the slice 4 spec row, add:

```markdown
| [`superpowers/plans/2026-10-12-messaging-m6-slice4.md`](superpowers/plans/2026-10-12-messaging-m6-slice4.md) | Implementation plan for Messaging M6 slice 4 (#370 row 9), 7 tasks in one PR: `cast_actee` on `SpellData` as the cast phase's Actee, `actions.NarrateCastStart` as the one cast-start body for player cast, mob cast and mob aid (through `SendTrio`, caster hidden by the target's sight, a hidden mob caster "something"), `spells` in the dash guard, a root guard on which spells carry the line, 35 authored lines, docs, gate and a playtest |
```

(Only if Task 7's planning PR did not already add it.)

- [ ] **Step 6: Context audit and commit**

Run: `python -I tools/context_md_audit.py`
Expected: no phantom symbols in `internal/spells` or `internal/actions`.

```bash
git add internal/spells/context.md internal/actions/context.md docs/superpowers/audits/messaging-m6-content-ledger.md docs/PATCH_NOTES.md docs/README.md
git commit -m "docs: M6 slice 4, cast_actee and NarrateCastStart

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Gate, PR, playtest

- [ ] **Step 1: Full suite**

Run: `go test ./... -count=1` (from the worktree root)
Expected: every package `ok`.

- [ ] **Step 2: Lint**

Run: `golangci-lint run --new-from-merge-base=origin/master`
Expected: `0 issues.`

- [ ] **Step 3: Boot check**

Follow the `dogmud-shipping` skill's detached-worktree boot check on a private port; the log must show the spell store loading with no validation error. Kill only the PID you started.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feat/m6-slice4
gh pr create --repo pruuk/DOGMud --base master --head feat/m6-slice4 --title "M6 slice 4: the spell target's own cast line" --body "..."
```

The body summarises the three behaviour points (target line, R1 dark rule, hidden mob caster now "something" in the room line too), names the ledger row as `Ledger row 9 (#370)` without a closing keyword, and ends with the attribution line. Then fill `#___` in the ledger (Task 6 Step 3) and push that commit.

- [ ] **Step 5: Playtest**

Follow the `dogmud-playtesting` skill with an ephemeral goals file and `--checkout feat/m6-slice4`, `mid` profile. Goals: (1) get a hostile caster to start a single-target spell at the player (any mob whose spell list includes mind-spike, neural-stun or kinetic-hurl; search the mob YAML for `spells:` to pick one in a lit room) and record the line read at cast start; (2) the same in a dark room; (3) cast a help spell (mend-wounds or iron-will) at a companion or second agent if the harness supports two players. Expected: the targeted player reads the `cast_actee` line, "Something" in the dark. File any finding as a `pruuk/DOGMud` issue.
