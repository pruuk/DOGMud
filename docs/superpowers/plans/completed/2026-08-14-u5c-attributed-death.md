# U5c Attributed Death Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move death from the deferred round-tick sweep to the moment harm lands, carrying a real killer reference and the overkill magnitude, keeping the sweep only as a backstop for paths that never call `ApplyHarm`.

**Architecture:** `ApplyHarm` detects lethal health harm and queues an `events.CharacterDied` instead of killing inline, because `Die` despawns mobs synchronously and would remove instances from under any loop damaging several targets. A listener resolves the death, running centralised prechecks first. Two states are kept strictly separate: **dying** (`Health < 1 && IsAlive()`) drives targeting and coup de grace rendering, while **`DeathQueued`** marks that an event is in flight and is the only thing the sweeps skip on.

**Tech Stack:** Go 1.25, `internal/characters` (pools, Life machine), `internal/events` (queue + listeners), `internal/hooks` (listeners and Life observers), YAML message pools under `_datafiles/world/dogmud/`.

**Spec:** `docs/superpowers/specs/completed/2026-08-14-u5c-attributed-death-design.md`
**Roadmap:** `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` (slice U5c)

---

## Read this before Task 1

**The trap that makes this design silently fail.** "Dying" and "death queued" are different states. A character reaped by a sweep is *dying but not queued* — it reached 0 HP without going through `ApplyHarm`. If a sweep skips on `Health <= 0` it skips exactly the population it exists to reap, and nothing ever dies on the non-harm paths. **Sweeps skip on `DeathQueued` only.** Task 5 has the test that catches this.

**Why the event carries plain ints, not `state.ActorRef`.** Every existing event in `internal/events/eventtypes.go` uses plain ids (see `MobDeath.KillerMobInstanceId`). Keeping that convention avoids adding a `state` import to the events package. The listener rebuilds the `ActorRef`.

**Verified before planning, do not re-litigate:**
- `internal/characters` already imports `internal/events` (`progression.go:10`, `validate.go:11`), and `internal/events` does not import `internal/characters`. No cycle.
- `Die` is idempotent: it returns immediately when `!c.IsAlive()`.
- The `Death_*.go` files in `internal/hooks` are **Life-machine observers** wired via `c.Life.Inner().AfterTransition(...)`. They are NOT event listeners. The new listener follows the `<Event>_<Action>.go` naming used by `Buff_ApplyBuffs.go`, so it is named `CharacterDied_RouteDeath.go`.
- `Death_MobKillCredit` reads `DeadData.DamageMap`, not `DeadData.Killer`. **Mob kill credit already works.** This slice does not touch it.

**Arc standing rules:** no balance literal under `internal/` (config only); `context.md` updates ship in this PR, not a follow-up.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/events/eventtypes.go` | **Modify.** Add the `CharacterDied` event. |
| `internal/characters/character.go` | **Modify.** Add the `DeathQueued` runtime field. |
| `internal/characters/pools.go` | **Modify.** `ApplyHarm` detects lethal harm and queues. |
| `internal/characters/pools_death_test.go` | **Create.** Detection, overkill, single-fire, pool scoping. |
| `internal/hooks/CharacterDied_RouteDeath.go` | **Create.** The listener: resolve, precheck, `Die`. |
| `internal/hooks/CharacterDied_RouteDeath_test.go` | **Create.** Revive, idempotence, nil victim. |
| `internal/hooks/hooks.go` | **Modify.** Register the listener. |
| `internal/hooks/NewRound_DoCombat.go` | **Modify.** Sweep skips on `DeathQueued`; delete two redundant inline `Die` calls. |
| `internal/hooks/NewRound_MobRoundTick.go` | **Modify.** Sweep skips on `DeathQueued`. |
| `internal/hooks/NewRound_AutoHeal.go` | **Modify.** Delete the redundant anonymous `Die`. |
| `internal/hooks/Buff_ApplyBuffs.go` | **Modify.** Delete the redundant anonymous `Die`. |
| `internal/hooks/sweep_backstop_test.go` | **Create.** The dying-vs-queued distinction. |
| `internal/items/itemspec.go` | **Modify.** Add the `coupdegrace` intensity. |
| `internal/items/attack_messages.go` | **Modify.** Guard the generic fallback against infinite recursion. |
| `_datafiles/world/dogmud/combat-messages/generic.yaml` | **Modify.** Fallback coup de grace block; any weapon file may override. |
| `internal/combat/coup_de_grace.go` | **Create.** `IsDying` only. |
| `internal/combat/coup_de_grace_test.go` | **Create.** Dying gate, generic pool present, recursion guard. |
| `internal/characters/die.go` | **Modify.** Delete the phantom Shadow Realm precondition. |
| `internal/characters/context.md`, `internal/hooks/context.md` | **Modify.** Document the new path. |
| `docs/PATCH_NOTES.md` | **Modify.** Player-facing entry. |

---

### Task 1: Add the `CharacterDied` event and the `DeathQueued` field

**Files:**
- Modify: `internal/events/eventtypes.go`
- Modify: `internal/characters/character.go:285`

- [ ] **Step 1: Add the event type**

Append to `internal/events/eventtypes.go`, after the `MobDeath` block:

```go
// CharacterDied fires when harm drives a character's health below 1. The death
// itself is resolved by the CharacterDied listener, NOT at the harm site,
// because Die despawns mobs synchronously (Death_MobInstanceCleanup) and would
// remove instances from under any loop damaging several targets — the AoE loop
// in usercommands.Throw is a live example.
//
// Killer is carried as plain ids rather than a state.ActorRef to match every
// other event in this file and to keep the events package free of a state
// import. The listener rebuilds the ActorRef.
//
// A zero killer is meaningful: environmental harm with no source is anonymous
// by truth, which is a different thing from the anonymity-by-accident this
// slice removes.
type CharacterDied struct {
	UserId        int // victim, if a player
	MobInstanceId int // victim, if a mob

	KillerUserId        int
	KillerMobInstanceId int

	Overkill int    // how far below zero the LETHAL blow drove health
	Trigger  string // life.TriggerHealthZero
}

func (c CharacterDied) Type() string { return `CharacterDied` }
```

- [ ] **Step 2: Add the runtime field**

In `internal/characters/character.go`, immediately after the `LastSuicideRound` line (currently line 285):

```go
	DeathQueued             bool                           `yaml:"-"` // runtime only — a CharacterDied event is in flight for this character. NOT the same as "dying" (Health < 1 && IsAlive()); sweeps skip on THIS, never on health, or they skip the population they exist to reap. See the U5c plan.
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/events/eventtypes.go internal/characters/character.go
git commit -m "feat(events): add CharacterDied and the DeathQueued marker (U5c)"
```

---

### Task 2: `ApplyHarm` queues the death

**Files:**
- Modify: `internal/characters/pools.go` (`ApplyHarm`, currently line 222)
- Test: `internal/characters/pools_death_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/characters/pools_death_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// newHarmTestChar builds a character with a known health pool.
func newHarmTestChar(health int) *Character {
	c := New()
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = health
	return c
}

func TestApplyHarm_LethalHealthHarmQueuesExactlyOneDeath(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest() // discard leftovers
	c := newHarmTestChar(5)

	c.ApplyHarm(PoolHealth, 25, state.ActorRef{MobInstanceId: 42})

	got := events.DrainQueuedCharacterDiedForTest()
	if len(got) != 1 {
		t.Fatalf("queued %d CharacterDied events, want 1", len(got))
	}
	if got[0].KillerMobInstanceId != 42 {
		t.Errorf("KillerMobInstanceId = %d, want 42", got[0].KillerMobInstanceId)
	}
	if got[0].Overkill != 20 {
		t.Errorf("Overkill = %d, want 20 (health 5 minus a 25 blow)", got[0].Overkill)
	}
	if !c.DeathQueued {
		t.Error("DeathQueued not set")
	}
}

func TestApplyHarm_NonLethalQueuesNothing(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest()
	c := newHarmTestChar(50)

	c.ApplyHarm(PoolHealth, 10, state.ActorRef{MobInstanceId: 42})

	if got := events.DrainQueuedCharacterDiedForTest(); len(got) != 0 {
		t.Fatalf("queued %d events on non-lethal harm, want 0", len(got))
	}
	if c.DeathQueued {
		t.Error("DeathQueued set by non-lethal harm")
	}
}

// The second blow lands on an already-dying target. It must not queue a second
// death, or the victim is attributed and reaped twice.
func TestApplyHarm_SecondLethalBlowDoesNotRequeue(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest()
	c := newHarmTestChar(5)

	c.ApplyHarm(PoolHealth, 25, state.ActorRef{MobInstanceId: 42})
	c.ApplyHarm(PoolHealth, 25, state.ActorRef{MobInstanceId: 99})

	got := events.DrainQueuedCharacterDiedForTest()
	if len(got) != 1 {
		t.Fatalf("queued %d events, want 1 — the first lethal blow wins", len(got))
	}
	if got[0].KillerMobInstanceId != 42 {
		t.Errorf("killer = %d, want 42 — the lethal blow, not the last swing",
			got[0].KillerMobInstanceId)
	}
}

// Stamina and conviction floor at 0 and are never lethal, at any magnitude.
func TestApplyHarm_NonHealthPoolsNeverQueue(t *testing.T) {
	for _, pool := range []Pool{PoolStamina, PoolConviction} {
		events.DrainQueuedCharacterDiedForTest()
		c := newHarmTestChar(100)
		c.StaminaMax.Base = 100
		c.StaminaMax.Recalculate()
		c.Stamina = 5
		c.ConvictionMax.Base = 100
		c.ConvictionMax.Recalculate()
		c.Conviction = 5

		c.ApplyHarm(pool, 9999, state.ActorRef{MobInstanceId: 42})

		if got := events.DrainQueuedCharacterDiedForTest(); len(got) != 0 {
			t.Errorf("pool %s queued a death", pool)
		}
	}
}
```

- [ ] **Step 2: Add the test drain helper**

Add to `internal/events/events.go`, beside `DrainQueuedInputsForTest` (line 268). This mirrors that function exactly, including the `heap.Init` that the priority queue needs after a rebuild:

```go
// DrainQueuedCharacterDiedForTest removes all CharacterDied events from the
// global queue and returns them. Test-only. Mirrors DrainQueuedInputsForTest.
func DrainQueuedCharacterDiedForTest() []CharacterDied {
	qLock.Lock()
	defer qLock.Unlock()

	var found []CharacterDied
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if d, ok := pe.event.(CharacterDied); ok {
			found = append(found, d)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}
```

Draining removes the events, so each test starts clean by calling it once to discard leftovers and again to assert. The Step 1 tests are already written against this helper.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/characters/ -run TestApplyHarm_ -v`
Expected: FAIL — `c.DeathQueued` compiles (Task 1) but no event is ever queued, so `len(got)` is 0 where 1 is wanted.

- [ ] **Step 4: Implement the detection**

In `internal/characters/pools.go`, replace the body of `ApplyHarm`:

```go
func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int {
	if amount <= 0 {
		return 0
	}

	applied := -c.applyVitalChange(pool, -amount)

	// U5c: lethal health harm queues an attributed death rather than killing
	// inline. Inline would despawn a mob synchronously and pull the instance
	// out from under any loop damaging several targets.
	//
	// The DeathQueued guard is what makes the killing blow fire exactly once:
	// later hits that round still land and still render (coup de grace), but
	// they do not re-queue and do not re-attribute.
	if pool == PoolHealth && !c.DeathQueued && c.Health < 1 && c.IsAlive() {
		c.DeathQueued = true

		overkill := 0
		if c.Health < 0 {
			overkill = -c.Health
		}

		events.AddToQueue(events.CharacterDied{
			UserId:              c.GetUserId(),
			MobInstanceId:       c.MobInstanceId,
			KillerUserId:        source.UserId,
			KillerMobInstanceId: source.MobInstanceId,
			Overkill:            overkill,
			Trigger:             life.TriggerHealthZero,
		})
	}

	return applied
}
```

Add the imports `"github.com/GoMudEngine/GoMud/internal/events"` and `"github.com/GoMudEngine/GoMud/internal/state/life"` to `pools.go`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/characters/ -run TestApplyHarm_ -v`
Expected: PASS, four tests.

- [ ] **Step 6: Run the whole package**

Run: `go test ./internal/characters/`
Expected: `ok`. If an existing test now fails, stop: U5c changes behaviour only at named sites and an unexpected failure means the detection is firing somewhere it should not.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/pools.go internal/characters/pools_death_test.go internal/events/events.go
git commit -m "feat(characters): ApplyHarm queues an attributed death (U5c)"
```

---

### Task 3: The listener

**Files:**
- Create: `internal/hooks/CharacterDied_RouteDeath.go`
- Test: `internal/hooks/CharacterDied_RouteDeath_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/CharacterDied_RouteDeath_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
)

func TestRouteAttributedDeath_WrongEventTypeIsRejected(t *testing.T) {
	if got := RouteAttributedDeath(events.Buff{}); got != events.Cancel {
		t.Errorf("got %v, want Cancel for a mismatched event type", got)
	}
}

func TestRouteAttributedDeath_UnknownVictimIsInert(t *testing.T) {
	got := RouteAttributedDeath(events.CharacterDied{MobInstanceId: 999999})
	if got != events.Continue {
		t.Errorf("got %v, want Continue when the victim is already gone", got)
	}
}

// ReviveOnDeath must heal, cancel the buff, clear DeathQueued and NOT die.
// Leaving DeathQueued set would make the character permanently unkillable;
// leaving health negative would just hand the kill to the sweep next tick.
func TestRouteAttributedDeath_ReviveHealsAndClearsQueue(t *testing.T) {
	mob := newRouteDeathTestMob(t, -20)
	mob.Character.AddBuff(deathProtectionBuffId, `test`)
	if !mob.Character.HasBuffFlag(buffs.ReviveOnDeath) {
		t.Fatal("precondition: buff did not apply the ReviveOnDeath flag")
	}
	mob.Character.DeathQueued = true

	RouteAttributedDeath(events.CharacterDied{MobInstanceId: mob.InstanceId})

	if !mob.Character.IsAlive() {
		t.Error("revive did not prevent the death")
	}
	if mob.Character.Health < 1 {
		t.Errorf("health = %d, want positive — a revived character left dying is reaped by the sweep",
			mob.Character.Health)
	}
	if mob.Character.DeathQueued {
		t.Error("DeathQueued still set after a revive; the character can never be killed again")
	}
	if mob.Character.HasBuffFlag(buffs.ReviveOnDeath) {
		t.Error("revive buff was not consumed")
	}
}
```

Add the helper to the same file. A mob that is not in the instance registry cannot be resolved by `mobs.GetInstance`, so the test would pass for the wrong reason — `SeedMobsForTest` is what registers it:

```go
// newRouteDeathTestMob builds a mob and registers it in the instance registry
// so mobs.GetInstance can resolve it, restoring the registry on cleanup.
func newRouteDeathTestMob(t *testing.T, health int) *mobs.Mob {
	t.Helper()

	m := &mobs.Mob{
		MobId:      1,
		InstanceId: 90001,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:      "Route-Death-Dummy",
			RoomId:    1,
			Health:    health,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	m.Character.HealthMax.Base = 100
	m.Character.HealthMax.Recalculate()

	cleanup := mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{m.InstanceId: m})
	t.Cleanup(cleanup)

	return m
}

// deathProtectionBuffId is the buff carrying the ReviveOnDeath flag,
// _datafiles/world/default/buffs/35-death_protection.yaml.
const deathProtectionBuffId = 35
```

`SeedMobsForTest(specs, instances map[int]*Mob) func()` is in `internal/mobs/test_helpers.go` and returns the restore closure.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hooks/ -run TestRouteAttributedDeath -v`
Expected: FAIL to build — `RouteAttributedDeath` undefined.

- [ ] **Step 3: Implement the listener**

Create `internal/hooks/CharacterDied_RouteDeath.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RouteAttributedDeath resolves a death queued by ApplyHarm.
//
// Death is resolved HERE rather than at the harm site because Die fires its
// observers synchronously and Death_MobInstanceCleanup despawns the instance
// inside that call. Killing inline would pull instances out from under any
// loop damaging several targets.
//
// This is also the single place the prechecks Die's doc used to delegate to
// callers now live. They were not in fact handled at each call site: only the
// suicide commands checked ReviveOnDeath, so the buff was inert on every
// combat and DoT death before U5c.
func RouteAttributedDeath(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(events.CharacterDied)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "CharacterDied", "Actual Type", e.Type())
		return events.Cancel
	}

	char := resolveDyingCharacter(evt)
	if char == nil {
		// The victim went away between the blow and the flush. Nothing to do;
		// Die would be a no-op anyway.
		return events.Continue
	}

	// Something else already resolved this death. Clear the marker so the
	// character is not left permanently unkillable.
	if !char.IsAlive() {
		char.DeathQueued = false
		return events.Continue
	}

	if char.HasBuffFlag(buffs.ReviveOnDeath) {
		reviveInsteadOfDeath(char)
		char.DeathQueued = false
		return events.Continue
	}

	char.Die(state.ActorRef{
		UserId:        evt.KillerUserId,
		MobInstanceId: evt.KillerMobInstanceId,
	}, evt.Trigger)

	char.DeathQueued = false
	return events.Continue
}

// resolveDyingCharacter re-resolves the victim at flush time. It may be gone.
func resolveDyingCharacter(evt events.CharacterDied) *characters.Character {
	if evt.MobInstanceId != 0 {
		if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
			return &m.Character
		}
		return nil
	}
	if evt.UserId != 0 {
		if u := users.GetByUserId(evt.UserId); u != nil {
			return u.Character
		}
	}
	return nil
}

// reviveInsteadOfDeath mirrors the revive branch in mobcommands/suicide.go:
// full heal, announce, consume the buff. Health MUST come back above zero —
// skipping the death while leaving health negative just hands the kill to the
// sweep on the next tick.
func reviveInsteadOfDeath(char *characters.Character) {
	char.Health = char.HealthMax.Value

	if room := rooms.LoadRoom(char.RoomId); room != nil {
		room.SendTextVisual(messaging.CategoryBuffApply,
			`<ansi fg="mobname">`+char.Name+`</ansi> is suddenly revived in a shower of sparks!`,
		)
	}

	char.CancelBuffsWithFlag(buffs.ReviveOnDeath)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/hooks/ -run TestRouteAttributedDeath -v`
Expected: PASS, three tests.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/CharacterDied_RouteDeath.go internal/hooks/CharacterDied_RouteDeath_test.go
git commit -m "feat(hooks): resolve attributed deaths and centralise Die's prechecks (U5c)"
```

---

### Task 4: Register the listener and delete the redundant anonymous deaths

**Files:**
- Modify: `internal/hooks/hooks.go`
- Modify: `internal/hooks/NewRound_DoCombat.go:433`, `:447`
- Modify: `internal/hooks/NewRound_AutoHeal.go:58`
- Modify: `internal/hooks/Buff_ApplyBuffs.go:132`

- [ ] **Step 1: Register the listener**

In `internal/hooks/hooks.go`, inside `RegisterListeners()`, after the `events.Buff{}` registration:

```go
	// U5c: attributed death, queued by ApplyHarm at the harm site.
	events.RegisterListener(events.CharacterDied{}, RouteAttributedDeath)
```

- [ ] **Step 2: Delete the four redundant inline deaths**

Each of these now happens through the queued event, because every one of them is reached only after harm has already been applied through `ApplyHarm`. Delete the `Die` call and its surrounding `if` at:

- `internal/hooks/NewRound_DoCombat.go:433` (player)
- `internal/hooks/NewRound_DoCombat.go:447` (mob)
- `internal/hooks/NewRound_AutoHeal.go:58` (player DoT)
- `internal/hooks/Buff_ApplyBuffs.go:132` (buff DoT)

**Do NOT delete the two sweeps** (`NewRound_DoCombat.go:224` and `NewRound_MobRoundTick.go:125`). They are the backstop and Task 5 modifies them.

Before deleting each one, confirm the surrounding block does nothing else — several carry `continue` or message sends that must be preserved.

- [ ] **Step 3: Build and run the affected packages**

Run: `go build ./... && go test ./internal/hooks/ ./internal/characters/ ./internal/combat/`
Expected: `ok` for all three.

- [ ] **Step 4: Commit**

```bash
git add internal/hooks/hooks.go internal/hooks/NewRound_DoCombat.go internal/hooks/NewRound_AutoHeal.go internal/hooks/Buff_ApplyBuffs.go
git commit -m "refactor(hooks): route the four anonymous death sites through the queue (U5c)"
```

---

### Task 5: The sweeps become a real backstop

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat.go:219-227`
- Modify: `internal/hooks/NewRound_MobRoundTick.go:123-128`
- Test: `internal/hooks/sweep_backstop_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/sweep_backstop_test.go`:

```go
package hooks

import "testing"

// The distinction the whole design rests on. A character that is DYING but was
// never QUEUED reached 0 HP outside ApplyHarm — that is exactly what the sweep
// exists for. A sweep that skips on health instead of DeathQueued skips its own
// purpose and nothing ever dies on the non-harm paths.
func TestSweepReapsDyingButUnqueuedCharacter(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.DeathQueued = false

	if !shouldSweepReap(&mob.Character) {
		t.Fatal("sweep skipped a dying, unqueued character — the backstop is dead")
	}
}

// The sweep must not reap a victim whose attributed death is already in flight.
// Die is idempotent, so the attributed event would then be a silent no-op and
// the killer would be lost while everything still looked correct.
func TestSweepSkipsQueuedCharacter(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.DeathQueued = true

	if shouldSweepReap(&mob.Character) {
		t.Fatal("sweep pre-empted a queued attributed death; attribution is lost")
	}
}

func TestSweepIgnoresHealthyCharacter(t *testing.T) {
	mob := newRouteDeathTestMob(t, 50)

	if shouldSweepReap(&mob.Character) {
		t.Fatal("sweep reaped a living character")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestSweep -v`
Expected: FAIL to build — `shouldSweepReap` undefined.

- [ ] **Step 3: Implement the predicate and use it in both sweeps**

Add to `internal/hooks/CharacterDied_RouteDeath.go`:

```go
// shouldSweepReap reports whether the backstop sweep should kill this
// character.
//
// The condition is deliberately NOT "Health <= 0". A character reaped by the
// sweep is dying but NOT queued: it reached zero outside ApplyHarm. Skipping on
// health would skip the entire population the sweep exists for.
func shouldSweepReap(char *characters.Character) bool {
	if char == nil {
		return false
	}
	return char.Health <= 0 && char.IsAlive() && !char.DeathQueued
}
```

Then in `internal/hooks/NewRound_DoCombat.go`, replace the sweep condition:

```go
	for _, mobId := range mobs.GetAllMobInstanceIds() {
		if mob := mobs.GetInstance(mobId); mob != nil && shouldSweepReap(&mob.Character) {
			// Backstop only. Anything reaching here reached 0 HP without going
			// through ApplyHarm, so there is no killer to attribute. If this
			// fires often, find the path that bypasses the harm helper —
			// that is the point of the log line.
			mudlog.Debug("U5c sweep", "reason", "unattributed death",
				"mob", mob.Character.Name, "instanceId", mobId)
			mob.Character.Die(state.ActorRef{}, life.TriggerHealthZero)
		}
	}
```

Apply the identical change to the sweep in `internal/hooks/NewRound_MobRoundTick.go:123-128`, preserving its existing `continue`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/hooks/ -run TestSweep -v`
Expected: PASS, three tests.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/CharacterDied_RouteDeath.go internal/hooks/NewRound_DoCombat.go internal/hooks/NewRound_MobRoundTick.go internal/hooks/sweep_backstop_test.go
git commit -m "fix(hooks): sweeps skip queued deaths and log when they fire (U5c)"
```

---

### Task 6: Coup de grace

**Files:**
- Modify: `internal/items/itemspec.go:192` (new `Intensity`)
- Modify: `internal/items/attack_messages.go:135-146` (recursion guard)
- Modify: `_datafiles/world/dogmud/combat-messages/generic.yaml`
- Create: `internal/combat/coup_de_grace.go`
- Test: `internal/combat/coup_de_grace_test.go`
- Modify: `internal/combat/combat_helpers.go` (`buildAttackMessages`)

**This needs no new loader and no new data file.** `items.GetPreAttackMessage(subType, intensity)` already tries the weapon subtype first and falls back to `Generic` when that subtype has no entry — exactly the per-weapon-then-generic behaviour wanted. Adding a `coupdegrace` intensity therefore gets per-weapon override, skill tiers, together/separate and token substitution for free, and any weapon file may define its own block later without touching code.

- [ ] **Step 1: Add the intensity**

In `internal/items/itemspec.go`, after `Fumble` (line 192):

```go
	CoupDeGrace Intensity = "coupdegrace"
```

- [ ] **Step 2: Guard the fallback against infinite recursion**

`GetPreAttackMessage` ends with `return GetPreAttackMessage(Generic, messageType)`. If `Generic` itself lacks the key it calls itself forever and overflows the stack, taking the server down. That is survivable today only because `generic.yaml` happens to define every intensity in use; adding a new one widens the exposure, so guard it now.

In `internal/items/attack_messages.go`, replace the final line of `GetPreAttackMessage`:

```go
	// Fall back to generic, but never recurse into ourselves: a missing generic
	// entry would otherwise loop until the stack overflows and take the server
	// down. Returning the zero value degrades to no message instead.
	if subType == Generic {
		return AttackOptions{}
	}
	return GetPreAttackMessage(Generic, messageType)
```

- [ ] **Step 3: Write the failing test**

Create `internal/combat/coup_de_grace_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
)

func newDyingTestChar(health int) *characters.Character {
	c := characters.New()
	c.Name = "Target-Dummy"
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = health
	c.Buffs = buffs.New()
	return c
}

// "Dying" is health-based, not DeathQueued-based: it is what the renderer and
// combat targeting key off, and it is true the instant the lethal blow lands.
func TestIsDying(t *testing.T) {
	if !IsDying(newDyingTestChar(-5)) {
		t.Error("a live character below 1 health is dying")
	}
	if IsDying(newDyingTestChar(1)) {
		t.Error("a character at 1 health is not dying")
	}
	if IsDying(nil) {
		t.Error("nil is not dying")
	}
}

// Generic MUST carry a coupdegrace block: it is the fallback every weapon
// without its own block lands on, and an empty result renders nothing.
func TestCoupDeGraceGenericPoolExists(t *testing.T) {
	opts := items.GetPreAttackMessage(items.Generic, items.CoupDeGrace)

	if len(opts.Together.ToAttacker) == 0 {
		t.Fatal("generic.yaml has no coupdegrace toattacker messages; every weapon falls back to silence")
	}
}

// The recursion guard: asking Generic for an intensity it does not define must
// return empty rather than looping until the stack overflows.
func TestUnknownIntensityOnGenericDoesNotRecurse(t *testing.T) {
	opts := items.GetPreAttackMessage(items.Generic, items.Intensity("no-such-intensity"))

	if len(opts.Together.ToAttacker) != 0 {
		t.Error("expected an empty result for an undefined intensity")
	}
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `go test ./internal/combat/ -run 'TestIsDying|TestCoupDeGrace|TestUnknownIntensity' -v`
Expected: FAIL to build (`IsDying` undefined), then FAIL on the generic pool until Step 6 adds the YAML.

- [ ] **Step 5: Implement `IsDying`**

Create `internal/combat/coup_de_grace.go`:

```go
package combat

import "github.com/GoMudEngine/GoMud/internal/characters"

// IsDying reports whether a character has taken a lethal blow whose death has
// not resolved yet.
//
// Health-based on purpose. This is the state the renderer and combat targeting
// care about, and it is true the instant the blow lands. It is NOT the same as
// Character.DeathQueued, which tracks whether an event is in flight and is what
// the backstop sweeps skip on. Conflating the two breaks the sweeps.
func IsDying(char *characters.Character) bool {
	if char == nil {
		return false
	}
	return char.Health < 1 && char.IsAlive()
}
```

- [ ] **Step 6: Author the generic message block**

In `_datafiles/world/dogmud/combat-messages/generic.yaml`, add a `coupdegrace:` entry under `options:`, matching the structure the other intensities use. Skill tiers are `beginner`, `expert`, `master`; the available tokens are listed at the top of that file.

Read the neighbouring `fumble:` block first and mirror its exact indentation and key set. If it carries keys this block omits, add them rather than leaving them absent.

```yaml
  coupdegrace:
    together:
      toattacker:
        beginner:
        - 'You strike <ansi fg="{targettype}">{target}</ansi> again, past any need for it.'
        - 'You drive your <ansi fg="item">{itemname}</ansi> down into <ansi fg="{targettype}">{target}</ansi> once more.'
        expert:
        - '<ansi fg="{targettype}">{target}</ansi> is already falling. You strike anyway.'
        - 'You hammer <ansi fg="{targettype}">{target}</ansi> down with your <ansi fg="item">{itemname}</ansi>.'
        master:
        - 'You bear <ansi fg="{targettype}">{target}</ansi> to the ground under a final flurry.'
        - 'Your <ansi fg="item">{itemname}</ansi> finds <ansi fg="{targettype}">{target}</ansi> again, and again after that.'
      todefender:
        beginner:
        - '<ansi fg="{sourcetype}">{source}</ansi> strikes you as you fall.'
        expert:
        - '<ansi fg="{sourcetype}">{source}</ansi> strikes you as you fall.'
        master:
        - '<ansi fg="{sourcetype}">{source}</ansi> bears you down under a final flurry.'
      toroom:
        beginner:
        - '<ansi fg="{sourcetype}">{source}</ansi> keeps striking <ansi fg="{targettype}">{target}</ansi> as they fall.'
        expert:
        - '<ansi fg="{sourcetype}">{source}</ansi> keeps striking <ansi fg="{targettype}">{target}</ansi> as they fall.'
        master:
        - '<ansi fg="{sourcetype}">{source}</ansi> batters <ansi fg="{targettype}">{target}</ansi> to the ground.'
    separate:
      toattacker:
        beginner:
        - 'Your shot finds <ansi fg="{targettype}">{target}</ansi> as they fall.'
        expert:
        - 'Your shot finds <ansi fg="{targettype}">{target}</ansi> as they fall.'
        master:
        - 'Your shot punches into <ansi fg="{targettype}">{target}</ansi> as they go down.'
      todefender:
        beginner:
        - 'A shot from the {entrancename} finds you as you fall.'
        expert:
        - 'A shot from the {entrancename} finds you as you fall.'
        master:
        - 'A shot from the {entrancename} punches into you as you go down.'
```

- [ ] **Step 7: Wire it into attack rendering**

In `internal/combat/combat_helpers.go`, inside `buildAttackMessages`, extend the branch that already selects the fumble pool:

```go
	var msgs items.AttackOptions
	isFeint := false
	if result.Fumble {
		msgs = items.GetPreAttackMessage(displaySubtype, items.Fumble)
	} else if IsDying(targetChar) {
		// U5c: the target has already taken its lethal blow and the death is
		// queued. Later hits this round still connect and still count toward
		// the damage map, but they read as a coup de grace rather than ordinary
		// hit text, and never as a second kill announcement.
		//
		// After the fumble branch on purpose: flubbing a swing at a falling
		// target is still a fumble.
		msgs = items.GetPreAttackMessage(displaySubtype, items.CoupDeGrace)
	} else {
		msgs = items.GetAttackMessage(displaySubtype, int(pctDamage))
		// Feint check: skilled attackers can turn misses into deliberate-looking feints
		if int(pctDamage) == 0 && !result.Fumble {
			isFeint = checkFeint(sourceChar.GetCombatSkillLevel())
		}
```

Everything downstream (skill-tier selection, token substitution, room delivery) is unchanged.

- [ ] **Step 8: Run to verify it passes**

Run: `go test ./internal/combat/ -run 'TestIsDying|TestCoupDeGrace|TestUnknownIntensity' -v`
Expected: PASS, three tests.

Then: `go build ./... && go test ./internal/combat/ ./internal/items/`
Expected: `ok` for both.

- [ ] **Step 9: Commit**

```bash
git add internal/items/itemspec.go internal/items/attack_messages.go _datafiles/world/dogmud/combat-messages/generic.yaml internal/combat/coup_de_grace.go internal/combat/coup_de_grace_test.go internal/combat/combat_helpers.go
git commit -m "feat(combat): coup de grace text, per-weapon with a generic fallback (U5c)"
```

---

### Task 7: Regression and safety tests

**Files:**
- Test: `internal/hooks/attributed_death_regression_test.go`

- [ ] **Step 1: Write the regression tests**

Create `internal/hooks/attributed_death_regression_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// The whole point of the slice: the killer reaches DeadData, where the two live
// consumers read it. Death_PlayerCorpse gates its gold transfer on
// !d.Killer.IsZero(), and attributeBountyKill's guard-kill branch requires
// killer.IsMob() with a non-zero MobInstanceId. Before U5c every combat and DoT
// death passed an empty ref, so both silently did nothing.
func TestAttributedDeath_KillerReachesDeadData(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)

	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId:       mob.InstanceId,
		KillerMobInstanceId: 4242,
		Trigger:             life.TriggerHealthZero,
	})

	d, ok := mob.Character.Life.DeadData()
	if !ok {
		t.Fatal("victim never entered the Dead state")
	}
	if d.Killer.IsZero() {
		t.Fatal("killer ref is empty; the gold transfer and the guard-kill bounty branch both no-op")
	}
	if d.Killer.MobInstanceId != 4242 {
		t.Errorf("killer = %+v, want mob 4242", d.Killer)
	}
	if !d.Killer.IsMob() {
		t.Error("killer must satisfy attributeBountyKill's IsMob branch")
	}
}

// The lethal blow is credited, not the next swing. Before U5c the death fired
// from whichever attack ran the death check next, so the credit went to
// whoever swung after the kill rather than whoever landed it.
func TestAttributedDeath_LethalBlowIsCreditedNotTheNextSwing(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)

	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId:       mob.InstanceId,
		KillerMobInstanceId: 111, // landed the lethal blow
		Trigger:             life.TriggerHealthZero,
	})
	// A later swing's event for the same victim must not re-attribute.
	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId:       mob.InstanceId,
		KillerMobInstanceId: 222, // swung afterwards
		Trigger:             life.TriggerHealthZero,
	})

	d, _ := mob.Character.Life.DeadData()
	if d.Killer.MobInstanceId != 111 {
		t.Errorf("killer = %d, want 111 — the lethal blow, not the next swing",
			d.Killer.MobInstanceId)
	}
}
```

- [ ] **Step 2: Write the PvP safety net**

Append to the same file:

```go
// No player-sourced harm can reach another player: PVP is disabled, player
// HarmArea populates mob targets only (hooks/spell_resolution.go), and Throw
// iterates room.GetMobs() only. Asserted so a future change cannot quietly open
// that door — if this fails, re-audit every AoE target selector before shipping.
func TestPvpRemainsDisabled(t *testing.T) {
	if got := string(configs.GetGamePlayConfig().PVP); got != configs.PVPDisabled {
		t.Fatalf("PVP = %q, want %q", got, configs.PVPDisabled)
	}
}
```

- [ ] **Step 3: Run**

Run: `go test ./internal/hooks/ -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/hooks/attributed_death_regression_test.go
git commit -m "test(hooks): pin killer attribution and the PvP-stays-off invariant (U5c)"
```

---

### Task 8: Documentation

**Files:**
- Modify: `internal/characters/die.go:19-21`
- Modify: `internal/characters/context.md`, `internal/hooks/context.md`
- Modify: `docs/superpowers/specs/completed/2026-08-14-u5c-attributed-death-design.md`

- [ ] **Step 1: Fix `Die`'s doc comment**

In `internal/characters/die.go`, replace the caller-precondition block:

```go
// Prechecks live in hooks.RouteAttributedDeath, NOT at call sites. The previous
// comment here claimed ReviveOnDeath was "already handled at each call site" —
// it was not. Only the suicide commands checked it, so the buff was inert on
// every combat and DoT death. It also told callers to apply a Shadow Realm zone
// guard that does not exist anywhere in this repository; that line is removed
// rather than carried forward as a phantom requirement.
//
// Die remains idempotent: if the Life machine is already Dead or Respawning it
// returns immediately without firing observers.
```

- [ ] **Step 2: Update both `context.md` files**

`internal/characters/context.md` — document that `ApplyHarm` queues `CharacterDied` on lethal health harm, and that `DeathQueued` is not the same as dying.

`internal/hooks/context.md` — document `RouteAttributedDeath` as the single death-resolution path, the prechecks it owns, and that the sweeps skip on `DeathQueued` and never on health.

- [ ] **Step 3: Correct one line in the spec**

The spec says the listener is registered "alongside the existing `Death_*` family". Those are Life-machine observers, not event listeners. Change it to say the listener follows the `<Event>_<Action>.go` convention used by `Buff_ApplyBuffs.go`.

- [ ] **Step 4: Commit**

```bash
git add internal/characters/die.go internal/characters/context.md internal/hooks/context.md docs/superpowers/specs/completed/2026-08-14-u5c-attributed-death-design.md
git commit -m "docs: record the centralised death path and drop the phantom precondition (U5c)"
```

---

### Task 9: Patch notes

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Add the entry**

At the top of `docs/PATCH_NOTES.md`, below the title. No hard numbers, no em dashes, wrap at 80 characters:

```markdown
## 2026-08-14: Death lands when the blow lands

Something killed no longer lingers for a moment before it falls. The blow that
finishes a fight now ends it there and then, instead of waiting for the world to
come around and notice.

The game also knows who did it. Before, anything that died to poison, to a
thrown flask, or to the last exchange of a long fight died to nobody in
particular, and everything downstream of that had to guess. Bounties on a
wanted character are settled correctly now, and so is what the victor walks
away with.

If you keep swinging at something already falling, your blows still land, and
they read as what they are.
```

- [ ] **Step 2: Verify no dashes slipped in**

Run: `sed -n '1,20p' docs/PATCH_NOTES.md | grep -n "—\|–" || echo clean`
Expected: `clean`.

- [ ] **Step 3: Commit**

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs: patch notes for U5c"
```

---

### Task 10: Pre-push gates and PR

- [ ] **Step 1: gofmt**

Run: `gofmt -l internal/ modules/`
Expected: no output.

- [ ] **Step 2: Full build and test**

Run: `go build ./... && go test ./...`
Expected: `ok` everywhere. `internal/relationships` may fail to build with a Windows Defender quarantine message; that is a known local false positive, unrelated, and CI runs it fine.

- [ ] **Step 3: Boot test**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log  # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Exit code 124 is the success case. Set non-default ports in the copied config first so this cannot collide with a running server. Clean up with `git worktree remove --force`.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feature/u5c-attributed-death
gh pr create --repo pruuk/DOGMud --base master --head feature/u5c-attributed-death --fill
gh pr checks <n> --repo pruuk/DOGMud
```

- [ ] **Step 5: Do NOT merge on green alone**

U5c changes behaviour at named sites. Confirm with the owner before merging, and state plainly in the PR that the arc's deploy gate still holds: merging is not deploying, and prod stays where it is until the whole arc is done and playtested.

---

## Not in this slice

- **U6's three modelling gates.** Untouched.
- **`MobDeath.KillerMobInstanceId`**, still last-aggro-target rather than last-hit. A real killer ref could improve it; not required here.
- **Death degradation bias** (always skullduggery and dexterity on Meirok). Investigate after this lands; the obvious search terms do not locate the code, so start from the `Death_*` observers.
- **`GoldLossFraction`'s shipped value.** Absent from `config.yaml`, so the Go default applies. If it is 0 the gold-transfer fix is latent rather than visible. Worth checking during review.
