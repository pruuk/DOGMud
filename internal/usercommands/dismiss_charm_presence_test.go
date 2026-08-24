package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const dismissCharmUserId = 9189

// seedCharmDismisser puts a CHARMED companion in mobRoomId while its owner
// stands in ownerRoomId, so a test can drive Dismiss across a room boundary.
func seedCharmDismisser(t *testing.T, ownerRoomId, mobRoomId int) (
	*users.UserRecord, *mobs.Mob, *rooms.Room, func(),
) {
	t.Helper()

	u := users.NewTestUser(dismissCharmUserId, "breaker", "Breaker", uint64(dismissCharmUserId))
	u.Character.RoomId = ownerRoomId
	u.Character.ConvictionMax.Value = 500
	u.Character.Conviction = 500
	u.Character.Companions = []characters.CompanionInfo{{
		MobId:             9903,
		InstanceId:        778,
		Name:              "Bandit Scout",
		SourceType:        characters.CompanionCharmed,
		ConvictionReserve: 150,
	}}

	template := &mobs.Mob{MobId: 9903}
	template.Character.Name = "bandit scout"

	instance := &mobs.Mob{MobId: 9903, InstanceId: 778}
	instance.Character.Name = "Bandit Scout"
	instance.Character.RoomId = mobRoomId
	instance.Character.Charm(dismissCharmUserId, 100, "")

	cleanMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{9903: template},
		map[int]*mobs.Mob{778: instance},
	)
	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{dismissCharmUserId: u})
	room := &rooms.Room{RoomId: ownerRoomId}

	events.DrainQueuedMessagesForTest(dismissCharmUserId)
	events.DrainQueuedVitalsChangedForTest(dismissCharmUserId)

	return u, instance, room, func() {
		events.DrainQueuedMessagesForTest(dismissCharmUserId)
		events.DrainQueuedVitalsChangedForTest(dismissCharmUserId)
		cleanUsers()
		cleanMobs()
	}
}

// The expiry path gates its grudge on the owner being present, because a
// hostile creature with patrol and pathto behaviour can otherwise follow a
// player across zones. dismiss must not be a way around that rule.
//
// Reaching this in play is very hard -- companions follow closely -- so this is
// a guard, not a bug fix. It exists so the two exits from a charmed bond cannot
// disagree about the same anti-grief rule.
func TestDismiss_AbsentCharmedCreatureDoesNotGetAggro(t *testing.T) {
	u, mob, room, cleanup := seedCharmDismisser(t, 999905, 999906)
	defer cleanup()

	handled, err := Dismiss("Bandit Scout", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("dismiss errored: handled=%v err=%v", handled, err)
	}

	// Dismiss still has to WORK -- the guard suppresses the betrayal, not the
	// command.
	if u.Character.GetCompanionByInstanceId(778) != nil {
		t.Error("the companion record survived a dismiss")
	}
	if res := u.Character.GetPoolReservation("conviction", 500); res != 0 {
		t.Errorf("reservation after dismiss = %d, want 0", res)
	}

	if mob.Character.Aggro != nil {
		t.Errorf("a creature dismissed from another room acquired aggro it can carry "+
			"across zones: %+v", mob.Character.Aggro)
	}
}

// The normal case must keep working: sever a bond with something standing next
// to you and it turns on you.
func TestDismiss_PresentCharmedCreatureStillTurnsOnYou(t *testing.T) {
	u, mob, room, cleanup := seedCharmDismisser(t, 999907, 999907)
	defer cleanup()

	handled, err := Dismiss("Bandit Scout", u, room, 0)
	if err != nil || !handled {
		t.Fatalf("dismiss errored: handled=%v err=%v", handled, err)
	}

	if mob.Character.Aggro == nil || mob.Character.Aggro.UserId != dismissCharmUserId {
		t.Fatalf("the betrayal did not land: aggro=%+v", mob.Character.Aggro)
	}
}
