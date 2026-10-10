package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A player who damaged a mob is credited with the kill when it dies (#468).
// The mob is wired through Validate, so every OnCharacterCreated observer is
// attached in its real registration order, instance cleanup included: credit
// must not depend on the instance surviving the other death observers.
func TestMobKillCredit_ThePlayerWhoDamagedItGetsTheKill(t *testing.T) {
	user := users.NewTestUser(7, "killer", "Killer", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{user.UserId: user}))

	mob := newRouteDeathTestMob(t, 100)
	mob.Character.MobInstanceId = mob.InstanceId
	if err := mob.Character.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	mob.Character.TrackPlayerDamage(user.UserId, 100)
	mob.Character.Die(state.ActorRef{UserId: user.UserId}, life.TriggerHealthZero)

	if got := user.Character.KD.TotalKills; got != 1 {
		t.Fatalf("TotalKills = %d, want 1: the kill was not credited", got)
	}
	if got := user.Character.KD.Kills[int(mob.MobId)]; got != 1 {
		t.Fatalf("Kills[%d] = %d, want 1", mob.MobId, got)
	}
}
