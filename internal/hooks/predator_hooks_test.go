package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpCallerActor is a minimal partyActor used only by TestPackFlee_*
// tests below to seed a non-leader caller into a party so we can verify
// the caller-death cleanup. It avoids importing internal/actions just
// for test scaffolding.
type helpCallerActor struct {
	mobInstanceId int
	name          string
}

func (h *helpCallerActor) IsPlayer() bool                      { return false }
func (h *helpCallerActor) GetUserId() int                      { return 0 }
func (h *helpCallerActor) GetMobInstanceId() int               { return h.mobInstanceId }
func (h *helpCallerActor) GetCharacter() *characters.Character { return nil }
func (h *helpCallerActor) GetRoom() *rooms.Room                { return nil }
func (h *helpCallerActor) GetName() string                     { return h.name }

// ─── Bleeding record tick ──────────────────────────────────────────────────

// The Bleeding record (122) ticks every round and STACKS (slice 1b, owner
// ruling 2026-09-14): each AddConditionMagnitude is its own stack with its own
// rounds, and one round tick lands the sum of the live stacks as ONE harm with
// ONE flavour line.
func TestRoundTick_BleedDamagesPlayer(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	require.NotNil(t, u)

	// The door validates on add, and Validate() recomputes HealthMax from
	// stats/balance config, clobbering a raw HealthMax.Value. Seeding Base
	// keeps HealthMax comfortably above the 40 this test needs.
	u.Character.HealthMax.Base = 100
	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 20, -5, "test")
	u.Character.Health = 40
	_ = drainPlain(1)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 35, u.Character.Health, "the bleed ticks on the first round tick")
	assert.Equal(t, 1, countContaining(drainPlain(1), "Blood seeps from your wounds!"),
		"the tick sends the bleed flavour line")

	// Cross-hook pin: AutoHeal's own bleed block is gone, so firing it after
	// the tick must not apply a second, redundant bleed hit. Health may move
	// by ordinary out-of-combat regen, but not by another 5-point bleed. The
	// record still has 19 rounds left here (Bleeding flag still held), so a
	// revived flag-gated block in AutoHeal is reachable and would be caught.
	AutoHeal(events.NewRound{RoundNumber: 3})
	assert.GreaterOrEqual(t, u.Character.Health, 35, "AutoHeal must not re-apply the bleed")
	assert.Less(t, u.Character.Health, 40, "AutoHeal's own regen should be small next to the 5-point bleed it must not repeat")

	u.Character.RemoveCondition(conditions.ConditionIdBleeding)
	u.Character.Health = 50
}

func TestRoundTick_BleedDamagesMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)

	mob.Character.HealthMax.Base = 100
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 1, -50, "test")
	mob.Character.Health = 2

	// tickMobConditions runs in MobRoundTick's idle lane, before the active-zone
	// check, so it fires for every mob regardless of zone activity.
	MobRoundTick(events.NewRound{RoundNumber: 1})

	assert.Less(t, mob.Character.Health, 0,
		"a 50-magnitude bleed on a 2-health mob should store overkill, not clamp to 0; health=%d", mob.Character.Health)
	assert.Less(t, mob.Character.Health, 1,
		"overkilled health must still satisfy the `< 1` death gate; health=%d", mob.Character.Health)

	mob.Character.RemoveCondition(conditions.ConditionIdBleeding)
	mob.Character.Health = 50
}

func TestRoundTick_BleedMinDamageOne(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)

	mob.Character.HealthMax.Base = 100
	// Magnitude -0.5 truncates to 0, so the stack's amount is floored to -1.
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 1, -0.5, "test")
	mob.Character.Health = 50

	MobRoundTick(events.NewRound{RoundNumber: 1})

	assert.Equal(t, 49, mob.Character.Health)

	mob.Character.RemoveCondition(conditions.ConditionIdBleeding)
	mob.Character.Health = 50
}

// A one-round stack's only tick is also its last. The player tick used to gate
// its whole body on !condition.Expired(), so that tick applied nothing and said
// nothing. This pins the expiring tick: the harm lands AND the flavour line
// goes out, exactly once.
//
// Null probe: restoring `!condition.Expired() &&` to the text gate in
// NewRound_UserRoundTick.go turns the line assertion red.
func TestRoundTick_BleedLineLandsOnExpiringTick(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	require.NotNil(t, u)

	// Discard anything an earlier test left queued, so the count below is
	// only what this round produced.
	_ = drainPlain(1)

	u.Character.HealthMax.Base = 100
	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 1, -5, "test")
	require.Equal(t, 1, u.Character.Conditions.GetConditions(conditions.ConditionIdBleeding)[0].TriggersLeft,
		"a one-round stack: the tick under test is its last")
	// Health is set AFTER the add, because the door validates on add.
	u.Character.Health = 80

	UserRoundTick(events.NewRound{RoundNumber: 1})

	assert.Equal(t, 75, u.Character.Health,
		"the stack's only tick is also its last, and it must still apply its harm")
	assert.Equal(t, 1, countContaining(drainPlain(1), "Blood seeps from your wounds!"),
		"the expiring tick must still send the bleed flavour line, exactly once")

	u.Character.RemoveCondition(conditions.ConditionIdBleeding)
	u.Character.Health = 50
}

// Two stacks sum into ONE harm and ONE line per round, and each stack ends on
// its own: 3 rounds of -2 and 5 rounds of -3 land -5, -5, -5, -3, -3, then
// nothing.
func TestRoundTick_BleedStacksSumIntoOneHarmPerRound_Player(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	u.Character.HealthMax.Base = 100
	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 3, -2, "test")
	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 5, -3, "test")
	require.Len(t, u.Character.Conditions.GetConditions(conditions.ConditionIdBleeding), 1, "two stacks, one record")
	u.Character.Health = 80
	_ = drainPlain(1)

	for round, want := range []int{75, 70, 65, 62, 59, 59} {
		UserRoundTick(events.NewRound{RoundNumber: uint64(round + 1)})
		assert.Equal(t, want, u.Character.Health, "round %d", round+1)
		lines := countContaining(drainPlain(1), "Blood seeps from your wounds!")
		if round < 5 {
			assert.Equal(t, 1, lines, "round %d: one line however many stacks are live", round+1)
		} else {
			assert.Equal(t, 0, lines, "round %d: the bleed has ended", round+1)
		}
	}

	u.Character.RemoveCondition(conditions.ConditionIdBleeding)
	u.Character.Health = 50
}

func TestRoundTick_BleedStacksSumIntoOneHarmPerRound_Mob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.HealthMax.Base = 100
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 3, -2, "test")
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 5, -3, "test")
	mob.Character.Health = 80

	for round, want := range []int{75, 70, 65, 62, 59, 59} {
		MobRoundTick(events.NewRound{RoundNumber: uint64(round + 1)})
		assert.Equal(t, want, mob.Character.Health, "round %d", round+1)
	}

	mob.Character.RemoveCondition(conditions.ConditionIdBleeding)
	mob.Character.Health = 50
}

// ─── PackFlee ───────────────────────────────────────────────────────────────

func TestPackFlee_WrongEventType(t *testing.T) {
	result := PackFlee(events.NewRound{RoundNumber: 1})
	assert.Equal(t, events.Continue, result)
}

func TestPackFlee_NoGroups(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	// MobId 2 (Merchant) has no Groups in the spec
	evt := events.MobDeath{
		MobId:         2,
		InstanceId:    999,
		RoomId:        1,
		CharacterName: "Merchant",
	}

	result := PackFlee(evt)
	assert.Equal(t, events.Continue, result)
}

func TestPackFlee_NoMobSpec(t *testing.T) {
	evt := events.MobDeath{
		MobId:         9999, // nonexistent
		InstanceId:    999,
		RoomId:        1,
		CharacterName: "Ghost",
	}

	result := PackFlee(evt)
	assert.Equal(t, events.Continue, result)
}

func TestPackFlee_InvalidRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	evt := events.MobDeath{
		MobId:         1,
		InstanceId:    999,
		RoomId:        9999, // nonexistent room
		CharacterName: "Skeleton",
	}

	result := PackFlee(evt)
	assert.Equal(t, events.Continue, result)
}

func TestPackFlee_TriggersForGroupmates(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	// mob instance 100 is in room 1 with Groups: ["undead"]
	// Mob spec 1 also has Groups: ["undead"]
	// When mob spec 1 dies, instance 100 (same group) should get flee queued

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)

	// Verify mob is in the room
	mobIds := room.GetMobs(rooms.FindAll)
	assert.Contains(t, mobIds, 100)

	evt := events.MobDeath{
		MobId:         1, // spec with Groups: ["undead"]
		InstanceId:    999,
		RoomId:        1,
		CharacterName: "Skeleton",
	}

	// PackFlee should queue flee commands on groupmates
	// It won't execute them (Command queues for later), but it should not panic
	result := PackFlee(evt)
	assert.Equal(t, events.Continue, result)
}

// TestPackFlee_ClearsHelpRoomOnCallerDeath verifies that when a non-leader
// party member who raised a help call dies, the death handler clears the
// party's HelpRoomId and HelpCallerInstanceId so in-flight responders stop
// trekking to the now-empty rally room.
//
// Models the bandit pack scenario: lookout (non-leader) calls help, dies
// to the player, and the camp mobs walking up to the rally point should
// not infinitely oscillate after the fight is over.
func TestPackFlee_ClearsHelpRoomOnCallerDeath(t *testing.T) {
	// p.Dissolve below runs in t.Cleanup, after seedAllRegistries' drain.
	drainEventQueueOnCleanup(t)
	cleanup := seedAllRegistries()
	defer cleanup()

	// Build a party: leader (instance 8000) + caller (instance 8001).
	leader := &helpCallerActor{mobInstanceId: 8000, name: "Soren"}
	caller := &helpCallerActor{mobInstanceId: 8001, name: "Lookout"}

	p := parties.NewByActor(leader)
	require.NotNil(t, p, "NewByActor failed for leader")
	t.Cleanup(func() { p.Dissolve("test-cleanup") })

	require.True(t, p.AddActor(caller), "AddActor failed for caller")

	// Simulate party_call_help having fired from the caller.
	p.HelpRoomId = 4043
	p.HelpCallerInstanceId = 8001

	// The caller dies. The leader is still alive, so the party should NOT
	// dissolve — but the help call MUST clear because the rallying point
	// no longer has a fight to come to.
	evt := events.MobDeath{
		MobId:         283, // bandit lookout template
		InstanceId:    8001,
		RoomId:        4043,
		CharacterName: "bandit lookout",
	}
	result := PackFlee(evt)
	assert.Equal(t, events.Continue, result)

	// Party still exists (leader didn't die).
	stillThere := parties.GetByMobInstanceId(8000)
	require.NotNil(t, stillThere, "leader should still be in their party")
	assert.Equal(t, p, stillThere)

	// Help call cleared.
	assert.Equal(t, 0, p.HelpRoomId, "HelpRoomId should be cleared after caller death")
	assert.Equal(t, 0, p.HelpCallerInstanceId, "HelpCallerInstanceId should be cleared after caller death")
}

// TestPackFlee_PreservesHelpRoomOnNonCallerDeath verifies that when a
// non-caller member dies, the help call stays active so other responders
// keep coming. Only the caller's death clears the call.
func TestPackFlee_PreservesHelpRoomOnNonCallerDeath(t *testing.T) {
	// p.Dissolve below runs in t.Cleanup, after seedAllRegistries' drain.
	drainEventQueueOnCleanup(t)
	cleanup := seedAllRegistries()
	defer cleanup()

	leader := &helpCallerActor{mobInstanceId: 8010, name: "Soren"}
	caller := &helpCallerActor{mobInstanceId: 8011, name: "Lookout"}
	other := &helpCallerActor{mobInstanceId: 8012, name: "Fighter"}

	p := parties.NewByActor(leader)
	require.NotNil(t, p)
	t.Cleanup(func() { p.Dissolve("test-cleanup") })
	require.True(t, p.AddActor(caller))
	require.True(t, p.AddActor(other))

	p.HelpRoomId = 4043
	p.HelpCallerInstanceId = 8011 // lookout is the caller

	// Fighter (the non-caller) dies.
	evt := events.MobDeath{
		MobId:         284, // bandit fighter template
		InstanceId:    8012,
		RoomId:        4043,
		CharacterName: "bandit fighter",
	}
	PackFlee(evt)

	// Help call should be unchanged — caller is still alive.
	assert.Equal(t, 4043, p.HelpRoomId, "HelpRoomId should persist when a non-caller dies")
	assert.Equal(t, 8011, p.HelpCallerInstanceId, "HelpCallerInstanceId should persist when a non-caller dies")
}

func TestPackFlee_SkipsNonGroupmates(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	// Mob spec 2 (Merchant) has no Groups in the spec.
	// If a merchant dies, no mobs should flee because there are no groupmates.
	// mob instance 100 (undead skeleton) is in room 1 but doesn't share groups
	// with the merchant.
	mob100 := mobs.GetInstance(100)
	require.NotNil(t, mob100)

	room := rooms.LoadRoom(1)
	require.NotNil(t, room)

	evt := events.MobDeath{
		MobId:         2, // merchant spec — no groups
		InstanceId:    999,
		RoomId:        1,
		CharacterName: "Merchant",
	}

	// This should not crash and should skip everyone (merchant has no groups)
	result := PackFlee(evt)
	assert.Equal(t, events.Continue, result)
}

// Owner ruling (2026-09-28, open question 1): when a packmate dies, only
// packmates already fighting flee; idle ones stay put and are not counted in
// the scatter line.
func TestPackFlee_IdlePackmateStaysPut(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, restore := packFleeFixture(t)
	defer restore()
	require.False(t, mob.Character.IsInCombat(), "fixture: mob 100 starts idle")

	PackFlee(events.MobDeath{MobId: 1, InstanceId: 999, RoomId: 1, CharacterName: "Skeleton"})

	require.False(t, packFleeQueued(mob), "an idle packmate was told to flee")
}

// The positive twin: without it the idle test above would pass on a PackFlee
// that queues nothing at all.
func TestPackFlee_FightingPackmateFlees(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, restore := packFleeFixture(t)
	defer restore()
	fleeingMob(t)

	PackFlee(events.MobDeath{MobId: 1, InstanceId: 999, RoomId: 1, CharacterName: "Skeleton"})

	require.True(t, packFleeQueued(mob), "a fighting packmate was not told to flee")
}

// A fighting packmate that cannot begin a flee (here, knocked down) is not
// told to flee and is not counted toward the scatter line: counting it printed
// a scatter nobody performed.
func TestPackFlee_FightingPackmateThatCannotFleeStaysPut(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, restore := packFleeFixture(t)
	defer restore()
	fleeingMob(t)
	setCombatPositionParallel(&mob.Character, position.Prone)
	require.True(t, mob.Character.IsInCombat(), "fixture: the packmate is fighting")

	PackFlee(events.MobDeath{MobId: 1, InstanceId: 999, RoomId: 1, CharacterName: "Skeleton"})

	require.False(t, packFleeQueued(mob), "a knocked-down packmate was told to flee")
}

// packFleeFixture reseeds the mob registry so spec 1 and instance 100 carry a
// species. The shared seedAllRegistries fixture leaves SpeciesId 0, and
// PackFlee returns before looking at any packmate when the dead mob's spec has
// no species, so a PackFlee test on the bare fixture cannot fail. GetMobSpec
// returns a copy, hence the reseed. The caller defers the returned cleanup,
// which must run before seedAllRegistries' own.
func packFleeFixture(t *testing.T) (*mobs.Mob, func()) {
	t.Helper()
	spec := mobs.GetMobSpec(1)
	require.NotNil(t, spec)
	spec.Character.SpeciesId = 1
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.SpeciesId = 1
	restore := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: spec, 2: mobs.GetMobSpec(2)},
		map[int]*mobs.Mob{100: mob},
	)
	events.DrainQueuedInputsForTest(mob.InstanceId)
	return mob, restore
}

func packFleeQueued(mob *mobs.Mob) bool {
	for _, cmd := range events.DrainQueuedInputsForTest(mob.InstanceId) {
		if cmd == "flee" {
			return true
		}
	}
	return false
}
