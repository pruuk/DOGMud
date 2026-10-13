# Messaging M6 slice 4 (one body for a caster's spell lines) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every caster's own spell lines (cast start, each fold round, magic) go through one body, `actions.NarrateSpellPhase`, under one sight and hiding rule; the player a spell is aimed at reads their own `cast_actee` line; a hidden caster's cast start is heard as an unnamed murmur; a harmful spell breaks hiding when it lands.

**Architecture:** `SpellData` gains `cast_actee`; `casting-messages.yaml` gains three pools (two generic room lines, one unseen-caster sound). The perception helpers move from `hooks` to `actions`. `NarrateSpellPhase` replaces seven hand-rolled sites in `usercommands`, `mobcommands` and `hooks`; a root guard keeps phase narration inside `actions`. The Awareness cascade stops revealing a caster whose combat entry is only its own fold, and the landing body reveals the caster of a harmful spell explicitly.

**Tech Stack:** Go, YAML world data, root guard tests (`package main`).

**Spec:** `docs/superpowers/specs/2026-10-12-messaging-m6-slice4-design.md` (owner approved 2026-10-12, rulings R1 to R7).

---

## Facts verified against source (master `eb0d57624`, 2026-10-12)

| Fact | Where |
|---|---|
| Spell text fields and yaml keys `cast_actor`, `cast_observer`, `wait_actor`, `wait_observer`, `magic_actor`, `magic_observer` | `internal/spells/spells.go:88-93` |
| `Narration(p)` returns `Variants{Actor, Observer}`; `Narrate` renders via `textutil.Narrate` with `narration.FirstPicker` | `internal/spells/narration.go:24-40`, `internal/textutil/narrate.go:25-30` |
| `textutil.Pool("")` returns nil; `TokenContext{ActorName, ActorPlainName, ActeeName, ActeePlainName}` | `internal/textutil/narrate.go:8-12`, `internal/textutil/tokens.go:14-19` |
| ⚠️ `Variants.Len()` returns 0 when two non-empty role pools differ in length, so a 3-line generic pool cannot ride beside a 1-line authored caster line in one `Variants`: render it separately | `internal/narration/render.go:58-72` |
| `validateNarration` skips a phase when Actor and Observer are both empty | `internal/spells/narration.go:50-52` |
| `TestNarrationCastPutsTheCasterInActor` asserts an empty Actee | `internal/spells/narration_test.go:30-32` |
| Root guard `TestStoreTextFieldsAreReadOnlyByTheirStore`: spell field regex at line 32; walks `messagingSurfaceGoRoots` (`internal`, `modules`), skips `_test.go`, and fails when the pattern matches nothing inside the owner | `store_text_fields_guard_test.go:21-120`, `messaging_surface_guard_test.go:427` |
| `CastingMessages` fields `AlreadyCasting`, `CastStarted`, `CastContinuing`, `ConcentrationSlipped`; `Validate` requires 3 variants per pool via `narration.ValidateVariants(Variants{Actor: pool}, 3, RoleActor)`; `GetCastMessage` returns a generic sentence when the store is not loaded (unit tests) | `internal/spells/casting_messages.go:16-60,128-170` |
| Spells test helper `seedCastingMessages(t, cm)` and fixture `shippedShape()` | `internal/spells/casting_messages_test.go:12-28` |
| Both worlds ship `casting-messages.yaml` (`dogmud` and `default`); dogmud's `concentration_slipped` second line has an em dash | `_datafiles/world/{dogmud,default}/casting-messages.yaml` |
| `internal/narration/snapshot_test.go` goldens the casting store (4 categories, via `GetCastMessage`) and loads the shipped world; regenerate with `go test ./internal/narration/... -run TestSnapshotStores -update` ONLY after reading the diff | `internal/narration/snapshot_test.go:113,157,714-737,858` |
| Site 1 player cast start (block "12b") | `internal/usercommands/skill.cast.go:310-339` |
| Site 2 mob cast start; the extra `"<name> begins weaving a spell."` at 173-174; harmful cast commits targeting at 179-190, AFTER `TransitionToCasting` (line ~156) | `internal/mobcommands/cast.go:118-190` |
| Site 3 mob `aid` cast start; `aid` cancels `conditions.Hidden` at 48 and 75 | `internal/mobcommands/aid.go:48-75` |
| Sites 4, 5 player wait lines in `handlePlayerFoldCasting` (CastComplete and StillCasting branches), sent by `r.SendText` (audio) | `internal/hooks/NewRound_DoCombat_helpers.go:653,707-722,850-866` |
| Site 6 mob fold round: `handleMobFoldCasting` StillCasting calls `sendMobWindupWarning` else `sendMobWeaving`; CastComplete (line 940) sends no wait line | `internal/hooks/NewRound_DoCombat_helpers.go:556-585,899,940,1023-1026` |
| Site 7 player magic line in `resolveSpell`, audio; `resolveMobSpell` has no magic line and ends `return anyLanded, false` | `internal/hooks/spell_resolution.go:195-221,599-685` |
| Helper bodies to move: `mobDisplayName`, `mobSeenName`, `mobHiddenFrom` | `internal/hooks/NewRound_DoCombat_helpers.go:405-436` |
| `conditionLineUnseenBy` (nil unless hidden, else `playersNotPerceiving`) | `internal/hooks/Condition_ApplyConditions.go:245-250` |
| `playersNotPerceiving(room, c)` | `internal/hooks/Logout_AwarenessCleanup.go:63-71` |
| `mobActUnseenBy(room, mob)` = `conditionLineUnseenBy(room, &mob.Character)` | `NewRound_DoCombat_helpers.go:519-521` |
| Callers: `mobHiddenFrom` 5, `mobSeenName` 9, `mobDisplayName` 13, `playersNotPerceiving` 2, `conditionLineUnseenBy` 6, `mobActUnseenBy` 6 (all in `internal/hooks`) | grep |
| `actions.UserActor{User, Room}`, `actions.MobActor{Mob, Room}`; `Actor.GetCharacter()`, `Actor.GetUserId()` | `internal/actions/actor_user.go:12-30`, `actor_mob.go:12-30`, `actor.go:18,34` |
| `messaging.Audience` / `Trio` / `SendTrio`; a nil recipient must stay unset; observer half is `SendTextVisualHidingNames` excluding ActorId and ActeeId | `internal/messaging/trio.go:38-159` |
| `messaging.UnseenFigure(SightNone)` = combat-anon "something"; `Normalize` capitalises the first letter of delivered lines | `internal/messaging/hidenames.go:30-41`, `normalize.go:91-94` |
| `Character.IsHidden()`, `Character.Perceives(other)` | `internal/characters/character.go:892,966` |
| Awareness cascade: Idle to Engaging reveals a Hidden character; registered by `characters.OnCharacterCreated(wireAwarenessFromCombatPhase)` | `internal/hooks/Awareness_Cascades.go:29-41,82` |
| `commitHarmfulSpellAggro(c spellEffectCtx, fresh bool)` is called by every harmful applier (8 call sites in `spell_effects.go` and `spell_help_effects.go`) | grep |
| Reveal idiom: `char.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: awareness.TriggerNoisyAction, ...})`; `awareness.TriggerCombatEntered` exists | `internal/actions/combat_rally.go:61-68`, `internal/state/awareness/transitions.go:19,24` |
| Hooks test fixtures: `hiddenHolderFixture`, `sneakHide`, `grantSeeHidden`, `drainPlain`, `countContaining`, `seedFallbackRoom(t, lamp, eyesId)`, `heatEyesConditionId`, `spellChannelMob` (Skeleton, instance 100) | `internal/hooks/hidden_mob_room_lines_test.go`, `dark_room_fallback_sight_test.go:24`, `spell_channel_sight_test.go:35` |
| Hooks tests calling `sendMobWeaving` | `spell_channel_sight_test.go:73,85`, `hidden_mob_room_lines_test.go:169` |
| Actions test fixtures: `users.NewTestUser`, `users.SeedUsersForTest`, `events.DrainQueuedMessagesForTest`, `seedParityWorld`, `drinkSightPlain`, `requireOneLine`, hidden-mob idiom in `search_stealth_test.go:15-38` | `internal/actions/*_test.go` |
| 40 single-target dogmud spells; exemptions fold-anchor, fold-recall, summon-hive-swarm, summon-steppe-spirit, heal | `_datafiles/world/dogmud/spells/` |
| Dash guard dirs lack `spells`; comment lines skipped; `core-drain.yaml:7` has ` -- ` | `prose_dash_guard_test.go:20-28,62-64` |

## File structure

- `internal/spells/spells.go`, `narration.go`, `narration_test.go`: `cast_actee`.
- `internal/spells/casting_messages.go`, `casting_messages_test.go`: three pools, two accessors.
- `_datafiles/world/{dogmud,default}/casting-messages.yaml`: the pools.
- Create `internal/actions/perception.go` + `_test.go`: the moved helpers.
- Create `internal/actions/spell_phase_lines.go` + `_test.go`: `NarrateSpellPhase`.
- Modify `internal/hooks/NewRound_DoCombat_helpers.go`, `Condition_ApplyConditions.go`, `Logout_AwarenessCleanup.go`, `spell_resolution.go`, `spell_effects.go`, `Awareness_Cascades.go`, plus their tests.
- Modify `internal/usercommands/skill.cast.go`, `internal/mobcommands/cast.go`, `aid.go`.
- Root guards: modify `store_text_fields_guard_test.go`, `prose_dash_guard_test.go`; create `spell_phase_narration_guard_test.go`, `spell_cast_actee_guard_test.go`.
- 35 spell files + `core-drain.yaml`; docs.

## Working rules for every task

- Worktree: `git worktree add C:/tmp/dogmud-m6s4 -b feat/m6-slice4 origin/master`. Name `C:/tmp/dogmud-m6s4` in the session handoff so the tmp sweep keeps it.
- Every text edit through the Edit tool. Never `sed -i`, heredoc appends, a Python read-modify-write, or `gofmt -w` on a file you did not just write; check with `gofmt -l`.
- `git add` named paths only. Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- `grep -c` exits 1 on zero matches: run "expect zero" checks standalone.
- Before relying on any symbol a step names, open its definition. If it differs from this plan, follow the source and note the difference in the commit message.

---

### Task 1: `cast_actee` in the spell data

**Files:** `internal/spells/spells.go:88-93`, `internal/spells/narration.go`, `internal/spells/narration_test.go`, `store_text_fields_guard_test.go:32`

- [ ] **Step 1: Failing tests.** In `narration_test.go`, `boltSpec` gains `CastActeeText: "{actor} gathers a bolt at you.",`. In `TestNarrationCastPutsTheCasterInActor` replace the empty-Actee check with:

```go
	if len(v.Actee) != 1 || v.Actee[0] != "{actor} gathers a bolt at you." {
		t.Fatalf("Actee: %v", v.Actee)
	}
```

In `TestNarrateSubstitutesSourceAndTarget`, after the first check:

```go
	if roles.Actee != "Kael gathers a bolt at you." {
		t.Fatalf("actee: %q", roles.Actee)
	}
```

Append:

```go
// The target's own line is a cast-phase line only (messaging M6 slice 4).
func TestNarrationActeeIsCastPhaseOnly(t *testing.T) {
	if v := boltSpec().Narration(PhaseWait); len(v.Actee) != 0 {
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

- [ ] **Step 2:** `go test ./internal/spells/ -count=1` fails: `unknown field CastActeeText`.

- [ ] **Step 3: Field.** In `spells.go`, after `CastRoomText`:

```go
	// CastActeeText is the target's own line when the cast begins (messaging
	// M6 slice 4): a player the spell is aimed at reads it instead of
	// cast_observer. A self-cast and a mob target read nothing from it.
	CastActeeText string `yaml:"cast_actee,omitempty"`
```

- [ ] **Step 4: Narration.** Replace `Narration`'s comment and body:

```go
// Narration assembles the variants for one phase: the caster's line is the
// Actor, the room's line the Observer. The cast phase also carries the
// target's own line as the Actee (cast_actee, messaging M6 slice 4); wait
// and magic have none. actions.NarrateSpellPhase is the one sender.
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

In `validateNarration` the skip becomes `if len(v.Actor) == 0 && len(v.Actee) == 0 && len(v.Observer) == 0 {`.

- [ ] **Step 5: Guard regex** (line 32): `\.(CastUserText|CastActeeText|CastRoomText|WaitUserText|WaitRoomText|MagicUserText|MagicRoomText)\b`.

- [ ] **Step 6:** `go test ./internal/spells/ -count=1` and `go test . -run TestStoreTextFieldsAreReadOnlyByTheirStore -count=1`: both `ok`.

- [ ] **Step 7: Commit** `internal/spells/spells.go internal/spells/narration.go internal/spells/narration_test.go store_text_fields_guard_test.go`, message `feat(spells): cast_actee, the target's own cast-start line`.

---

### Task 2: The three casting pools

**Files:** `internal/spells/casting_messages.go`, `internal/spells/casting_messages_test.go`, `_datafiles/world/dogmud/casting-messages.yaml`, `_datafiles/world/default/casting-messages.yaml`

- [ ] **Step 1: Failing tests.** In `casting_messages_test.go`, `shippedShape()` becomes:

```go
func shippedShape() *CastingMessages {
	return &CastingMessages{
		AlreadyCasting:       []string{"a", "b", "c"},
		CastStarted:          []string{"a", "b", "c"},
		CastContinuing:       []string{"a", "b", "c", "d"},
		ConcentrationSlipped: []string{"a", "b", "c"},
		CastStartedRoom:      []string{"{actor} a.", "{actor} b.", "{actor} c."},
		CastContinuingRoom:   []string{"{actor} a.", "{actor} b.", "{actor} c."},
		CastStartedUnseen:    []string{"a", "b", "c"},
	}
}
```

`seedCastingMessages` always marks the store loaded; give it the unloaded case, so a nil store means "no world booted":

```go
func seedCastingMessages(t *testing.T, cm *CastingMessages) {
	t.Helper()
	prevStore, prevLoaded := castingMessages, castingMessagesLoaded
	castingMessages, castingMessagesLoaded = cm, cm != nil
	t.Cleanup(func() { castingMessages, castingMessagesLoaded = prevStore, prevLoaded })
}
```

Append:

```go
// Messaging M6 slice 4, R7: the generic room lines are data, with a fallback
// only when no world is loaded (unit tests), as GetCastMessage has.
func TestCastRoomPoolReadsTheStoreThenFallsBack(t *testing.T) {
	cm := shippedShape()
	cm.CastStartedRoom = []string{"{actor} a.", "{actor} b.", "{actor} c."}
	seedCastingMessages(t, cm)
	if got := CastRoomPool(PhaseCast); len(got) != 3 || got[0] != "{actor} a." {
		t.Fatalf("cast pool: %v", got)
	}
	if got := CastRoomPool(PhaseMagic); got != nil {
		t.Fatalf("magic has no generic line, got %v", got)
	}
}

func TestCastRoomPoolFallbackWhenUnloaded(t *testing.T) {
	seedCastingMessages(t, nil)
	if got := CastRoomPool(PhaseWait); len(got) != 1 || got[0] != "{actor} weaves magic with focused intent." {
		t.Fatalf("wait fallback: %v", got)
	}
	if got := CastRoomPool(PhaseCast); len(got) != 1 || got[0] != "{actor} begins weaving a spell." {
		t.Fatalf("cast fallback: %v", got)
	}
	if CastStartedUnseenLine() == "" {
		t.Fatal("unseen line must never be empty")
	}
}

func TestValidateRejectsAShortUnseenPool(t *testing.T) {
	cm := shippedShape()
	cm.CastStartedUnseen = cm.CastStartedUnseen[:1]
	if err := cm.Validate(); err == nil || !strings.Contains(err.Error(), "cast_started_unseen") {
		t.Fatalf("got %v", err)
	}
}
```

- [ ] **Step 2:** `go test ./internal/spells/ -run 'CastRoomPool|Unseen' -count=1` fails: undefined fields and functions.

- [ ] **Step 3: Implementation.** In `CastingMessages`, after `ConcentrationSlipped`:

```go
	// CastStartedRoom and CastContinuingRoom are the room's line for a cast
	// start and a fold round when the spell authors no *_observer line
	// (messaging M6 slice 4, owner ruling R7). {actor} is the caster.
	CastStartedRoom    []string `yaml:"cast_started_room"`
	CastContinuingRoom []string `yaml:"cast_continuing_room"`
	// CastStartedUnseen is what a reader who does not perceive a hidden
	// caster hears when the cast starts (owner ruling R5). It names nobody
	// and does not say who is targeted.
	CastStartedUnseen []string `yaml:"cast_started_unseen"`
```

Add the three to `Validate`'s pool list (names `cast_started_room`, `cast_continuing_room`, `cast_started_unseen`); the two room pools validate as `narration.Variants{Observer: p.pool}` with `narration.RoleObserver`, so split the loop into actor pools and observer pools rather than validating `{actor}` text as an Actor line. Update the `LoadCastingMessages` log `"categories", 4` to 7.

Append:

```go
// CastRoomPool is the room's generic line for a phase whose spell authors no
// *_observer line (owner ruling R7): cast_started_room for the cast,
// cast_continuing_room for a fold round, none for magic. With no world
// loaded (unit tests) it returns the one sentence each pool started from,
// for the reason GetCastMessage gives: there is no authored text to shadow.
func CastRoomPool(p Phase) []string {
	if castingMessagesLoaded && castingMessages != nil {
		switch p {
		case PhaseCast:
			if len(castingMessages.CastStartedRoom) > 0 {
				return castingMessages.CastStartedRoom
			}
		case PhaseWait:
			if len(castingMessages.CastContinuingRoom) > 0 {
				return castingMessages.CastContinuingRoom
			}
		}
	}
	switch p {
	case PhaseCast:
		return []string{"{actor} begins weaving a spell."}
	case PhaseWait:
		return []string{"{actor} weaves magic with focused intent."}
	}
	return nil
}

// CastStartedUnseenLine picks the line a reader who does not perceive a
// hidden caster hears at cast start (owner ruling R5).
func CastStartedUnseenLine(picker ...narration.Picker) string {
	pool := []string{"You hear a faint murmur nearby, though you see no one."}
	if castingMessagesLoaded && castingMessages != nil && len(castingMessages.CastStartedUnseen) > 0 {
		pool = castingMessages.CastStartedUnseen
	}
	var pick narration.Picker
	if len(picker) > 0 {
		pick = picker[0]
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pick).Actor
}
```

Read how `GetCastMessage` passes a nil picker to `narration.Render` and match it exactly (it may substitute `narration.DefaultPicker`).

- [ ] **Step 4: Data.** Append to `_datafiles/world/dogmud/casting-messages.yaml`:

```yaml

# The room's line when a spell authors no cast_observer / wait_observer
# (messaging M6 slice 4, R7). {actor} is the caster.
cast_started_room:
  - "{actor} begins weaving a spell."
  - "{actor} draws a slow breath and begins to shape a spell."
  - "{actor} goes still, gathering the threads of a spell."

cast_continuing_room:
  - "{actor} weaves magic with focused intent."
  - "{actor} keeps their focus, the spell slowly taking shape."
  - "{actor} works the threads of a spell, eyes fixed and distant."

# Heard at cast start by a reader who does not perceive a hidden caster (R5).
# Names nobody, says nothing of the target, fits any school.
cast_started_unseen:
  - "You hear a faint murmur nearby, though you see no one."
  - "The air prickles, as if someone close by is gathering power."
  - "A low whisper rises somewhere close, then fades."
```

Append the same three pools to `_datafiles/world/default/casting-messages.yaml`. In the dogmud file, rewrite the `concentration_slipped` line with the em dash: `"The mental pattern of {spell} dissolves as your focus fractures at the critical moment."`.

- [ ] **Step 5:** `go test ./internal/spells/ -count=1`, then `go test ./internal/narration/ -run TestSnapshotStores -count=1`. If the casting golden fails, read the diff: the only allowed change is the `concentration_slipped` line losing its dash. If so, rerun with `-update` and re-run without it to confirm green. Any other diff: stop and investigate.

- [ ] **Step 6: Commit** the two Go files, both YAML files and any regenerated golden under `internal/narration/testdata/stores/`, message `feat(spells): room and unseen casting pools`.

---

### Task 3: Spells and casting messages join the dash guard

**Files:** `prose_dash_guard_test.go`, `_datafiles/world/dogmud/spells/core-drain.yaml:7`

- [ ] **Step 1:** Add `"spells",` to `proseDashDirs`. The guard walks directories; add a sibling list and loop for single files:

```go
// proseDashFiles are bare player-text files at the world root.
var proseDashFiles = []string{
	"casting-messages.yaml",
}
```

Refactor the per-file scanning closure in `TestPlayerProseHasNoDashes` into a local `scanFile := func(path string) error` used by both the directory walk and a loop over `proseDashFiles` (each file must exist, or `t.Fatalf`).

- [ ] **Step 2:** `go test . -run TestPlayerProseHasNoDashes -count=1` fails on `core-drain.yaml:7` (and any other file it names; fix every one).

- [ ] **Step 3:** `core-drain.yaml` lines 7-8:

```yaml
  its own cold fire. Mob-castable only; this is the Guardian's
  signature recharge, not a spell any living caster could learn.
```

- [ ] **Step 4:** guard `ok`. **Commit** `prose_dash_guard_test.go _datafiles/world/dogmud/spells/core-drain.yaml`, message `copy(spells): no dashes; the dash guard covers spells and casting messages`.

---

### Task 4: Perception helpers move to `actions`

**Files:** create `internal/actions/perception.go`, `internal/actions/perception_test.go`; modify `internal/hooks/NewRound_DoCombat_helpers.go:405-436`, `internal/hooks/Condition_ApplyConditions.go:245-250`, `internal/hooks/Logout_AwarenessCleanup.go:63-71`

- [ ] **Step 1: Failing test.** `internal/actions/perception_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The perception helpers answer "who must not read a line about this
// character" once, for hooks and actions alike (messaging M6 slice 4).
func TestUnseenBy_ListsOnlyPlayersWhoDoNotPerceiveAHiddenCharacter(t *testing.T) {
	seedParityWorld(t)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7371: users.NewTestUser(7371, "ana", "Ana", 97371),
		7372: users.NewTestUser(7372, "bo", "Bo", 97372),
	}))
	// hiddenMobRoom (search_stealth_test.go) builds room 7370 holding a
	// hidden mob, instance 98371.
	room := hiddenMobRoom(t, 7370, 98371, 50)
	for _, id := range []int{7371, 7372} {
		room.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	hidden := mobs.GetInstance(98371)
	if hidden == nil {
		t.Fatal("fixture mob missing")
	}
	if got := UnseenBy(room, &hidden.Character); len(got) != 2 {
		t.Fatalf("neither player perceives the hidden mob, want both listed, got %v", got)
	}
	hidden.Character.Awareness = nil // IsHidden reads the Awareness FSM: nil is visible
	if got := UnseenBy(room, &hidden.Character); got != nil {
		t.Fatalf("a visible character is unseen by nobody, got %v", got)
	}
}
```

- [ ] **Step 2:** `go test ./internal/actions/ -run UnseenBy -count=1` fails: `undefined: UnseenBy`.

- [ ] **Step 3: `internal/actions/perception.go`.** Move the bodies (comments with them) from hooks, exported:

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The one answer to "who perceives this character" and "what does a reader
// call this mob" for every line about an act (messaging M6 slice 4). The
// hooks package's mobDisplayName, mobSeenName, mobHiddenFrom,
// playersNotPerceiving and conditionLineUnseenBy delegate here.

// MobDisplayName is a mob's name in a line about what it did or had done to
// it, as viewingUserId reads it: "something" for a hidden mob the viewer does
// not perceive, else MobSeenName. Viewer 0 is the room as a whole.
func MobDisplayName(mob *mobs.Mob, room *rooms.Room, viewingUserId int) string {
	if MobHiddenFrom(mob, viewingUserId) {
		return messaging.UnseenFigure(messaging.SightNone)
	}
	return MobSeenName(mob, room, viewingUserId)
}

// MobSeenName is MobDisplayName for a reader known to perceive the mob: the
// identity tag with its duplicate index, without adjectives, quest star or
// pet name.
func MobSeenName(mob *mobs.Mob, room *rooms.Room, viewingUserId int) string {
	dupIdx := room.GetMobDuplicateIndex(mob.InstanceId)
	f := mob.Character.GetMobNameIndexed(viewingUserId, dupIdx)
	f.Adjectives = nil
	f.QuestAlert = false
	f.PetName = ``
	return f.String()
}

// MobHiddenFrom reports whether a line about mob must not name it to
// viewingUserId. Viewer 0 perceives no hidden mob.
func MobHiddenFrom(mob *mobs.Mob, viewingUserId int) bool {
	if !mob.Character.IsHidden() {
		return false
	}
	if viewingUserId == 0 {
		return true
	}
	u := users.GetByUserId(viewingUserId)
	return u == nil || !u.Character.Perceives(&mob.Character)
}

// PlayersNotPerceiving lists the players in room who do not perceive c.
func PlayersNotPerceiving(room *rooms.Room, c *characters.Character) []int {
	var ids []int
	for _, uid := range room.GetPlayers() {
		if v := users.GetByUserId(uid); v != nil && !v.Character.Perceives(c) {
			ids = append(ids, uid)
		}
	}
	return ids
}

// UnseenBy lists the players in room who must not read a line about an act
// of c's own: nil for a visible c (no room walk) or a nil room, else
// PlayersNotPerceiving (owner ruling R3, #274).
func UnseenBy(room *rooms.Room, c *characters.Character) []int {
	if room == nil || c == nil || !c.IsHidden() {
		return nil
	}
	return PlayersNotPerceiving(room, c)
}
```

Copy the existing doc comments' substance from the hooks originals (they carry #382, #454, #458 history) rather than the short versions above where they say more.

- [ ] **Step 4: Hooks delegate.** Each hooks function keeps its name and doc comment; its body becomes one line: `return actions.MobDisplayName(mob, room, viewingUserId)`, `return actions.MobSeenName(mob, room, viewingUserId)`, `return actions.MobHiddenFrom(mob, viewingUserId)`, `return actions.PlayersNotPerceiving(room, c)`, and `conditionLineUnseenBy`'s body `return actions.UnseenBy(r, holder)`. Add a sentence to each comment: "The body lives in actions (messaging M6 slice 4)."

- [ ] **Step 5:** `go build ./... && go test ./internal/actions/ ./internal/hooks/ -count=1`: all `ok`. The existing hooks tests (`hidden_mob_room_lines_test.go`, `spell_channel_sight_test.go`, `hooks_test.go`, `spell_effects_test.go`) are the regression net for the move.

- [ ] **Step 6: Commit** the five files, message `refactor(actions): one home for the perception helpers`.

---

### Task 5: `actions.NarrateSpellPhase`

**Files:** create `internal/actions/spell_phase_lines.go`, `internal/actions/spell_phase_lines_test.go`

- [ ] **Step 1: Failing tests.** Create `internal/actions/spell_phase_lines_test.go`:

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

// Messaging M6 slice 4: every caster's own spell lines go through one body.

const (
	phaseCasterId    = 7351
	phaseTargetId    = 7352
	phaseBystanderId = 7353
	phaseMobId       = 98351
	phaseInfraredId  = 7361
	phaseSeeHiddenId = 7362
)

func phaseSpell(withActee bool) *spells.SpellData {
	s := &spells.SpellData{
		SpellId: "test-spike", Name: "Test Spike", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		CastUserText: "You focus a spike.", CastRoomText: "{actor} stares intently.",
		WaitUserText: "You hold the spike.", WaitRoomText: "{actor} trembles with effort.",
	}
	if withActee {
		s.CastActeeText = "{actor} stares at you."
	}
	return s
}

// bareSpell authors no text at all, so every room line is the generic pool.
func bareSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "test-bare", Name: "Test Bare", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle}
}

// phaseScene stands a caster (7351), a target (7352) and a bystander (7353)
// in one room. Infrared, when asked, goes to the target and the bystander,
// so a dark cave leaves them at shapes.
func phaseScene(t *testing.T, biome string, infrared bool) *rooms.Room {
	t.Helper()
	seedParityWorld(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		phaseInfraredId: {ConditionId: phaseInfraredId, Name: "Test Heat Eyes",
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		phaseSeeHiddenId: {ConditionId: phaseSeeHiddenId, Name: "Test Keen Eyes",
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		"city": {BiomeId: "city", Lamp: rooms.LampPtr(90)},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		phaseCasterId:    users.NewTestUser(phaseCasterId, "kael", "Kael", 97351),
		phaseTargetId:    users.NewTestUser(phaseTargetId, "mira", "Mira", 97352),
		phaseBystanderId: users.NewTestUser(phaseBystanderId, "tob", "Tob", 97353),
	}))
	if infrared {
		for _, id := range []int{phaseTargetId, phaseBystanderId} {
			if !users.GetByUserId(id).Character.Conditions.AddCondition(phaseInfraredId, true) {
				t.Fatal("precondition: infrared")
			}
		}
	}
	room := &rooms.Room{RoomId: 7350, Biome: biome}
	for _, id := range []int{phaseCasterId, phaseTargetId, phaseBystanderId} {
		room.AddPlayer(id)
		users.GetByUserId(id).Character.RoomId = 7350
		events.DrainQueuedMessagesForTest(id)
	}
	return room
}

func phaseHide(t *testing.T, c *characters.Character) {
	t.Helper()
	c.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "spell_phase_lines_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	if !c.IsHidden() {
		t.Fatal("fixture is not hidden")
	}
}

func phaseMob(t *testing.T, room *rooms.Room, hidden bool) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: phaseMobId}
	m.Character.Name = "Shaman"
	m.Character.Conditions = conditions.New()
	m.Character.RoomId = room.RoomId
	if hidden {
		phaseHide(t, &m.Character)
	}
	mobs.SetInstanceForTest(phaseMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(phaseMobId, nil) })
	room.AddMob(phaseMobId)
	return m
}

func phaseLines(uid int) []string { return drinkSightPlain(events.DrainQueuedMessagesForTest(uid)) }

func castAt(caster Actor, s *spells.SpellData, phase spells.Phase, room *rooms.Room, targets ...int) {
	NarrateSpellPhase(caster, s, phase, targets, nil, room, messaging.CategorySpellFold)
}

func TestSpellPhase_CastTargetReadsActeeBystanderReadsObserver(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)

	requireOneLine(t, "caster", phaseLines(phaseCasterId), "You focus a spike.")
	requireOneLine(t, "target", phaseLines(phaseTargetId), "Kael stares at you.")
	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael stares intently.")
}

func TestSpellPhase_UnauthoredActeeLeavesTheTargetTheRoomLine(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(false), spells.PhaseCast, room, phaseTargetId)

	requireOneLine(t, "target", phaseLines(phaseTargetId), "Kael stares intently.")
}

func TestSpellPhase_SelfCastSendsNoActee(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast, room, phaseCasterId)

	requireOneLine(t, "caster", phaseLines(phaseCasterId), "You focus a spike.")
	requireOneLine(t, "other player", phaseLines(phaseTargetId), "Kael stares intently.")
}

// R1: darkness hides the caster's name in the target's line; it does not
// silence the line.
func TestSpellPhase_TargetLineHidesTheCasterBySight(t *testing.T) {
	for _, tc := range []struct {
		name     string
		infrared bool
		want     string
	}{{"shapes", true, "A figure stares at you."}, {"none", false, "Something stares at you."}} {
		t.Run(tc.name, func(t *testing.T) {
			room := phaseScene(t, "cave", tc.infrared)
			castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)
			requireOneLine(t, "target", phaseLines(phaseTargetId), tc.want)
		})
	}
}

// R7: a phase with no authored room line reads the generic pool line, for a
// player caster as for a mob.
func TestSpellPhase_GenericRoomLineForAnyCaster(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), bareSpell(), spells.PhaseCast, room, phaseTargetId)
	requireOneLine(t, "bystander, player cast", phaseLines(phaseBystanderId), "Kael begins weaving a spell.")

	m := phaseMob(t, room, false)
	castAt(NewMobActorInRoom(m, room), bareSpell(), spells.PhaseWait, room)
	got := phaseLines(phaseBystanderId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "weaves magic with focused intent.") {
		t.Fatalf("bystander, mob fold round: %q", got)
	}
}

// A fold round's room line is sight-only: the audio leak at the old player
// wait sites is gone.
func TestSpellPhase_WaitLineIsSightOnly(t *testing.T) {
	room := phaseScene(t, "cave", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(false), spells.PhaseWait, room)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("a bystander who sees nothing read %q", got)
	}
	requireOneLine(t, "caster", phaseLines(phaseCasterId), "You hold the spike.")
}

// R3 + R5: a hidden caster's lines are silent to a reader who does not
// perceive them, who instead hears one unnamed line at cast start only.
func TestSpellPhase_HiddenCasterIsHeardNotSeenAtCastStart(t *testing.T) {
	room := phaseScene(t, "city", false)
	caster := users.GetByUserId(phaseCasterId)
	phaseHide(t, caster.Character)

	castAt(NewUserActorInRoom(caster, room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)

	unseen := spells.CastStartedUnseenLine()
	for _, id := range []int{phaseTargetId, phaseBystanderId} {
		got := phaseLines(id)
		if len(got) != 1 || got[0] != unseen {
			t.Fatalf("player %d read %q, want only the unseen line", id, got)
		}
	}

	castAt(NewUserActorInRoom(caster, room), phaseSpell(true), spells.PhaseWait, room, phaseTargetId)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("a fold round of a hidden caster must be silent, got %q", got)
	}
}

func TestSpellPhase_SeeHiddenReaderReadsTheNamedLines(t *testing.T) {
	room := phaseScene(t, "city", false)
	m := phaseMob(t, room, true)
	if !users.GetByUserId(phaseTargetId).Character.Conditions.AddCondition(phaseSeeHiddenId, true) {
		t.Fatal("precondition: see hidden")
	}
	if !users.GetByUserId(phaseTargetId).Character.Perceives(&m.Character) {
		t.Fatal("precondition: the target should perceive the hidden mob")
	}

	castAt(NewMobActorInRoom(m, room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)

	got := phaseLines(phaseTargetId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "stares at you.") {
		t.Fatalf("a perceiving target reads the named actee line, got %q", got)
	}
	if got := phaseLines(phaseBystanderId); len(got) != 1 || got[0] != spells.CastStartedUnseenLine() {
		t.Fatalf("a non-perceiving bystander hears the unseen line, got %q", got)
	}
}

func TestSpellPhase_MobTargetReadsNothingAndTheRoomReadsTheRoomLine(t *testing.T) {
	room := phaseScene(t, "city", false)
	m := phaseMob(t, room, false)
	NarrateSpellPhase(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast,
		nil, []int{m.InstanceId}, room, messaging.CategorySpellFold)
	requireOneLine(t, "bystander", phaseLines(phaseTargetId), "Kael stares intently.")
}
```

Add `"github.com/GoMudEngine/GoMud/internal/characters"` to the imports (for `phaseHide`). With the casting store unloaded in `actions` tests, `CastStartedUnseenLine()` is the fallback sentence, and `CastRoomPool` the two fallback sentences the tests name. If `conditions.SeeHidden` is not the flag `Perceives` reads via `HasFlagFromAnySource`, use whatever `characters.go:973` names.

- [ ] **Step 2:** `go test ./internal/actions/ -run SpellPhase -count=1` fails: `undefined: NarrateSpellPhase`.

- [ ] **Step 3: `internal/actions/spell_phase_lines.go`.**

```go
package actions

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// NarrateSpellPhase is the one sender of a caster's own spell lines, for
// every caster and phase (messaging M6 slice 4): the cast start, each fold
// round, and the magic line. Landing and disruption lines have their own
// bodies in hooks; root guard spell_phase_narration_guard_test.go keeps
// every other site from rendering a phase itself.
//
//   - A player caster reads the phase's *_actor line.
//   - Cast phase only: the first target, when it is a player other than the
//     caster and the spell authors cast_actee, reads cast_actee instead of the
//     room line, the caster hidden by the target's sight (owner ruling R1).
//   - The room reads the phase's *_observer line, or the generic pool line
//     when none is authored (R7), by sight only, names hidden by each
//     reader's sight.
//   - A hidden caster's lines skip every player who does not perceive them
//     (R3), and at cast start each of those players hears one unnamed
//     cast_started_unseen line instead (R5).
func NarrateSpellPhase(caster Actor, spell *spells.SpellData, phase spells.Phase,
	targetUserIds, targetMobIds []int, room *rooms.Room, cat messaging.Category) {

	if caster == nil || spell == nil || room == nil {
		return
	}
	cc := caster.GetCharacter()
	if cc == nil {
		return
	}
	var casterUser *users.UserRecord
	actorName, actorPlain := cc.GetCharacterName(true), cc.GetCharacterName(false)
	switch a := caster.(type) {
	case *UserActor:
		casterUser = a.User
	case *MobActor:
		actorName = MobSeenName(a.Mob, room, 0)
	}
	unseenBy := UnseenBy(room, cc)

	ctx := textutil.TokenContext{ActorName: actorName, ActorPlainName: actorPlain}
	var target *users.UserRecord
	if len(targetUserIds) > 0 {
		if tu := users.GetByUserId(targetUserIds[0]); tu != nil {
			ctx.ActeeName, ctx.ActeePlainName = tu.Character.GetCharacterName(true), tu.Character.GetCharacterName(false)
			if casterUser == nil || tu.UserId != casterUser.UserId {
				target = tu
			}
		}
	} else if len(targetMobIds) > 0 {
		if tm := mobs.GetInstance(targetMobIds[0]); tm != nil {
			ctx.ActeeName, ctx.ActeePlainName = MobDisplayName(tm, room, 0), tm.Character.GetCharacterName(false)
		}
	}

	// Authored roles render together (they are index-paired); the generic
	// room pool renders on its own, since Variants.Len is 0 for pools of
	// different lengths.
	roles := spell.Narrate(phase, ctx)
	observer := roles.Observer
	if observer == "" {
		if pool := spells.CastRoomPool(phase); len(pool) > 0 {
			observer = textutil.Narrate(narration.Variants{Observer: pool}, ctx).Observer
		}
	}

	aud := messaging.Audience{ActorName: actorName, ActeeName: ctx.ActeeName, Room: room}
	trio := messaging.Trio{}
	exclude := slices.Clone(unseenBy)
	if casterUser != nil {
		aud.Actor, aud.ActorId = casterUser, casterUser.UserId
		trio.Actor = messaging.Say(cat, roles.Actor)
		exclude = append(exclude, casterUser.UserId)
	}
	if phase == spells.PhaseCast && target != nil && roles.Actee != "" && !slices.Contains(unseenBy, target.UserId) {
		aud.Actee, aud.ActeeId = target, target.UserId
		trio.Actee = messaging.Say(cat, roles.Actee)
		exclude = append(exclude, target.UserId)
	}
	messaging.SendTrio(trio, aud)

	// The room line goes here rather than through SendTrio, whose observer
	// half cannot skip the readers who do not perceive a hidden caster.
	if observer != "" {
		room.SendTextVisualHidingNames(cat, observer, []string{actorName, ctx.ActeeName}, exclude...)
	}
	if phase == spells.PhaseCast {
		for _, uid := range unseenBy {
			if casterUser != nil && uid == casterUser.UserId {
				continue
			}
			if u := users.GetByUserId(uid); u != nil {
				u.SendText(cat, spells.CastStartedUnseenLine())
			}
		}
	}
}
```

Check `SendTextVisualHidingNames` tolerates an empty `ActeeName` in its names list (`messaging.NoName` is `""`, and `longestFirst` skips it).

- [ ] **Step 4:** `go test ./internal/actions/ -run SpellPhase -count=1 -v`: all PASS. Then `go test . -run TestEveryAudienceLiteralPairsIdsWithRecipients -count=1`: `ok`.

- [ ] **Step 5: Commit** the two files, message `feat(actions): NarrateSpellPhase, one body for a caster's spell lines`.

---

### Task 6: Cast-start sites through the body

**Files:** `internal/usercommands/skill.cast.go:310-339`, `internal/mobcommands/cast.go:118-140,173-174`, `internal/mobcommands/aid.go:58-73`

- [ ] **Step 1: Player.** Replace the whole `// 12b. Send YAML cast text (if defined).` block with:

```go
	// 12b. Cast-start lines through the one body every caster shares
	// (messaging M6 slice 4): the caster's, the target's own, the room's.
	if castRoom := rooms.LoadRoom(user.Character.RoomId); castRoom != nil {
		actions.NarrateSpellPhase(actions.NewUserActorInRoom(user, castRoom), spellInfo, spells.PhaseCast,
			result.TargetUserIds, result.TargetMobInstanceIds, castRoom, messaging.CategorySpellFold)
	}
```

- [ ] **Step 2: Mob.** Replace the `// Send YAML cast text (if defined).` block in `cast.go` with:

```go
	// Cast-start lines through the one body every caster shares (messaging M6
	// slice 4). A mob caster has no client, so it reads nothing.
	if castRoom := rooms.LoadRoom(mob.Character.RoomId); castRoom != nil {
		actions.NarrateSpellPhase(actions.NewMobActorInRoom(mob, castRoom), spellInfo, spells.PhaseCast,
			result.TargetUserIds, result.TargetMobInstanceIds, castRoom, messaging.CategorySpellFold)
	}
```

Delete the `room.SendTextVisual(... "begins weaving a spell." ...)` statement at lines 173-174; its line is now the generic `cast_started_room` fallback, sent only when the spell authors no `cast_observer` (R7).

- [ ] **Step 3: Aid.** Replace its `// Send YAML cast text (if defined).` block with:

```go
	// Cast-start lines through the one body every caster shares (messaging M6
	// slice 4). The aided player is the target.
	if castRoom := rooms.LoadRoom(mob.Character.RoomId); castRoom != nil && spellInfo != nil {
		actions.NarrateSpellPhase(actions.NewMobActorInRoom(mob, castRoom), spellInfo, spells.PhaseCast,
			[]int{aidPlayerId}, nil, castRoom, messaging.CategorySpellVital)
	}
```

- [ ] **Step 4:** `go build ./... && go vet ./internal/usercommands/ ./internal/mobcommands/`; remove any import the build reports unused (`textutil`, `fmt`) with the Edit tool. `go test ./internal/usercommands/ ./internal/mobcommands/ -count=1`: `ok`. A test that pinned "begins weaving a spell" alongside an authored line is now wrong by design (R7): read it, and change its expectation only if it pins the doubled line.

- [ ] **Step 5: Commit** the three files, message `refactor(cast): cast starts go through NarrateSpellPhase`.

---

### Task 7: Fold-round sites through the body

**Files:** `internal/hooks/NewRound_DoCombat_helpers.go` (`handlePlayerFoldCasting` 707-722 and 850-866; `handleMobFoldCasting` 940 and 1023-1026; delete `sendMobWeaving` and `sendMobWindupWarning`), `internal/hooks/spell_channel_sight_test.go`, `internal/hooks/hidden_mob_room_lines_test.go`

- [ ] **Step 1: Retarget the existing tests first.** In both test files, replace each `sendMobWeaving(m, room)` with `sendMobFoldRoundForTest(m, room)` and add to `spell_channel_sight_test.go`:

```go
// sendMobFoldRoundForTest is a mob's fold-round line for a spell that
// authors none: the generic weave, through the one body (messaging M6
// slice 4).
func sendMobFoldRoundForTest(m *mobs.Mob, room *rooms.Room) {
	actions.NarrateSpellPhase(actions.NewMobActorInRoom(m, room),
		&spells.SpellData{SpellId: "test-weave", Name: "Test Weave"}, spells.PhaseWait,
		nil, nil, room, messaging.CategorySpellFold)
}
```

Run `go test ./internal/hooks/ -run 'WeaveIsSightOnly|LitRoomNamesTheCaster|OwnActsAreSilent' -count=1`: they must PASS already (the body from Task 5 sends the same line under the same rule). If one fails, compare the mob name rendering (`MobSeenName` vs the test's expectation) before changing anything.

- [ ] **Step 2: Player sites.** In `handlePlayerFoldCasting`, the CastComplete branch's `// Send YAML wait text (if defined).` block becomes:

```go
		// The fold round's lines through the one body (messaging M6 slice 4):
		// sight only, silent to readers who do not perceive a hidden caster.
		if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
			actions.NarrateSpellPhase(actions.NewUserActorInRoom(user, r), spellData, spells.PhaseWait,
				cs.TargetUserIds, cs.TargetMobInstanceIds, r, messaging.CategorySpellFold)
		}
```

and the StillCasting branch's block the same with `waitSpellInfo` in place of `spellData`.

- [ ] **Step 3: Mob sites.** In `handleMobFoldCasting`, the StillCasting case becomes:

```go
	case result.StillCasting:
		cs := result.CastingData
		actions.NarrateSpellPhase(actions.NewMobActorInRoom(mob, mobRoom), result.SpellData, spells.PhaseWait,
			cs.TargetUserIds, cs.TargetMobInstanceIds, mobRoom, messaging.CategorySpellFold)
```

and the CastComplete case gains, as its first statements after `spellData := result.SpellData`:

```go
		// The completing fold round reads as every other fold round, for a mob
		// as for a player (messaging M6 slice 4).
		actions.NarrateSpellPhase(actions.NewMobActorInRoom(mob, mobRoom), spellData, spells.PhaseWait,
			cs.TargetUserIds, cs.TargetMobInstanceIds, mobRoom, messaging.CategorySpellFold)
```

Check `result.CastingData` exists on the StillCasting result for mobs (it does for players at line 851); if not, read the casting data off `mob.Character.Activity.CastingData()`.

Delete `sendMobWeaving` and `sendMobWindupWarning`. `min_cast_rounds` spells keep their `wait_observer` warning: the body sends any authored `wait_observer` for every caster.

- [ ] **Step 4:** `go build ./... && go test ./internal/hooks/ -count=1`: `ok`. Grep `sendMobWeaving\|sendMobWindupWarning` across `internal/`: only the comment history may mention them; remove stale references in comments that describe them as live code.

- [ ] **Step 5: Commit** the three files, message `refactor(hooks): fold rounds go through NarrateSpellPhase; no audio leak`.

---

### Task 8: Magic sites through the body

**Files:** `internal/hooks/spell_resolution.go:195-221,683-685`

- [ ] **Step 1: Player.** Replace the `// --- Run spell script onMagic (if present) ---` text block (from `// Send YAML magic text (if defined).` through its closing brace) with:

```go
	// The magic line through the one body (messaging M6 slice 4).
	actions.NarrateSpellPhase(actions.NewUserActorInRoom(user, room), spellData, spells.PhaseMagic,
		cs.TargetUserIds, cs.TargetMobInstanceIds, room, spellSchoolCategory(spellData))
```

Keep the `// --- Run spell script onMagic (if present) ---` heading if code below it still belongs to it.

- [ ] **Step 2: Mob.** In `resolveMobSpell`, immediately before the final `return anyLanded, false`:

```go
	// The magic line, as a player's cast has it (messaging M6 slice 4).
	actions.NarrateSpellPhase(actions.NewMobActorInRoom(mob, room), spellData, spells.PhaseMagic,
		cs.TargetUserIds, cs.TargetMobInstanceIds, room, spellSchoolCategory(spellData))
```

- [ ] **Step 3:** `go build ./... && go test ./internal/hooks/ -count=1`: `ok`. No shipped spell authors `magic_*`, so no golden moves.

- [ ] **Step 4: Commit** `internal/hooks/spell_resolution.go`, message `refactor(hooks): magic lines go through NarrateSpellPhase`.

---

### Task 9: Guard: phase narration only in `actions`

**Files:** create `spell_phase_narration_guard_test.go`

- [ ] **Step 1: The guard.**

```go
package main

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// spellPhaseNarrationRE matches a site rendering a spell phase: a Narrate or
// Narration call with a spells.Phase constant, or (inside the body) with the
// phase parameter.
var spellPhaseNarrationRE = regexp.MustCompile(`\.(Narrate|Narration)\((spells\.Phase\w+|phase)\b`)

// TestSpellPhaseNarrationLivesInOneBody keeps a caster's own spell lines in
// actions.NarrateSpellPhase (messaging M6 slice 4). Seven sites once rendered
// them under four rules; a new site must call the body, not render a phase.
func TestSpellPhaseNarrationLivesInOneBody(t *testing.T) {
	const home = "internal/actions/spell_phase_lines.go"
	inside := 0
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel := filepath.ToSlash(path)
			if d.IsDir() || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") ||
				strings.HasPrefix(rel, "internal/spells/") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			sc := bufio.NewScanner(bytes.NewReader(src))
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			n := 0
			for sc.Scan() {
				n++
				if !spellPhaseNarrationRE.MatchString(sc.Text()) {
					continue
				}
				if rel == home {
					inside++
					continue
				}
				t.Errorf("%s:%d renders a spell phase itself: %s\n  call actions.NarrateSpellPhase instead",
					rel, n, strings.TrimSpace(sc.Text()))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if inside == 0 {
		t.Errorf("the pattern matched nothing in %s; the guard is blind, not the tree clean", home)
	}
}
```

- [ ] **Step 2: Prove it can fail.** Temporarily add `_ = spellInfo.Narration(spells.PhaseCast)` to `internal/mobcommands/cast.go`, run `go test . -run TestSpellPhaseNarrationLivesInOneBody -count=1`, see it FAIL naming that line, then remove the line with the Edit tool and see it PASS.

- [ ] **Step 3: Commit** `spell_phase_narration_guard_test.go`, message `test(guard): spell phase narration lives in NarrateSpellPhase`.

---

### Task 10: One hiding rule (R6)

**Files:** `internal/hooks/Awareness_Cascades.go:30-41`, `internal/hooks/spell_effects.go` (`commitHarmfulSpellAggro`), create `internal/hooks/spell_hiding_test.go`

- [ ] **Step 1: Failing tests.** Create `internal/hooks/spell_hiding_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/stretchr/testify/require"
)

// Messaging M6 slice 4, owner ruling R6: starting a cast never breaks
// hiding; a harmful spell landing does, for every caster.

func hiddenCaster(t *testing.T, casting bool) *characters.Character {
	t.Helper()
	c := characters.New() // OnCharacterCreated wires the Awareness cascade
	reason := state.TransitionReason{Trigger: "spell_hiding_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
	if casting {
		require.NoError(t, c.Activity.TransitionToCasting(activity.CastingData{
			SpellId: "test-spike", FoldsNeeded: 4, FoldsPerRound: 1, TotalConvictionCost: 10,
		}, state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	}
	return c
}

func TestCasterEnteringCombatForItsOwnFoldStaysHidden(t *testing.T) {
	c := hiddenCaster(t, true)
	targeting.Commit(c, state.ActorRef{UserId: 1}, targeting.ReasonAttack)
	require.True(t, c.IsHidden(), "a fold-only combat entry must not reveal the caster")
}

// Null probe: the same commit without a fold in hand reveals, so the test
// above can fail.
func TestNonCasterEnteringCombatIsRevealed(t *testing.T) {
	c := hiddenCaster(t, false)
	targeting.Commit(c, state.ActorRef{UserId: 1}, targeting.ReasonAttack)
	require.False(t, c.IsHidden())
}
```

`characters.New()` builds the `CombatPhase`, `Awareness` and `Activity` machines (`character.go:423-426`) and fires the `OnCharacterCreated` callbacks, which the hooks package's `init` registers `wireAwarenessFromCombatPhase` with (`Awareness_Cascades.go:81-83`), so the cascade is live on it. If `targeting.Commit` refuses because user 1 is not seeded, start each test with `t.Cleanup(seedAllRegistries())` (user 1 exists there).

Add the landing test, built on the existing fixtures:

```go
// A harmful landing reveals a hidden caster explicitly, even one already in
// combat, which makes no Idle to Engaging transition of its own.
func TestHarmfulLandingRevealsAHiddenMobCaster(t *testing.T) {
	seer, _, m := hiddenHolderFixture(t)
	sneakHide(t, &m.Character)
	room := rooms.LoadRoom(1)
	spell := &spells.SpellData{SpellId: "test-spike", Name: "Test Spike", AttackType: combatvocab.AttackSpell,
		DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle}
	c := newSpellEffectCtx(&m.Character, actions.NewMobActorInRoom(m, room),
		actions.NewUserActorInRoom(seer, room), room, spell, 100, uncontestedSpellResult())
	commitHarmfulSpellAggro(c, true)
	require.False(t, m.Character.IsHidden())
}
```

Read `newSpellEffectCtx`'s exact parameter list (`spell_effects.go:51`) and match it; add the imports this needs (`actions`, `combatvocab`, `rooms`, `spells`).

- [ ] **Step 2:** `go test ./internal/hooks/ -run 'StaysHidden|IsRevealed|RevealsAHidden' -count=1`: `StaysHidden` and `RevealsAHidden` FAIL, `IsRevealed` PASSES.

- [ ] **Step 3: Cascade.** In `wireAwarenessFromCombatPhase`'s Idle to Engaging handler, before the Hidden check:

```go
			// A caster entering combat only so its own fold steps (a mob's
			// harmful cast commits at cast start, #242) stays hidden: the
			// spell's landing reveals it (commitHarmfulSpellAggro, owner
			// ruling R6, messaging M6 slice 4).
			if c.Activity != nil && c.Activity.IsCasting() {
				return
			}
```

- [ ] **Step 4: Landing.** At the top of `commitHarmfulSpellAggro`:

```go
	// A harmful spell breaks its caster's hiding when it lands, for every
	// caster (owner ruling R6). Explicit, because a caster already in combat
	// makes no Idle to Engaging transition for the cascade to see.
	if cc := c.casterChar; cc != nil && cc.IsHidden() {
		_ = cc.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerCombatEntered,
			Metadata: map[string]any{"spell": c.spell.SpellId},
		})
	}
```

Add the `awareness` import if `spell_effects.go` lacks it.

- [ ] **Step 5:** `go test ./internal/hooks/ -count=1`: `ok`. Then `go test ./internal/... -count=1` once here: the cascade change is global, and a test elsewhere may cover a casting character being attacked.

- [ ] **Step 6: Commit** the three files, message `fix(stealth): a harmful spell breaks hiding when it lands, not when it starts`.

---

### Task 11: The `cast_actee` guard and the 35 lines

**Files:** create `spell_cast_actee_guard_test.go`; modify 35 files in `_datafiles/world/dogmud/spells/`

- [ ] **Step 1: Guard.**

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
// cast text at all (messaging M6 slice 4).
var castActeeExempt = map[string]bool{
	"fold-anchor":          true,
	"fold-recall":          true,
	"summon-hive-swarm":    true,
	"summon-steppe-spirit": true,
	"heal":                 true,
}

// TestCastActeeCoversEveryAimedSpell: every single-target spell that can land
// on someone else authors cast_actee, and no other spell does.
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

Note: `s.Narration(spells.PhaseCast)` here does not trip Task 9's guard, which skips `_test.go` files.

- [ ] **Step 2:** `go test . -run TestCastActeeCoversEveryAimedSpell -count=1`: FAIL, 35 errors plus the fatal.

- [ ] **Step 3: Lines.** Add one `cast_actee:` line to each file directly after its `cast_actor` value (after the last line of a folded `cast_actor: >-` block), double-quoted, exactly:

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

- [ ] **Step 4:** `go test . -run 'CastActee|ShippedNarration|PlayerProseHasNoDashes|ProseTagSpacing' -count=1`: `ok`. If the narration snapshot goldens the spells store, run `go test ./internal/narration/ -run TestSnapshotStores -count=1`; a diff that only adds the 35 `cast_actee` lines may be regenerated with `-update`.

- [ ] **Step 5: Shape check.** Write `$TMP/shapecheck.py` with the Write tool (the Bash tool collapses `\\`):

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
added = {"cast_actee", "cast_started_room", "cast_continuing_room", "cast_started_unseen"}
paths = sorted((root / "_datafiles/world/dogmud/spells").glob("*.yaml")) + [
    root / "_datafiles/world/dogmud/casting-messages.yaml",
    root / "_datafiles/world/default/casting-messages.yaml",
]
bad = 0
for p in paths:
    rel = p.relative_to(root).as_posix()
    old = subprocess.run(["git", "-C", str(root), "show", f"origin/master:{rel}"], capture_output=True, text=True)
    if old.returncode != 0:
        continue
    a, b = shape(yaml.safe_load(old.stdout)), shape(yaml.safe_load(p.read_text(encoding="utf-8")))
    for k in added:
        b.pop(k, None)
    if a != b:
        bad += 1
        print("shape changed:", rel)
print("files with changed shape:", bad)
```

Run `python -I "$TMP/shapecheck.py" "C:/tmp/dogmud-m6s4"`: `files with changed shape: 0`.

- [ ] **Step 6: Commit** the guard and the 35 spell files by name (check `git status --short` lists nothing else under `spells/`), message `copy(spells): cast_actee for the 35 spells aimed at one target`.

---

### Task 12: Docs

**Files:** `internal/spells/context.md`, `internal/actions/context.md`, `internal/hooks/context.md`, `docs/superpowers/audits/messaging-m6-content-ledger.md`, `docs/PATCH_NOTES.md`

- [ ] **Step 1: `internal/spells/context.md`.** "six text fields" becomes "seven"; "The six authored keys are" becomes "The seven authored keys are", inserting `` `cast_actee`, `` after `` `cast_actor` / `cast_observer`, ``; the sentence listing the sites that render through `Narrate` becomes "`actions.NarrateSpellPhase` is the one sender of every phase (root guard `spell_phase_narration_guard_test.go`)"; drop "The two shipped `wait_observer` lines still go out on the audio channel; that is filed". Append:

```markdown
`cast_actee` (messaging M6 slice 4) is the target's own cast-start line,
the `Actee` variant of `Narration(PhaseCast)`. Every single-target spell that
can land on someone else authors one (root guard
`spell_cast_actee_guard_test.go`). `casting-messages.yaml` also carries
`cast_started_room` and `cast_continuing_room`, the room's line when a spell
authors no `cast_observer` / `wait_observer` (`CastRoomPool`), and
`cast_started_unseen`, what a reader who does not perceive a hidden caster
hears at cast start (`CastStartedUnseenLine`).
```

- [ ] **Step 2: `internal/actions/context.md`.** Add:

```markdown
## Spell phase lines and perception

`NarrateSpellPhase(caster, spell, phase, targetUserIds, targetMobIds, room,
cat)` sends a caster's own lines for the cast start, each fold round and the
magic line, for every caster (player cast, mob cast, mob aid, the fold-round
loops, both resolvers). The caster reads `*_actor`; at cast start a player
target other than the caster reads `cast_actee`; the room reads
`*_observer` or the generic pool line, by sight only. A hidden caster's lines
skip the players who do not perceive them, and at cast start those players
hear one `cast_started_unseen` line.

`MobDisplayName`, `MobSeenName`, `MobHiddenFrom`, `PlayersNotPerceiving` and
`UnseenBy` are the one answer to who perceives a character and what a reader
calls a mob; `hooks` delegates its helpers of the same names here.
```

- [ ] **Step 3: `internal/hooks/context.md`.** Where it lists `mobDisplayName` / `conditionLineUnseenBy` or the spell-channel helpers, note that their bodies live in `actions` and that `sendMobWeaving` / `sendMobWindupWarning` are gone (fold rounds go through `actions.NarrateSpellPhase`). Add one line under the Awareness cascade: a casting character's Idle to Engaging does not reveal; `commitHarmfulSpellAggro` reveals the caster of a harmful spell.

- [ ] **Step 4: Ledger row 9** Status cell:

```
done (PR #___, M6 slice 4): landing lines already existed as Go templates in every applier and every heal spell carries a condition, so the gaps were the cast start and seven sites narrating a caster's own lines under four rules; actions.NarrateSpellPhase now sends them all, cast_actee is authored for 35 spells, a hidden caster's cast start is heard unnamed, and a harmful spell breaks hiding when it lands
```

- [ ] **Step 5: PATCH_NOTES**, under the first `## Unreleased` heading:

```markdown
- When someone starts a spell aimed at you, you now read a line of your own
  telling you so.
- A hidden caster starting a spell nearby is heard but not seen: a faint
  murmur or a prickle in the air, with no name and no hint of the target.
  A hidden caster now stays hidden until a harmful spell lands.
- When a player starts casting, the room now sees it begin, as it always
  has for creatures.
```

- [ ] **Step 6:** `python -I tools/context_md_audit.py`: no phantom symbols in `internal/spells`, `internal/actions`, `internal/hooks`. **Commit** the five files, message `docs: M6 slice 4, NarrateSpellPhase, cast_actee, hiding rule`.

---

### Task 13: Gate, PR, playtest

- [ ] **Step 1:** `go test ./... -count=1` from the worktree root: every package `ok`.
- [ ] **Step 2:** `golangci-lint run --new-from-merge-base=origin/master`: `0 issues.`
- [ ] **Step 3:** The `dogmud-shipping` skill's detached-worktree boot check on a private port: the casting-messages and spells stores load without a validation panic. Kill only the PID you started.
- [ ] **Step 4:** Push and open the PR:

```bash
git push -u origin feat/m6-slice4
gh pr create --repo pruuk/DOGMud --base master --head feat/m6-slice4 --title "M6 slice 4: one body for a caster's spell lines" --body-file "$TMP/pr4.md"
```

Write `$TMP/pr4.md` with the Write tool: the seven sites unified, the target line, R5 unseen sound, R6 hiding rule, R7 generic lines for players, the three audio leaks closed; `Ledger row 9 (#370)` with no closing keyword; end with the attribution line. Then fill `#___` in the ledger and push that commit.

- [ ] **Step 5: Playtest** via the `dogmud-playtesting` skill, ephemeral goals file, `--checkout feat/m6-slice4`, `mid` profile. Goals: (1) a hostile caster starts a single-target spell at the player in a lit room (pick a mob whose `spells:` list holds mind-spike, neural-stun or kinetic-hurl; search mob YAML): record the cast-start line the player reads; (2) the same in a dark room; (3) the player casts a spell with no `cast_observer` and a second agent or the log confirms the room line; (4) if the harness can hide a mob (a sneaking mob, or admin hide), its cast start reads as the unseen sound. File findings as `pruuk/DOGMud` issues.
