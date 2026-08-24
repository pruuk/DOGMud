package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A fielded companion holds a slice of its owner's Conviction. Char.Vitals
// carries that figure to the web client as a PUSH-ONLY snapshot, so a release
// that emits no event leaves the client showing a reservation for a companion
// that no longer exists.
//
// dismiss.go has publishReleasedReservation for this, and the charm-expiry path
// does the same two calls. Death was the third site and it had neither, for
// EVERY companion type -- summons, the homunculus, charmed creatures. The regen
// tick cannot cover for it: NewRound_AutoHeal republishes only when health moved
// OR the player still has a companion, so losing your LAST companion at full
// health is exactly the case that never self-corrects.
func TestCompanionCleanup_DeathRepublishesVitals(t *testing.T) {
	const userId = 9187

	u := users.NewTestUser(userId, "bereaved", "Bereaved", uint64(userId))
	u.Character.ConvictionMax.Value = 500
	u.Character.Conviction = 500
	u.Character.Companions = []characters.CompanionInfo{{
		MobId:             9904,
		InstanceId:        779,
		Name:              "Gravelfist",
		SourceType:        characters.CompanionConjured,
		ConvictionReserve: 150,
	}}

	restore := users.SeedUsersForTest(map[int]*users.UserRecord{userId: u})
	defer restore()

	events.DrainQueuedVitalsChangedForTest(userId)
	defer events.DrainQueuedVitalsChangedForTest(userId)

	if res := u.Character.GetPoolReservation("conviction", 500); res != 150 {
		t.Fatalf("fixture: reservation = %d, want 150", res)
	}

	CompanionCleanup(events.MobDeath{
		MobId:         9904,
		InstanceId:    779,
		CharacterName: "Gravelfist",
	})

	if u.Character.GetCompanionByInstanceId(779) != nil {
		t.Error("the dead companion's record survived")
	}
	if got := events.DrainQueuedVitalsChangedForTest(userId); len(got) == 0 {
		t.Error("a companion died and no CharacterVitalsChanged was queued, so " +
			"Char.Vitals is never re-sent and the web client keeps showing a " +
			"reservation for a companion that is dead")
	}
}
