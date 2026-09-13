package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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

// The Bleeding record's shipped triggerrate is 3 rounds (matching the old
// AutoHeal hook, which only ever applied its DoT on every third round while
// the old condition enum decremented duration every round): buffs.TickTriggers
// converts an old rounds-literal duration into the equivalent trigger count,
// and RoundCounter must reach a multiple of 3 (rounds 1, 2, 3 of ticking)
// before the record fires even once.
//
// The record is seeded with buffs.TickTriggers(20) (6 triggers), not
// TickTriggers(3) (1 trigger): the cross-hook pin below needs the Bleeding
// flag to still be held, not expired, when AutoHeal runs after round 4, or a
// revived flag-gated AutoHeal bleed block would have nothing to gate on and
// the pin would pass for the wrong reason.
func TestRoundTick_BleedDamagesPlayer(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	require.NotNil(t, u)

	// The door validates on add, and Validate() recomputes HealthMax from
	// stats/balance config, clobbering a raw HealthMax.Value. Seeding Base
	// keeps HealthMax comfortably above the 40 this test needs.
	u.Character.HealthMax.Base = 100
	_ = u.Character.AddBuffMagnitude(buffs.BuffIdBleeding, buffs.TickTriggers(20), -5, "test")
	u.Character.Health = 40

	// Rounds 1 and 2: RoundCounter isn't yet a multiple of 3, no trigger.
	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 40, u.Character.Health, "no trigger yet on round 1")
	UserRoundTick(events.NewRound{RoundNumber: 2})
	assert.Equal(t, 40, u.Character.Health, "no trigger yet on round 2")

	// Round 3: RoundCounter reaches 3, the first of six triggers lands.
	UserRoundTick(events.NewRound{RoundNumber: 3})
	assert.Equal(t, 35, u.Character.Health, "the first trigger should land on round 3")
	// Whole-branch review (slice 1): the trigger text lands whenever the
	// tick fires, including a record's final trigger; pin the bleed case
	// alongside the poison one so the flavour line has coverage here too.
	assert.Equal(t, 1, countContaining(drainPlain(1), "Blood seeps from your wounds!"),
		"the third-round trigger sends the bleed flavour line")

	// RoundCounter is 4, not a multiple of 3; a fourth round tick must not
	// re-trigger it yet.
	UserRoundTick(events.NewRound{RoundNumber: 4})
	assert.Equal(t, 35, u.Character.Health, "the interval hasn't come back around; a further tick must not re-fire it")

	// Cross-hook pin: AutoHeal's own bleed block is gone, so firing it after
	// the tick must not apply a second, redundant bleed hit. Health may move
	// by ordinary out-of-combat regen, but not by another 5-point bleed. The
	// record still has five triggers left here (Bleeding flag still held),
	// so a revived flag-gated block in AutoHeal is reachable and would be
	// caught.
	AutoHeal(events.NewRound{RoundNumber: 3})
	assert.GreaterOrEqual(t, u.Character.Health, 35, "AutoHeal must not re-apply the bleed")
	assert.Less(t, u.Character.Health, 40, "AutoHeal's own regen should be small next to the 5-point bleed it must not repeat")

	u.Character.RemoveBuff(buffs.BuffIdBleeding)
	u.Character.Health = 50
}

func TestRoundTick_BleedDamagesMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)

	mob.Character.HealthMax.Base = 100
	_ = mob.Character.AddBuffMagnitude(buffs.BuffIdBleeding, buffs.TickTriggers(3), -50, "test")
	mob.Character.Health = 2

	// tickMobBuffs runs in MobRoundTick's idle lane, before the active-zone
	// check, so it fires for every mob regardless of zone activity. Three
	// ticks are needed before the record's one trigger lands.
	MobRoundTick(events.NewRound{RoundNumber: 1})
	MobRoundTick(events.NewRound{RoundNumber: 2})
	MobRoundTick(events.NewRound{RoundNumber: 3})

	assert.Less(t, mob.Character.Health, 0,
		"a 50-magnitude bleed on a 2-health mob should store overkill, not clamp to 0; health=%d", mob.Character.Health)
	assert.Less(t, mob.Character.Health, 1,
		"overkilled health must still satisfy the `< 1` death gate; health=%d", mob.Character.Health)

	mob.Character.RemoveBuff(buffs.BuffIdBleeding)
	mob.Character.Health = 50
}

func TestRoundTick_BleedMinDamageOne(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)

	mob.Character.HealthMax.Base = 100
	// Magnitude -0.5 truncates to 0, so the snapshot is floored to -1 in sign.
	_ = mob.Character.AddBuffMagnitude(buffs.BuffIdBleeding, buffs.TickTriggers(3), -0.5, "test")
	mob.Character.Health = 50

	MobRoundTick(events.NewRound{RoundNumber: 1})
	MobRoundTick(events.NewRound{RoundNumber: 2})
	MobRoundTick(events.NewRound{RoundNumber: 3})

	assert.Equal(t, 49, mob.Character.Health)

	mob.Character.RemoveBuff(buffs.BuffIdBleeding)
	mob.Character.Health = 50
}

// The ordinary bleed is a ONE-TRIGGER record: buffs.TickTriggers(3) yields 1.
// The player tick used to gate its whole body, harm and text alike, on
// !buff.Expired(), so that single trigger -- which is also the record's final
// one -- applied nothing and said nothing, and every ordinary bleed a player
// took was silent and harmless. This pins the expiring trigger: the harm lands
// AND the flavour line goes out, exactly once.
//
// Null probe: restoring `!buff.Expired() &&` to the text gate in
// NewRound_UserRoundTick.go turns the line assertion red.
func TestRoundTick_BleedLineLandsOnExpiringTick(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	require.NotNil(t, u)

	// Discard anything an earlier test left queued, so the count below is
	// only what these three rounds produced.
	_ = drainPlain(1)

	// Validate() recomputes HealthMax from stats and balance config, so seed
	// Base rather than Value.
	u.Character.HealthMax.Base = 100
	_ = u.Character.AddBuffMagnitude(buffs.BuffIdBleeding, buffs.TickTriggers(3), -5, "test")
	require.Equal(t, 1, u.Character.Buffs.GetBuffs(buffs.BuffIdBleeding)[0].TriggersLeft,
		"an ordinary bleed is a single-trigger record; the trigger under test is its last")
	// Health is set AFTER the add, because the door validates on add.
	u.Character.Health = 80

	// triggerrate is 3 rounds, so RoundCounter must reach 3 before it fires.
	UserRoundTick(events.NewRound{RoundNumber: 1})
	UserRoundTick(events.NewRound{RoundNumber: 2})
	UserRoundTick(events.NewRound{RoundNumber: 3})

	assert.Equal(t, 75, u.Character.Health,
		"the record's only trigger is also its last, and it must still apply its harm")
	assert.Equal(t, 1, countContaining(drainPlain(1), "Blood seeps from your wounds!"),
		"the expiring trigger must still send the bleed flavour line, exactly once")

	u.Character.RemoveBuff(buffs.BuffIdBleeding)
	u.Character.Health = 50
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
