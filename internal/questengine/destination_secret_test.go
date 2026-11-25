package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Guidance may send a player through a secret exit only once they found it
// (#781, owner ruling 1: one rule everywhere). Being on the player's map or
// having been there recently no longer counts.
func TestSecretExitsKnownTo(t *testing.T) {
	const here, beyond = 61001, 61002
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		here:   {RoomId: here, Zone: "zk"},
		beyond: {RoomId: beyond, Zone: "zk"},
	}, map[string]*rooms.ZoneConfig{}))

	u := destUser(here)
	known := secretExitsKnownTo(u)
	if known(here, "west", beyond) {
		t.Fatalf("a secret exit the player has not found must not be used")
	}

	u.Character.AddDiscovery(here, characters.SecretExitDiscoveryKey("west"))
	if !known(here, "west", beyond) {
		t.Errorf("a secret exit `search` found must be usable")
	}
	if known(here, "north", beyond) {
		t.Errorf("finding the west exit must not reveal the north one")
	}

	v := destUser(here)
	v.Character.MarkRoomVisited("zk", beyond)
	if secretExitsKnownTo(v)(here, "north", beyond) {
		t.Errorf("the room beyond on the player's map must no longer open an unfound secret exit")
	}

	w := destUser(here)
	rooms.LoadRoom(beyond).MarkVisited(w.UserId, rooms.VisitorUser)
	if secretExitsKnownTo(w)(here, "north", beyond) {
		t.Errorf("a recent visit to the room beyond must no longer open an unfound secret exit")
	}
}

// ResolveDestination hands the nearest search the player's own knowledge.
func TestResolveDestination_PassesTheSecretExitsThePlayerKnows(t *testing.T) {
	var got mapper.SecretExitKnown
	installDestWorld(t, destWorld{
		zones: map[int]string{1: "z", 5: "z"},
		near: func(from int, in func(int) bool, zones []string, known mapper.SecretExitKnown) (int, int, string, bool) {
			got = known
			return 5, 5, "north", true
		},
	})
	sentinel := false
	prev := secretExitsKnownTo
	secretExitsKnownTo = func(*users.UserRecord) mapper.SecretExitKnown {
		return func(int, string, int) bool { sentinel = true; return true }
	}
	t.Cleanup(func() { secretExitsKnownTo = prev })

	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Room: 5}}})
	e.ResolveDestination(destUser(1), 700, "start")
	if got == nil {
		t.Fatalf("the nearest search got no secret-exit rule")
	}
	got(1, "west", 2)
	if !sentinel {
		t.Errorf("the nearest search got a rule other than the player's")
	}
}
