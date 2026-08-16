package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Vitals carries conviction_reserved to the web client, and it is a
// push-only snapshot: the client shows whatever was last sent until something
// sends again. A 2026-08-15 playtest dismissed a companion and watched the
// client keep reporting the released reservation while `status` and the prompt
// bar had both let go of it, because those two read GetPoolReservation live and
// the client cannot.
//
// Nothing on the dismiss path emitted an event, and the regen tick could not
// cover for it: NewRound_AutoHeal republishes only when health actually moved
// OR when the player still has a companion, so dismissing the LAST one at full
// health is precisely the case that never self-corrects.

const dismissUserId = 9188

func seedDismisser(t *testing.T, reserve int) (*users.UserRecord, *rooms.Room, func()) {
	t.Helper()

	u := users.NewTestUser(dismissUserId, "releaser", "Releaser", uint64(dismissUserId))
	u.Character.ConvictionMax.Value = 500
	u.Character.Conviction = 500
	u.Character.Companions = []characters.CompanionInfo{{
		MobId:             9902,
		InstanceId:        777,
		Name:              "Gravelfist",
		SourceType:        characters.CompanionConjured,
		ConvictionReserve: reserve,
	}}

	template := &mobs.Mob{MobId: 9902}
	template.Character.Name = "stone golem"

	instance := &mobs.Mob{MobId: 9902, InstanceId: 777}
	instance.Character.Name = "Gravelfist"

	cleanMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{9902: template},
		map[int]*mobs.Mob{777: instance},
	)
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{dismissUserId: u})
	room := &rooms.Room{RoomId: 999904}

	events.DrainQueuedMessagesForTest(dismissUserId)
	events.DrainQueuedVitalsChangedForTest(dismissUserId)

	return u, room, func() {
		events.DrainQueuedMessagesForTest(dismissUserId)
		events.DrainQueuedVitalsChangedForTest(dismissUserId)
		cleanUsers()
		cleanMobs()
	}
}

func TestDismiss_RepublishesVitalsSoNoPhantomReservationSurvives(t *testing.T) {
	u, room, cleanup := seedDismisser(t, 150)
	defer cleanup()

	if res := u.Character.GetPoolReservation("conviction", 500); res != 150 {
		t.Fatalf("fixture: reservation = %d, want 150", res)
	}

	handled, err := Dismiss("Gravelfist", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("dismiss errored: handled=%v err=%v", handled, err)
	}

	// The live readers (prompt bar, status) let go immediately. This is the
	// state the client has to be told about.
	if res := u.Character.GetPoolReservation("conviction", 500); res != 0 {
		t.Fatalf("reservation after dismiss = %d, want 0", res)
	}

	if got := events.DrainQueuedVitalsChangedForTest(dismissUserId); len(got) == 0 {
		t.Error("dismiss queued no CharacterVitalsChanged, so Char.Vitals is never " +
			"re-sent and the web client keeps showing a reservation for a companion " +
			"that no longer exists")
	}
}

// The same obligation on the path where the companion instance is already gone
// (logged out, despawned, reaped). The record still carries the reservation, so
// removing it still has to be published.
func TestDismiss_OfflineCompanion_AlsoRepublishesVitals(t *testing.T) {
	u, room, cleanup := seedDismisser(t, 150)
	defer cleanup()

	// Point the record at an instance that does not exist.
	u.Character.Companions[0].InstanceId = 778

	handled, err := Dismiss("Gravelfist", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("dismiss errored: handled=%v err=%v", handled, err)
	}

	if res := u.Character.GetPoolReservation("conviction", 500); res != 0 {
		t.Fatalf("reservation after dismiss = %d, want 0", res)
	}

	if got := events.DrainQueuedVitalsChangedForTest(dismissUserId); len(got) == 0 {
		t.Error("dismissing an offline companion queued no CharacterVitalsChanged")
	}
}
