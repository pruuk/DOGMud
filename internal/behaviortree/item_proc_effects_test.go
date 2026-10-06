package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The four proc effects (item behaviour slice 3, Rule 20), moved unchanged
// from internal/hooks/item_procs.go with their tests.

const (
	effectRoom      = 90621
	effectEmptyRoom = 90622
)

// seedStunWorld registers condition 84 (the 1-round stagger-Stun) and two
// rooms, effectRoom and an empty effectEmptyRoom.
func seedStunWorld(t *testing.T) *rooms.Room {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		84: {
			ConditionId:   84,
			Name:          "Stunned",
			Description:   "Reeling — no meaningful attack or defense this round.",
			RoundInterval: 1,
			TriggerCount:  1,
		},
	}))
	room := &rooms.Room{RoomId: effectRoom}
	empty := &rooms.Room{RoomId: effectEmptyRoom}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{effectRoom: room, effectEmptyRoom: empty}, map[string]*rooms.ZoneConfig{}))
	return room
}

// addEffectMob registers a mob instance in room for aoe_stun targeting.
func addEffectMob(t *testing.T, room *rooms.Room, instanceId int, nonCombatant bool) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{
		InstanceId: instanceId,
		HomeRoomId: room.RoomId,
		Character: characters.Character{
			Name:         "Test Beast",
			RoomId:       room.RoomId,
			NonCombatant: nonCombatant,
			Conditions:   conditions.New(),
			Cooldowns:    map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 50
	m.Character.Health = 50
	mobs.SetInstanceForTest(instanceId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instanceId, nil) })
	room.AddMob(instanceId)
	return m
}

// effectOwner is a player character (user id 1) in room.
func effectOwner(room int) *characters.Character {
	c := characters.New()
	c.SetUserId(1)
	c.RoomId = room
	return c
}

func TestProcAoeStun_StunsHostilesSkipsProtected(t *testing.T) {
	room := seedStunWorld(t)
	hostileA := addEffectMob(t, room, 90631, false)
	hostileB := addEffectMob(t, room, 90632, false)
	nonCombatant := addEffectMob(t, room, 90633, true)

	if ok := procAoeStun(effectOwner(effectRoom), room, map[string]float64{}); !ok {
		t.Fatal("aoe_stun should execute (true) with hostile mobs present")
	}
	if !hostileA.Character.HasCondition(84) || !hostileB.Character.HasCondition(84) {
		t.Error("both hostile mobs should be stunned (condition 84)")
	}
	if nonCombatant.Character.HasCondition(84) {
		t.Error("the non-combatant must NOT be stunned")
	}
}

// Charmed mobs are spared whoever their master is: the owner's AND a
// bystander's (player-cast HarmArea parity).
func TestProcAoeStun_SkipsCharmedCompanions(t *testing.T) {
	room := seedStunWorld(t)
	hostile := addEffectMob(t, room, 90631, false)
	ownerCompanion := addEffectMob(t, room, 90632, false)
	ownerCompanion.Character.Charm(1, characters.CharmPermanent, "")
	bystanderCompanion := addEffectMob(t, room, 90633, false)
	bystanderCompanion.Character.Charm(2, characters.CharmPermanent, "")

	if ok := procAoeStun(effectOwner(effectRoom), room, map[string]float64{}); !ok {
		t.Fatal("aoe_stun should execute with a hostile present")
	}
	if !hostile.Character.HasCondition(84) {
		t.Error("the hostile mob should be stunned")
	}
	if ownerCompanion.Character.HasCondition(84) || bystanderCompanion.Character.HasCondition(84) {
		t.Error("charmed companions must NOT be stunned")
	}
}

// An empty room does not execute, so the branch's cooldown is not armed.
func TestProcAoeStun_EmptyRoomReturnsFalse(t *testing.T) {
	seedStunWorld(t)
	if procAoeStun(effectOwner(effectEmptyRoom), nil, map[string]float64{}) {
		t.Fatal("aoe_stun in an empty room should return false")
	}
}

// A mob owner (GetUserId() == 0) stuns nothing and returns false.
func TestProcAoeStun_MobOwnerIsNoOp(t *testing.T) {
	room := seedStunWorld(t)
	hostile := addEffectMob(t, room, 90631, false)
	if procAoeStun(characters.New(), room, map[string]float64{}) {
		t.Fatal("aoe_stun from a mob owner should return false")
	}
	if hostile.Character.HasCondition(84) {
		t.Error("mob-owner aoe_stun must not stun anything")
	}
}

func TestProcLifesteal(t *testing.T) {
	attacker := characters.New()
	attacker.HealthMax.Value = 200
	attacker.Health = 100
	if healed := procLifesteal(attacker, 80, map[string]float64{"ratio": 0.25}); healed != 20 {
		t.Fatalf("expected 20 healed (25%% of 80), got %d", healed)
	}
	if attacker.Health != 120 {
		t.Fatalf("expected health 120, got %d", attacker.Health)
	}
	attacker.Health = 195
	procLifesteal(attacker, 80, map[string]float64{"ratio": 0.25})
	if attacker.Health != 200 {
		t.Fatalf("expected clamp at 200, got %d", attacker.Health)
	}
}

func TestProcApplyCondition_Bleed(t *testing.T) {
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	target := characters.New()
	if !procApplyCondition(target, map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("apply_condition should execute")
	}
	if got := target.Conditions.TriggersLeft(conditions.ConditionIdBleeding); got != 6 {
		t.Fatalf("expected 6: duration is the stack's rounds and the record ticks every round, got %d", got)
	}
	held := target.GetConditions(conditions.ConditionIdBleeding)
	if len(held) != 1 || held[0].Magnitude != -12 {
		t.Fatalf("expected one Bleeding record at magnitude -12, got %+v", held)
	}
	if len(held[0].Stacks) != 1 || held[0].Stacks[0].RoundsLeft != 6 || held[0].Stacks[0].Amount != -12 {
		t.Fatalf("expected one stack of 6 rounds at -12, got %+v", held[0].Stacks)
	}
}

// With the Bleeding spec absent the add fails, and procApplyCondition must
// report it rather than claim success, or the branch's cooldown would be
// armed for a bleed that never landed. Null probe: making case 1 return
// true unconditionally turns this red.
func TestProcApplyCondition_BleedSpecMissing_ReturnsFalse(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	if procApplyCondition(characters.New(), map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("procApplyCondition must return false when the Bleeding spec is missing")
	}
}

func TestProcApplyCondition_NilAndUnknown(t *testing.T) {
	if procApplyCondition(nil, map[string]float64{"condition": 1}) {
		t.Fatal("nil target must not execute")
	}
	if procApplyCondition(characters.New(), map[string]float64{"condition": 99}) {
		t.Fatal("unknown condition id must not execute")
	}
}

func TestProcStealPool_Conviction(t *testing.T) {
	caster := characters.New()
	caster.ConvictionMax.Value = 100
	caster.Conviction = 40
	target := characters.New()
	target.ConvictionMax.Value = 100
	target.Conviction = 50
	if !procStealPool(caster, target, map[string]float64{"pool": 3, "amount_pct": 0.10}) {
		t.Fatal("steal_pool should execute")
	}
	if target.Conviction != 40 || caster.Conviction != 50 {
		t.Fatalf("10%% of the target's max moves over: target=%d caster=%d, want 40 and 50", target.Conviction, caster.Conviction)
	}
	// clamps: target at 0, caster at max
	target.Conviction = 3
	caster.Conviction = 95
	procStealPool(caster, target, map[string]float64{"pool": 3, "amount_pct": 0.10})
	if target.Conviction != 0 || caster.Conviction != 98 {
		t.Fatalf("clamps wrong: target=%d caster=%d", target.Conviction, caster.Conviction)
	}
}

func TestProcStealPool_Guards(t *testing.T) {
	caster := characters.New()
	target := characters.New()
	if procStealPool(nil, target, map[string]float64{"pool": 3, "amount_pct": 0.1}) {
		t.Fatal("nil owner must not execute")
	}
	if procStealPool(caster, nil, map[string]float64{"pool": 3, "amount_pct": 0.1}) {
		t.Fatal("nil target must not execute")
	}
	if procStealPool(caster, target, map[string]float64{"pool": 3}) {
		t.Fatal("missing amount_pct must not execute")
	}
	target.Conviction = 0
	target.ConvictionMax.Value = 100
	if procStealPool(caster, target, map[string]float64{"pool": 3, "amount_pct": 0.1}) {
		t.Fatal("empty target pool must not execute")
	}
	if procStealPool(caster, target, map[string]float64{"pool": 2, "amount_pct": 0.1}) {
		t.Fatal("unimplemented pool ids must not execute")
	}
}
