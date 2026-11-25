package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #781 D17: a packmate walking behind its alpha through a secret exit learns
// it; through an open exit it records nothing.
func TestPackFollowerStep_PackmateLearnsTheAlphasSecretExit(t *testing.T) {
	// QuoteMobStep resolves the exit through FindExitByName, which reads the
	// direction aliases; a test binary has none loaded.
	t.Cleanup(keywords.SeedKeywordsForTest())
	here := &rooms.Room{RoomId: 1, Exits: map[string]exit.RoomExit{
		"north": {RoomId: 100, Secret: true},
		"south": {RoomId: 200},
	}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1:   here,
		100: {RoomId: 100},
		200: {RoomId: 200},
	}, map[string]*rooms.ZoneConfig{}))

	follower := &mobs.Mob{InstanceId: 7801}
	follower.Character = *characters.New()
	follower.Character.RoomId = 1
	follower.Character.StaminaMax.Value = 100
	follower.Character.Stamina = 100

	if !packFollowerStep(here, "north")(follower) {
		t.Fatal("a rested packmate should follow its alpha north")
	}
	if !follower.Character.KnowsSecretExit(1, "north") {
		t.Fatal("the packmate walked through the secret north behind its alpha and did not learn it")
	}

	if !packFollowerStep(here, "south")(follower) {
		t.Fatal("a rested packmate should follow its alpha south")
	}
	if follower.Character.KnowsSecretExit(1, "south") {
		t.Fatal("an open exit is not a secret to learn")
	}
}
