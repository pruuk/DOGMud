package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Proc dispatch (item behaviour slice 3, Rule 21): which worn item each
// proc event reaches, the ItemProcsEnabled gate, and the kill (ruling S4).

const (
	dispatchProbeWeapon  = 999920
	dispatchProbeOffhand = 999921
	dispatchProbeBody    = 999922
)

// dispatchProbeTree procs on every proc event; the kill branch has a chance,
// so a draw can be seen.
const dispatchProbeTree = `
tree:
  type: selector
  children:
    - type: action
      event: on_hit
      do: proc
      effect: lifesteal
      ratio: 0.5
    - type: action
      event: on_block
      do: proc
      effect: lifesteal
      ratio: 0.5
    - type: action
      event: on_grapple
      do: proc
      effect: apply_condition
      condition: 1
      duration: 6
      magnitude: 12
    - type: action
      event: on_spell_hit
      do: proc
      effect: steal_pool
      pool: 3
      amount_pct: 0.10
    - type: decorator
      event: on_kill
      mod: random
      percent: 50
      child:
        type: action
        do: proc
        effect: aoe_stun
`

// seedDispatchProbe seeds the registries (users 1 and 2 and hostile mob 100
// in room 1), three probe items sharing the probe tree, procs on.
func seedDispatchProbe(t *testing.T) (alice, bob *characters.Character) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	setItemProcsEnabled(t, true)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		dispatchProbeWeapon:  {ItemId: dispatchProbeWeapon, Name: "probe blade", Type: items.Weapon, Hands: 1, Behavior: "dispatch_probe"},
		dispatchProbeOffhand: {ItemId: dispatchProbeOffhand, Name: "probe shield", Type: items.Offhand, Behavior: "dispatch_probe"},
		dispatchProbeBody:    {ItemId: dispatchProbeBody, Name: "probe harness", Type: items.Body, Behavior: "dispatch_probe"},
	}))
	behaviortree.LoadItemTreeForTest(t, "dispatch_probe", dispatchProbeTree)
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	alice, bob = users.GetByUserId(1).Character, users.GetByUserId(2).Character
	for _, c := range []*characters.Character{alice, bob} {
		c.HealthMax.Value = 200
		c.Health = 100
		c.ConvictionMax.Value = 100
		c.Conviction = 50
	}
	return alice, bob
}

func TestFireItemProc_HitAndBlockReachTheirSlot(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	alice.Equipment.Weapon = items.New(dispatchProbeWeapon)

	// A block reaches the offhand, which is empty: nothing heals.
	fireItemProc(behaviortree.EventContext{EventType: "on_block"}, alice, bob, nil, 80)
	if alice.Health != 100 {
		t.Fatalf("a block reached the weapon: health %d", alice.Health)
	}
	fireItemProc(behaviortree.EventContext{EventType: "on_hit"}, alice, bob, nil, 80)
	if alice.Health != 140 {
		t.Fatalf("on_hit lifesteal: health %d, want 140", alice.Health)
	}
	alice.Equipment.Offhand = items.New(dispatchProbeOffhand)
	fireItemProc(behaviortree.EventContext{EventType: "on_block"}, alice, bob, nil, 20)
	if alice.Health != 150 {
		t.Fatalf("on_block lifesteal from the offhand: health %d, want 150", alice.Health)
	}
}

// A grapple fires for both sides; each reaches its own body armour and
// wounds the other.
// A mob's weapon procs through the same entry: the holder resolves from the
// mob instance id, not a user id.
func TestFireItemProc_MobWeaponProcs(t *testing.T) {
	_, bob := seedDispatchProbe(t)
	const instId = 5150
	mob := &mobs.Mob{MobId: 2, InstanceId: instId}
	mob.Character.Name = "Brute"
	mob.Character.Conditions = conditions.New()
	mob.Character.MobInstanceId = instId
	mob.Character.HealthMax.Value = 200
	mob.Character.Health = 100
	mob.Character.Equipment.Weapon = items.New(dispatchProbeWeapon)
	mobs.SetInstanceForTest(instId, mob)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	fireItemProc(behaviortree.EventContext{EventType: "on_hit"}, &mob.Character, bob, nil, 80)
	if mob.Character.Health != 140 {
		t.Fatalf("a mob's on_hit lifesteal: health %d, want 140", mob.Character.Health)
	}
}

func TestFireItemProc_GrappleReachesTheBody(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	alice.Equipment.Body = items.New(dispatchProbeBody)

	fireItemProc(behaviortree.EventContext{EventType: "on_grapple"}, alice, bob, nil, 0)
	fireItemProc(behaviortree.EventContext{EventType: "on_grapple"}, bob, alice, nil, 0)
	if !bob.HasCondition(conditions.ConditionIdBleeding) {
		t.Error("the harness wearer's opponent should be bleeding")
	}
	if alice.HasCondition(conditions.ConditionIdBleeding) {
		t.Error("the opponent wears no harness; the wearer should not bleed")
	}
}

func TestFireItemProc_SpellHitReachesTheWeapon(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	alice.Equipment.Weapon = items.New(dispatchProbeWeapon)
	fireItemProc(behaviortree.EventContext{EventType: "on_spell_hit"}, alice, bob, nil, 25)
	if bob.Conviction != 40 || alice.Conviction != 60 {
		t.Fatalf("steal_pool: target %d caster %d, want 40 and 60", bob.Conviction, alice.Conviction)
	}
}

// ItemProcsEnabled off: no proc fires and no number is drawn, on the
// dispatcher's events and on a kill.
func TestItemProcsOffStopsEveryProcAndDrawsNothing(t *testing.T) {
	alice, bob := seedDispatchProbe(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	alice.Equipment.Weapon = items.New(dispatchProbeWeapon)
	alice.Equipment.Offhand = items.New(dispatchProbeOffhand)
	alice.Equipment.Body = items.New(dispatchProbeBody)
	setItemProcsEnabled(t, false)

	restore := util.SetRandForTest(42)
	want := util.Rand(1000)
	restore()
	restore = util.SetRandForTest(42)
	defer restore()
	for _, ev := range []string{"on_hit", "on_block", "on_grapple", "on_spell_hit"} {
		fireItemProc(behaviortree.EventContext{EventType: ev}, alice, bob, nil, 80)
	}
	MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{1: 1}})
	if got := util.Rand(1000); got != want {
		t.Errorf("a disabled proc drew a number (probe %d, want %d)", got, want)
	}
	if alice.Health != 100 || bob.Conviction != 50 || bob.HasCondition(conditions.ConditionIdBleeding) {
		t.Errorf("a disabled proc fired: health %d, target conviction %d", alice.Health, bob.Conviction)
	}
}

// Ruling S4: a kill reaches an on_kill proc on any worn item, here the
// offhand, not the weapon alone.
func TestKillReachesAnOnKillProcOnTheOffhand(t *testing.T) {
	alice, _ := seedDispatchProbe(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		84: {ConditionId: 84, Name: "Stunned", RoundInterval: 1, TriggerCount: 1},
	}))
	alice.SetUserId(1)
	alice.Equipment.Offhand = items.New(dispatchProbeOffhand)
	hostile := mobs.GetInstance(100)
	if hostile == nil {
		t.Fatal("expected seeded hostile mob instance 100")
	}
	restore := util.SetRandForTest(1)
	defer restore()
	for i := 0; i < 20 && !hostile.Character.HasCondition(84); i++ {
		MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{1: 1}})
	}
	if !hostile.Character.HasCondition(84) {
		t.Error("twenty kills never fired the offhand's on_kill proc")
	}
}

// The on_kill hook stamps the hunger-anchor round for every player with
// damage attribution.
func TestMobDeathItemProcs_RecordsLastKill(t *testing.T) {
	seedDispatchProbe(t)
	u := users.GetByUserId(1)
	if ret := MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{1: 50}}); ret != events.Continue {
		t.Fatalf("expected events.Continue, got %v", ret)
	}
	got, ok := characters.MiscRound(u.Character.GetMiscData("pinnacle_last_kill_round"))
	if !ok || got != util.GetRoundCount() {
		t.Fatalf("expected last-kill round %d, got %v (ok=%v)", util.GetRoundCount(), got, ok)
	}
}
