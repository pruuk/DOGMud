# Follow-up slice F: mobs perceive darkness, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A mob stops picking targets it cannot see, and the dark-dwelling species get back the night vision their data has always claimed.

**Architecture:** No new concepts. Mob sight reuses `messaging.CanSeeSightImpairedOnly`, exactly what combat already calls; creature perception reuses `characters.Character.Perceives`, the rule slice A introduced. Five hostile room scans gain a sight gate, two single hooks gain a viewer, buff 29 is restored, and a boot guard makes a species referencing a missing buff id fail loudly.

**Tech Stack:** Go, `internal/behaviortree`, `internal/mobcommands`, `internal/actions`, `internal/species`, `internal/buffs`, YAML world data.

Spec: `docs/superpowers/specs/2026-09-11-followup-slice-f-mobs-perceive-darkness-design.md`

---

## Sequencing constraint, read this first

**Task 1 must land before Task 2.** The boot guard added in Task 2 panics on a
species referencing a buff that does not exist. Buff 29 is exactly such a
reference today, so wiring the guard first would panic the server on the very
data it exists to catch. Restore the buff, then add the guard.

## File structure

| File | Change |
|---|---|
| `_datafiles/world/dogmud/buffs/29-night_vision.yaml` | Create, copied from `world/default` |
| `internal/buffs/buffspec.go` | Add `HasSpec(int) bool` |
| `internal/species/species.go` | Add `ValidateSpeciesBuffIds(buffIdExists func(int) bool)` |
| `main.go` | Wire the new guard after line 1693 |
| `internal/behaviortree/sight.go` | Create: the one mob-sight helper |
| `internal/behaviortree/conditions_player.go` | Gate `condPlayersInRoom`, `condMultipleEnemies` |
| `internal/behaviortree/actions_party.go` | Gate `engageHostilePlayerInRoom` |
| `internal/behaviortree/actions_archer.go` | Gate `archerMeleeEngaged` |
| `internal/behaviortree/conditions_scout.go` | Route `condRoomHasHiddenEntity` through `Perceives` |
| `internal/mobcommands/attack.go` | Pass `&mob.Character` instead of `nil` |
| `internal/actions/cast_admission.go` | Stop exempting mobs |
| `lookup_viewer_guard_test.go` | Move the mob caller from exception to filtered |

---

### Task 1: Restore buff 29

**Files:**
- Create: `_datafiles/world/dogmud/buffs/29-night_vision.yaml`
- Test: `internal/species/species_night_vision_test.go`

- [ ] **Step 1: Write the test that pins the species list**

```go
package species

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The eight species that declare buff 29 have been silently blind since dogmud
// forked: the buff exists only in world/default. 67 mobs are affected.
func TestNightVisionSpeciesDeclareBuff29(t *testing.T) {
	want := []int{2, 4, 5, 8, 9, 11, 17, 24}
	for _, id := range want {
		sp := GetSpecies(id)
		require.NotNil(t, sp, "species %d missing", id)
		require.Contains(t, sp.BuffIds, 29, "species %d (%s) should declare night vision", id, sp.Name)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/species/ -run TestNightVisionSpeciesDeclareBuff29 -v`
Expected: PASS. The species data is already correct; it is the BUFF that is missing. This test exists so a later data edit cannot silently shrink the list.

- [ ] **Step 3: Copy the buff file**

```bash
cp _datafiles/world/default/buffs/29-night_vision.yaml _datafiles/world/dogmud/buffs/29-night_vision.yaml
```

- [ ] **Step 4: Verify the flag the predicate reads**

```bash
grep -A 2 '^flags:' _datafiles/world/dogmud/buffs/29-night_vision.yaml
grep -n 'NightVision .*Flag' internal/buffs/buffspec.go
```

Expected: the file lists `- nightvision`, and `buffspec.go:62` declares ``NightVision Flag = `nightvision` ``. They must match exactly. The Cat's Eye Draught shipped inert because its flag was misspelled.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/buffs/29-night_vision.yaml internal/species/species_night_vision_test.go
git commit -m "fix(data): restore Night Vision, the buff 8 species already claimed"
```

---

### Task 2: The boot guard

**Files:**
- Modify: `internal/buffs/buffspec.go`, `internal/species/species.go`, `main.go`
- Test: `internal/species/validate_buffids_test.go`

- [ ] **Step 1: Write the failing test**

```go
package species

import "testing"

func TestValidateSpeciesBuffIdsPanicsOnMissingBuff(t *testing.T) {
	orig := allSpecies
	t.Cleanup(func() { allSpecies = orig })
	allSpecies = map[int]*Species{
		99: {SpeciesId: 99, Name: "Fixture", BuffIds: []int{123456}},
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a species referencing a buff that does not exist")
		}
	}()
	ValidateSpeciesBuffIds(func(int) bool { return false })
}

func TestValidateSpeciesBuffIdsAcceptsKnownBuffs(t *testing.T) {
	orig := allSpecies
	t.Cleanup(func() { allSpecies = orig })
	allSpecies = map[int]*Species{
		99: {SpeciesId: 99, Name: "Fixture", BuffIds: []int{29}},
	}
	ValidateSpeciesBuffIds(func(id int) bool { return id == 29 })
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/species/ -run TestValidateSpeciesBuffIds -v`
Expected: FAIL, `undefined: ValidateSpeciesBuffIds`

- [ ] **Step 3: Add the buffs checker**

In `internal/buffs/buffspec.go`, beside `GetBuffSpec`:

```go
// HasSpec reports whether a buff id is defined. Mirrors mutations.HasSpec so
// cross-package validators can take it as an injected checker.
func HasSpec(buffId int) bool {
	return GetBuffSpec(buffId) != nil
}
```

- [ ] **Step 4: Add the validator, mirroring the existing precedent**

In `internal/species/species.go`, directly below `ValidateBodyPartTags`:

```go
// ValidateSpeciesBuffIds scans all loaded species and panics on any buff id
// that does not exist. Called from main after species + buffs are loaded.
//
// buffIdExists is a callback for cross-package lookup — pass buffs.HasSpec.
//
// This exists because buff 29 (Night Vision) was referenced by eight species
// and absent from dogmud entirely, so 67 mobs were silently blind in their own
// caves. Nothing failed, because nothing checked.
func ValidateSpeciesBuffIds(buffIdExists func(id int) bool) {
	for _, sp := range allSpecies {
		for _, id := range sp.BuffIds {
			if !buffIdExists(id) {
				panic(fmt.Sprintf(
					"species %q (id %d): unknown buff id in buffids: %d",
					sp.Name, sp.SpeciesId, id))
			}
		}
	}
}
```

- [ ] **Step 5: Wire it at boot**

In `main.go`, immediately after the existing `species.ValidateBodyPartTags(mutations.HasSpec)` (line 1693):

```go
	species.ValidateSpeciesBuffIds(buffs.HasSpec)
```

Buffs load at `main.go:1635` and species at `:1640`, so both are populated here.

- [ ] **Step 6: Run the tests**

```bash
go test ./internal/species/ ./internal/buffs/
go build ./...
```

Expected: PASS, clean build.

- [ ] **Step 7: Prove the guard catches the real thing**

```bash
mv _datafiles/world/dogmud/buffs/29-night_vision.yaml /tmp/29.bak
go run . 2>&1 | head -20     # expect a panic naming a species and buff 29
cp /tmp/29.bak _datafiles/world/dogmud/buffs/29-night_vision.yaml
```

**Restore with a copy, never `git checkout --`.** That discarded uncommitted work earlier in this arc.

- [ ] **Step 8: Commit**

```bash
git add internal/buffs/buffspec.go internal/species/species.go internal/species/validate_buffids_test.go main.go
git commit -m "feat(species): fail the boot on a species referencing a buff that does not exist"
```

---

### Task 3: The one mob-sight helper

**Files:**
- Create: `internal/behaviortree/sight.go`, `internal/behaviortree/sight_test.go`

`internal/behaviortree` already imports `internal/messaging` (five files), and
`messaging` does not import `behaviortree`, so this needs no injected predicate.

- [ ] **Step 1: Write the failing test and the shared fixture**

This `sightScene` helper is used by Tasks 3, 4, 5 and 6. It seeds BOTH buffs:
29 so a mob can see in the dark, and 1 so a bystander can light the room.

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

const (
	sightNightVisionBuffId = 29
	sightIlluminationBuffId = 1
)

// sightScene stands one mob (instance 8101) in room 8100 of the given biome.
// SpeciesId is 1 deliberately: species.GetSpecies() returns nil otherwise and
// several mob paths dereference it.
func sightScene(t *testing.T, biome string) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", DarkArea: true},
		"city": {BiomeId: "city", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sightNightVisionBuffId:  {BuffId: sightNightVisionBuffId, Name: "Night Vision", Flags: []buffs.Flag{buffs.NightVision}},
		sightIlluminationBuffId: {BuffId: sightIlluminationBuffId, Name: "Illumination", Flags: []buffs.Flag{buffs.EmitsLight}},
	}))

	room := &rooms.Room{RoomId: 8100, Biome: biome}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8100: room}))

	m := &mobs.Mob{
		MobId:      8000,
		InstanceId: 8101,
		HomeRoomId: 8100,
		Character: characters.Character{
			Name:      "Watcher",
			RoomId:    8100,
			Health:    100,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
			SpeciesId: 1,
		},
	}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(8101, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(8101, nil) })
	return m, room
}

func TestMobCanSeeDarkRoom(t *testing.T) {
	m, room := sightScene(t, "cave")
	require.False(t, mobCanSee(m, room), "an unlit cave blinds a mob with no night vision")

	require.NoError(t, m.Character.AddBuff(sightNightVisionBuffId, true))
	require.True(t, mobCanSee(m, room), "night vision restores sight in the dark")
}

func TestMobCanSeeLitRoom(t *testing.T) {
	m, room := sightScene(t, "city")
	require.True(t, mobCanSee(m, room))
}

// These run on every tree tick; a missing instance must not blind the world.
func TestMobCanSeeDefaultsOpen(t *testing.T) {
	require.True(t, mobCanSee(nil, nil))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/behaviortree/ -run TestMobCanSee -v`
Expected: FAIL, `undefined: mobCanSee`

- [ ] **Step 3: Implement**

```go
package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// mobCanSee reports whether a mob can make out the room it is standing in.
//
// It is the SAME predicate combat already uses (combat/combat.go builds
// combatContext from it), so a mob's decisions and its combat penalty cannot
// disagree. Restoring buff 29 flows through here automatically, because
// CanSeeSightImpairedOnly ends by reading the NightVision flag, and a
// light-bearing player or mob lifts the darkness for everyone via
// Room.GetVisibility.
//
// A nil mob or room returns true. These run on every behaviour tree tick and a
// missing instance must not silently blind the world.
func mobCanSee(mob *mobs.Mob, room *rooms.Room) bool {
	if mob == nil || room == nil {
		return true
	}
	return messaging.CanSeeSightImpairedOnly(&mob.Character, room)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/behaviortree/ -run TestMobCanSee -v`
Expected: PASS

- [ ] **Step 5: Sabotage probe**

Replace the body with `return true`. Confirm `TestMobCanSeeDarkRoom` goes red on its FIRST assertion, naming the unlit cave. Restore with a file copy.

Do NOT probe by swapping `CanSeeSightImpairedOnly` for `CanSeeClearly`: they differ only on sleep, so the test would stay green and prove nothing.

- [ ] **Step 6: Commit**

```bash
git add internal/behaviortree/sight.go internal/behaviortree/sight_test.go
git commit -m "feat(behaviortree): one mob-sight helper, shared with combat"
```

---

### Task 4: Gate the two player-scan conditions

**Files:**
- Modify: `internal/behaviortree/conditions_player.go` (`condPlayersInRoom` at :115, `condMultipleEnemies` at :192)
- Test: `internal/behaviortree/conditions_sight_test.go`

- [ ] **Step 1: Write the failing test**

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestCondPlayersInRoomNeedsSight(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8110, "kesh", "Kesh", 98110)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8110: u}))
	room.AddPlayer(8110)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condPlayersInRoom(nil, ctx), "a blind mob finds nobody")

	require.NoError(t, m.Character.AddBuff(sightNightVisionBuffId, true))
	require.Equal(t, Success, condPlayersInRoom(nil, ctx), "night vision finds them")
}

// The case that keeps the Ironwind cave bosses working: a light carried by
// anyone lifts the darkness for everyone, so a mob with no night vision of its
// own still sees. If this test ever fails, the gate is reading the biome
// instead of Room.GetVisibility.
func TestCondPlayersInRoomSeesWhenSomeoneCarriesLight(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8111, "lume", "Lume", 98111)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8111: u}))
	room.AddPlayer(8111)
	require.NoError(t, u.Character.AddBuff(sightIlluminationBuffId, true))

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Success, condPlayersInRoom(nil, ctx))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/behaviortree/ -run TestCondPlayersInRoom -v`
Expected: FAIL on the first assertion. Today a blind mob still finds players.

- [ ] **Step 3: Gate `condPlayersInRoom`**

```go
func condPlayersInRoom(params map[string]any, ctx *EvalContext) Result {
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}
	// Slice F: a mob that cannot see the room finds no one in it. All three
	// trees using this condition are hostile (ambusher archetype, bandit
	// leader, chrysalis phantom), so this is target acquisition.
	if !mobCanSee(mobs.GetInstance(ctx.InstanceId), room) {
		return Failure
	}
	if len(room.GetPlayers()) > 0 {
		return Success
	}
	return Failure
}
```

- [ ] **Step 4: Gate `condMultipleEnemies`**

Immediately after its existing `mob := mobs.GetInstance(ctx.InstanceId)` and the
`charmedByUserId` block, before `count := 0`:

```go
	if !mobCanSee(mob, room) {
		return Failure
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/behaviortree/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/behaviortree/conditions_player.go internal/behaviortree/conditions_sight_test.go
git commit -m "feat(behaviortree): a blind mob finds no players to fight"
```

---

### Task 5: Gate the party and archer engage paths

**Files:**
- Modify: `internal/behaviortree/actions_party.go` (`engageHostilePlayerInRoom`), `internal/behaviortree/actions_archer.go` (`archerMeleeEngaged`)
- Test: extend `internal/behaviortree/conditions_sight_test.go`

The two take their mob differently: `engageHostilePlayerInRoom(mobInstanceId, roomId int)` looks it up, `archerMeleeEngaged(mob *mobs.Mob)` receives it.

- [ ] **Step 1: Write the failing test**

```go
func TestEngageHostilePlayerInRoomNeedsSight(t *testing.T) {
	m, room := sightScene(t, "cave")
	m.AutoAggro = true
	u := users.NewTestUser(8120, "kesh", "Kesh", 98120)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8120: u}))
	room.AddPlayer(8120)

	require.False(t, engageHostilePlayerInRoom(m.InstanceId, room.RoomId), "a blind mob does not pick up aggro")
	require.False(t, m.Character.IsInCombat(), "and starts no fight")

	require.NoError(t, m.Character.AddBuff(sightNightVisionBuffId, true))
	require.True(t, engageHostilePlayerInRoom(m.InstanceId, room.RoomId), "night vision engages normally")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/behaviortree/ -run TestEngageHostilePlayerInRoom -v`
Expected: FAIL. The blind mob engages today.

- [ ] **Step 3: Gate `engageHostilePlayerInRoom`**

After the existing `room := rooms.LoadRoom(roomId)` nil check. It sits BELOW the
`mob.Character.IsInCombat()` early return, so a fight already under way is
untouched (owner ruling 4).

```go
	if !mobCanSee(mob, room) {
		return false
	}
```

- [ ] **Step 4: Gate `archerMeleeEngaged`**

After its `room := rooms.LoadRoom(myRoom)` nil check. Leave check (1), the
mob's own existing aggro target, ungated: that is an existing fight, not a new
target.

```go
	if !mobCanSee(mob, room) {
		return false
	}
```

- [ ] **Step 5: Run the tests and commit**

```bash
go test ./internal/behaviortree/
git add internal/behaviortree/actions_party.go internal/behaviortree/actions_archer.go internal/behaviortree/conditions_sight_test.go
git commit -m "feat(behaviortree): blind mobs do not start fights"
```

---

### Task 6: Scout detection through Perceives

**Files:**
- Modify: `internal/behaviortree/conditions_scout.go` (`condRoomHasHiddenEntity` at :21)
- Test: extend `internal/behaviortree/conditions_sight_test.go`

- [ ] **Step 1: Write the failing test**

```go
func hideForTest(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "slice_f_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
}

// A lit room: this is about hiding, not darkness.
func TestCondRoomHasHiddenEntityUsesPerceives(t *testing.T) {
	m, room := sightScene(t, "city")
	u := users.NewTestUser(8130, "kesh", "Kesh", 98130)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8130: u}))
	room.AddPlayer(8130)
	hideForTest(t, u.Character)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condRoomHasHiddenEntity(nil, ctx),
		"a scout with no see-hidden must not sense a hider")
}
```

Imports to add: `internal/state`, `internal/state/awareness`, `internal/characters`.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/behaviortree/ -run TestCondRoomHasHiddenEntity -v`
Expected: FAIL. The raw `IsHidden()` scan succeeds today.

- [ ] **Step 3: Implement**

Replace both loops. A hider counts only when the scout perceives it.

```go
	self := mobs.GetInstance(ctx.InstanceId)
	for _, pId := range room.GetPlayers() {
		p := users.GetByUserId(pId)
		if p == nil || !p.Character.IsHidden() {
			continue
		}
		if self == nil || self.Character.Perceives(p.Character) {
			return Success
		}
	}
	for _, mId := range room.GetMobs() {
		other := mobs.GetInstance(mId)
		if other == nil || other.InstanceId == ctx.InstanceId || !other.Character.IsHidden() {
			continue
		}
		if self == nil || self.Character.Perceives(&other.Character) {
			return Success
		}
	}
	return Failure
```

- [ ] **Step 4: Run the tests and commit**

```bash
go test ./internal/behaviortree/
git add internal/behaviortree/conditions_scout.go internal/behaviortree/conditions_sight_test.go
git commit -m "feat(behaviortree): a scout only senses hiders it perceives"
```

---

### Task 7: The mob attack hook

**Files:**
- Modify: `internal/mobcommands/attack.go:42`
- Test: `internal/mobcommands/attack_sight_test.go`
- Modify: `lookup_viewer_guard_test.go`

- [ ] **Step 1: Write the failing test**

```go
package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A mob cannot attack by name what it cannot see. Slice F.
func TestMobAttackTargetNeedsSight(t *testing.T) {
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", DarkArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		29: {BuffId: 29, Name: "Night Vision", Flags: []buffs.Flag{buffs.NightVision}},
	}))

	room := &rooms.Room{RoomId: 8200, Biome: "cave"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8200: room}))

	m := &mobs.Mob{
		MobId: 8200, InstanceId: 8201, HomeRoomId: 8200,
		Character: characters.Character{
			Name: "Lurker", RoomId: 8200, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(8201, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(8201, nil) })

	u := users.NewTestUser(8210, "kesh", "Kesh", 98210)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8210: u}))
	room.AddPlayer(8210)

	blind := actions.FindAttackTarget("kesh", room, 0, m.InstanceId, &m.Character)
	require.False(t, blind.Found, "a blind mob must not resolve a named target")

	require.NoError(t, m.Character.AddBuff(29, true))
	seen := actions.FindAttackTarget("kesh", room, 0, m.InstanceId, &m.Character)
	require.True(t, seen.Found, "night vision resolves it")
	require.Equal(t, 8210, seen.UserId)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/mobcommands/ -run TestMobAttackTargetNeedsSight -v`
Expected: the first assertion FAILS until the viewer is wired, because
`FindAttackTarget` is currently called with `nil`.

- [ ] **Step 3: Pass the viewer**

In `internal/mobcommands/attack.go:42`:

```go
		t := actions.FindAttackTarget(rest, room, 0, mob.InstanceId, &mob.Character)
```

- [ ] **Step 4: Update the lookup registry**

In `lookup_viewer_guard_test.go`, change the entry

```go
	"internal/mobcommands/attack.go|Attack": {plain: 1, why: whyMob},
```

to

```go
	"internal/mobcommands/attack.go|Attack": {viewer: 1},
```

The root guard then enforces it, and slice F removes one of the `whyMob`
exemptions that slice A deliberately left behind.

- [ ] **Step 5: Run the gates and commit**

```bash
go test ./internal/mobcommands/ ./internal/actions/ .
git add internal/mobcommands/attack.go internal/mobcommands/attack_sight_test.go lookup_viewer_guard_test.go
git commit -m "feat(mobcommands): a mob cannot attack by name what it cannot see"
```

---

### Task 8: The mob cast hook

**Files:**
- Modify: `internal/actions/cast_admission.go:47`
- Modify: `internal/actions/cast_sight_test.go` (invert one existing test)

🪤 **Trap:** `mobcommands/cast.go:42` builds `&actions.MobActor{Mob: mob, Room: room}`
with the room populated directly. `NewMobActor` returns a nil Room, and
`admitCastAim` returns early on a nil room, so switching that line to
`NewMobActor(mob)` would silently disable this entire gate. Leave it as is.

- [ ] **Step 1: Invert the existing test**

`cast_sight_test.go` currently asserts today's behaviour:

```go
func TestCastSight_MobCasterIsUnaffected(t *testing.T) {
	a, room := castSightScene(t, "cave")
	mobCaster := newStubActor(a.GetCharacter(), room) // IsPlayer false
	r := InitiateCast(mobCaster, "sight-heal", "witness")
	require.True(t, r.Initiated, "mobs perceiving darkness is slice F")
}
```

Replace it with:

```go
func TestCastSight_MobCasterNeedsSightToo(t *testing.T) {
	a, room := castSightScene(t, "cave")
	mobCaster := newStubActor(a.GetCharacter(), room) // IsPlayer false
	r := InitiateCast(mobCaster, "sight-heal", "witness")
	require.False(t, r.Initiated, "slice F: a mob's targeted cast needs sight")
	require.True(t, r.NoTarget)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/actions/ -run TestCastSight_MobCaster -v`
Expected: FAIL. Mobs are exempt today.

- [ ] **Step 3: Implement**

In `cast_admission.go`, replace the player-only early return:

```go
	room := actor.GetRoom()
	if room == nil {
		return aim, false
	}
```

Mobs now fall through to the same sight rules. The shapes branch is unreachable
for them, because no dogmud mob has buff 85, and `castsAtSelf` still exempts a
self-cast, which works for mobs because `MobActor.GetName` returns the mob's
character name.

- [ ] **Step 4: Run the full actions suite and commit**

```bash
go test ./internal/actions/
git add internal/actions/cast_admission.go internal/actions/cast_sight_test.go
git commit -m "feat(cast): a mob's targeted cast needs sight too"
```

---

### Task 9: Gates, playtest, PR

- [ ] **Step 1: Full gates**

```bash
gofmt -l internal/ modules/ lookup_viewer_guard_test.go   # want nothing
go build ./...
go test ./...
golangci-lint run --new-from-rev=master
```

- [ ] **Step 2: Isolated boot check**

Per the `dogmud-shipping` skill: detached worktree, copy `config.yaml` in by
hand, build to a fixed `boot-check.exe`, `timeout 180`, expect exit 124, zero
panics, one `Server Ready`. The new boot guard runs here, so this also proves
buff 29 loads.

- [ ] **Step 3: Write the playtest goals file**

Create `tools/playtest/goals/2026-09-11-slice-f-cave-bosses.yaml`:

```yaml
# Slice F: confirm the Ironwind cave bosses still attack. Neither the Stone
# Beetle Queen (species 12) nor the Windscour Wyrm (species 23) has night
# vision, so they fight only because a carried light lifts the darkness.
ephemeral:
  profile: veteran
  start_room: 3112
  budgets:
    wall_clock: 20m

goals:
  - >-
    Type `conditions` and confirm Illumination is NOT active yet. Then type
    `cast chrysalis-glow` and confirm it takes effect. Type `look` and quote
    the room description.
  - >-
    You are in the Beetle Queen's Chamber with the glow up. Wait up to 6 rounds
    without attacking. Quote anything the Stone Beetle Queen does. SHE MUST
    ATTACK YOU. If she does not, say so at once: that is the defect this run
    exists to find.
  - >-
    Flee or walk to the Wyrm's Den (room 3114) with the glow still up. Wait up
    to 6 rounds. The Windscour Wyrm MUST attack. Quote what it does.
  - >-
    Let the glow lapse (do not recast). Leave and re-enter the den. Quote what
    you see and whether the wyrm engages. Then type `attack wyrm` and confirm a
    fight starts anyway. Record both outcomes verbatim.
```

- [ ] **Step 4: Run it**

```text
/playtest local --checkout <absolute repo path> bug-finder 2026-09-11-slice-f-cave-bosses.yaml
```

Confirm teardown with `docker ps` and remove only `dogmud-playtest-<run_id>-server-1`
by exact name. Playtest reports are gitignored, so extract findings to memory
before the tree moves on.

- [ ] **Step 5: PR**

```bash
git push -u origin feature/followup-slice-f-mobs-perceive-darkness
gh pr create --repo pruuk/DOGMud --base master --head feature/followup-slice-f-mobs-perceive-darkness --title "Follow-up slice F: mobs perceive darkness" --body-file <path written with the Write tool>
gh pr checks <n> --repo pruuk/DOGMud --watch
```

Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. Merge only on the
owner's word. The owner runs the deploy.
