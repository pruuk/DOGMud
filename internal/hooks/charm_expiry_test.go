package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// expiringBond stands up an owner in a room with a charmed creature whose clock
// has just run out, and hands both back so a test can drive tickMobCharmState
// over them. sourceType is a knob because gate 1 turns on it.
func expiringBond(
	t *testing.T,
	roomId, instId int,
	sourceType characters.CompanionSourceType,
) (*users.UserRecord, *mobs.Mob, func()) {
	t.Helper()
	charmDurationTestConfig(t)

	room, cleanupRoom := seedHookRoom(t, roomId)

	mob := charmTestMob(t, instId, roomId)
	mobs.SetInstanceForTest(mob.InstanceId, mob)

	user := users.NewTestUser(1, "charmer", "Charmer", 9101)
	user.Character.RoomId = roomId
	user.Character.Stats.Charisma.Base = 150
	user.Character.Stats.Charisma.Recalculate()
	user.Character.Stats.Willpower.Base = 150
	user.Character.Stats.Willpower.Recalculate()
	user.Character.RecalculateStats()
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: user})

	// Bind it for real through the production arm so the companion record, the
	// reservation and the charm link are all exactly what play produces...
	_ = applyMobEffect_charm(user, mob, room, charmTestSpellData(),
		combat.ChannelDefenceResult{AttackerNormalizedMargin: 2.0}, "Bandit Scout")

	// ...then relabel the source when the test is asking about a non-charm
	// companion, and run the clock out.
	if comp := user.Character.GetCompanionByInstanceId(mob.InstanceId); comp != nil {
		comp.SourceType = sourceType
	}
	if mob.Character.Charmed != nil {
		mob.Character.Charmed.RoundsRemaining = 0
	}

	return user, mob, func() {
		restoreUsers()
		mobs.SetInstanceForTest(instId, nil)
		cleanupRoom()
	}
}

// The headline of the risk half: when a charmed bond lapses with its owner
// standing there, the creature turns on them.
func TestCharmExpiry_PresentOwner_ProducesTheGrudge(t *testing.T) {
	user, mob, cleanup := expiringBond(t, 8840, 8850, characters.CompanionCharmed)
	defer cleanup()

	tickMobCharmState(mob)

	if mob.Character.IsCharmed() {
		t.Error("the bond should be broken after expiry")
	}
	if user.Character.GetCompanionByInstanceId(mob.InstanceId) != nil {
		t.Error("the companion record should be gone after expiry")
	}
	if mob.Character.Aggro == nil || mob.Character.Aggro.UserId != user.UserId {
		t.Fatalf("the creature should be attacking its former owner, aggro=%+v", mob.Character.Aggro)
	}
}

// RemoveCompanion does NOT release the reservation on its own -- the reserve is
// derived during RecalculateStats. Assert the DERIVED value, never
// GetPoolReservation: that helper recomputes from the live companion slice and
// so reads 0 the instant the entry is dropped, which would pass even if
// RecalculateStats were never called.
func TestCharmExpiry_ReleasesTheReservation(t *testing.T) {
	user, mob, cleanup := expiringBond(t, 8841, 8851, characters.CompanionCharmed)
	defer cleanup()

	bound := user.Character.EffectivePoolMax(characters.PoolConviction)
	full := user.Character.ConvictionMax.Value
	if bound >= full {
		t.Fatalf("precondition: a live charm should reserve conviction (usable=%d max=%d)", bound, full)
	}

	tickMobCharmState(mob)

	if got := user.Character.EffectivePoolMax(characters.PoolConviction); got != full {
		t.Errorf("usable conviction is %d after the bond broke, want the full %d "+
			"-- the reserve was not released", got, full)
	}
}

// Five other systems put a creature in Charmed state. A conjured creature
// turning on the conjurer would be a bug, not a mechanic.
func TestCharmExpiry_SummonedCompanionNeverGrudges(t *testing.T) {
	user, mob, cleanup := expiringBond(t, 8842, 8852, characters.CompanionSummoned)
	defer cleanup()

	tickMobCharmState(mob)

	if mob.Character.Aggro != nil {
		t.Errorf("a summoned companion must never turn on its summoner, aggro=%+v",
			mob.Character.Aggro)
	}
	_ = user
}

// Spec 3.10 anti-grief: a bond lapsing while the caster is elsewhere must not
// manufacture a creature that hunts them across zones.
func TestCharmExpiry_AbsentOwner_NoGrudge(t *testing.T) {
	user, mob, cleanup := expiringBond(t, 8843, 8853, characters.CompanionCharmed)
	defer cleanup()

	user.Character.RoomId = 8899 // walked away

	tickMobCharmState(mob)

	if mob.Character.Aggro != nil {
		t.Errorf("an absent owner must not be hunted, aggro=%+v", mob.Character.Aggro)
	}
	if mob.Character.IsCharmed() {
		t.Error("the bond should still be broken even with no grudge")
	}
}

// Firing a grudge at someone whose connection just dropped is indefensible.
func TestCharmExpiry_LinkDeadOwner_NoGrudge(t *testing.T) {
	user, mob, cleanup := expiringBond(t, 8844, 8854, characters.CompanionCharmed)
	defer cleanup()

	user.Character.SetAdjective(`zombie`, true)

	tickMobCharmState(mob)

	if mob.Character.Aggro != nil {
		t.Errorf("a link-dead owner must not be attacked, aggro=%+v", mob.Character.Aggro)
	}
}

// characters.CharmPermanent is -1, not 0. It must be inert on both the
// decrement and the expiry gate, or every summon, the homunculus and the
// tutorial Guide would eventually break free.
func TestCharmExpiry_PermanentSentinelNeverExpires(t *testing.T) {
	user, mob, cleanup := expiringBond(t, 8845, 8855, characters.CompanionCharmed)
	defer cleanup()

	mob.Character.Charmed.RoundsRemaining = characters.CharmPermanent

	tickMobCharmDuration(mob)
	if got := mob.Character.Charmed.RoundsRemaining; got != characters.CharmPermanent {
		t.Errorf("the permanent sentinel decremented to %d", got)
	}

	tickMobCharmState(mob)

	if !mob.Character.IsCharmed() {
		t.Error("a permanent bond must never expire")
	}
	if mob.Character.Aggro != nil {
		t.Errorf("a permanent bond must never grudge, aggro=%+v", mob.Character.Aggro)
	}
	_ = user
}
