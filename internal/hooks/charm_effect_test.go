package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

func charmTestSpellData() *spells.SpellData {
	return &spells.SpellData{
		SpellId:           "charm",
		Name:              "Charm",
		Type:              spells.HarmSingle,
		EffectType:        "charm",
		PrimaryStat:       "charisma",
		TargetDefenseType: "social",
		Schools:           []string{"manifestation"},
	}
}

func charmTestMob(t *testing.T, instanceId, roomId int) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{
		MobId:      1,
		InstanceId: instanceId,
		HomeRoomId: roomId,
		Character: characters.Character{
			Name:      "Bandit Scout",
			RoomId:    roomId,
			Health:    30,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 30
	return m
}

// applyMobEffect is reached with a nil user when a MOB casts -- its own
// docstring says so, and resolveMobSpellAgainstMob does it. No mob carries
// charm today because the behaviour tree skips it, but a switch arm whose
// safety depends on an exclusion in another package is a landmine, and every
// sibling arm guards.
func TestApplyMobEffectCharm_NilUserDoesNotPanic(t *testing.T) {
	const roomId = 8801
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	mob := charmTestMob(t, 8811, roomId)
	mobs.SetInstanceForTest(mob.InstanceId, mob)
	defer mobs.SetInstanceForTest(mob.InstanceId, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("applyMobEffect_charm panicked on a nil user: %v", r)
		}
	}()

	if got := applyMobEffect_charm(nil, mob, room, charmTestSpellData(),
		combat.ChannelDefenceResult{}, "Bandit Scout"); got != 0 {
		t.Errorf("applyMobEffect_charm returned %d, want 0 (charm deals no damage)", got)
	}
	if mob.Character.IsCharmed() {
		t.Error("a nil-user charm must not bind the creature to anyone")
	}
}

// A nil mob must be as safe as a nil user; applyMobEffect's other arms are
// reached only with a live mob, but this arm is now a public-ish seam.
func TestApplyMobEffectCharm_NilMobDoesNotPanic(t *testing.T) {
	const roomId = 8802
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("applyMobEffect_charm panicked on a nil mob: %v", r)
		}
	}()
	_ = applyMobEffect_charm(nil, nil, room, charmTestSpellData(),
		combat.ChannelDefenceResult{}, "Nobody")
}

// A charm-immune creature is refused before anything else happens, and in
// particular before the reservation gate, so the refusal a player sees names
// the real reason.
func TestApplyMobEffectCharm_ImmuneCreatureIsRefused(t *testing.T) {
	const roomId = 8803
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	mob := charmTestMob(t, 8813, roomId)
	mob.CharmImmune = true
	mobs.SetInstanceForTest(mob.InstanceId, mob)
	defer mobs.SetInstanceForTest(mob.InstanceId, nil)

	_ = applyMobEffect_charm(nil, mob, room, charmTestSpellData(),
		combat.ChannelDefenceResult{}, "Bandit Scout")

	if mob.Character.IsCharmed() {
		t.Error("a charm-immune creature must never be bound")
	}
}
