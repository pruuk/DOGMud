package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
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

// charmedRounds drives the charm arm with a given contest result and reports
// the clock it set. The mob is fresh each call so the bond cannot carry over.
func charmedRounds(t *testing.T, roomId, instId int, out combat.ChannelDefenceResult) int {
	t.Helper()
	room, cleanupRoom := seedHookRoom(t, roomId)
	defer cleanupRoom()

	mob := charmTestMob(t, instId, roomId)
	mobs.SetInstanceForTest(mob.InstanceId, mob)
	defer mobs.SetInstanceForTest(mob.InstanceId, nil)

	user := users.NewTestUser(1, "charmer", "Charmer", 9101)
	user.Character.RoomId = roomId
	// A real caster's pool. The flat CompanionReserveDefault of 280 is gated by
	// PoolReservationCapPct against ConvictionMax = 5 + Cha*3 + Wil, so a
	// default fixture is REFUSED and every duration assertion would read 0.
	user.Character.Stats.Charisma.Base = 150
	user.Character.Stats.Charisma.Recalculate()
	user.Character.Stats.Willpower.Base = 150
	user.Character.Stats.Willpower.Recalculate()
	user.Character.RecalculateStats()
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: user})
	defer restoreUsers()

	_ = applyMobEffect_charm(user, mob, room, charmTestSpellData(), out, "Bandit Scout")

	if mob.Character.Charmed == nil {
		return 0
	}
	return mob.Character.Charmed.RoundsRemaining
}

// The headline mechanic: a dominant win holds far longer than a scraped one.
func TestApplyMobEffectCharm_DurationRidesTheMargin(t *testing.T) {
	charmDurationTestConfig(t)

	scraped := charmedRounds(t, 8820, 8830, combat.ChannelDefenceResult{AttackerNormalizedMargin: 0.1})
	dominant := charmedRounds(t, 8821, 8831, combat.ChannelDefenceResult{AttackerNormalizedMargin: 2.0})

	if scraped <= 0 || dominant <= 0 {
		t.Fatalf("precondition: both casts should bind (scraped=%d dominant=%d)", scraped, dominant)
	}
	if dominant <= scraped {
		t.Errorf("dominant win held %d rounds, scraped held %d; the margin is inverted or ignored",
			dominant, scraped)
	}
	// The permanence this slice removes.
	for name, got := range map[string]int{"scraped": scraped, "dominant": dominant} {
		if got == 99999 || got == characters.CharmPermanent {
			t.Errorf("%s bond is still permanent (%d)", name, got)
		}
	}
}

// Spec 15. A sleeping victim forces the crit, which returns from the seam above
// its margin assignment and so reads zero -- the MINIMUM. Without the
// correction, the most decisive charm in the game would buy the shortest bond.
func TestApplyMobEffectCharm_ForcedCritBuysTheCeiling(t *testing.T) {
	charmDurationTestConfig(t)

	forced := charmedRounds(t, 8822, 8832,
		combat.ChannelDefenceResult{AttackerCrit: true, AttackerNormalizedMargin: 0})
	scraped := charmedRounds(t, 8823, 8833,
		combat.ChannelDefenceResult{AttackerNormalizedMargin: 0.1})

	if forced != charmDurationFor(charmMarginCeiling) {
		t.Errorf("forced crit held %d rounds, want the ceiling %d",
			forced, charmDurationFor(charmMarginCeiling))
	}
	if forced <= scraped {
		t.Errorf("a SLEEPING victim held %d rounds vs a scraped win's %d -- the inversion is back",
			forced, scraped)
	}
}
