package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func considerTestMob(instId int, name string, roomId int) *mobs.Mob {
	return &mobs.Mob{
		MobId:      1,
		InstanceId: instId,
		HomeRoomId: roomId,
		Character: characters.Character{
			Name:      name,
			RoomId:    roomId,
			Health:    30,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
}

// Consider truncated its target to the FIRST WORD -- it passed args[0] to the
// resolver, so `consider bandit archer` searched for "bandit". Every other
// name-resolving command passes the whole string; look does, and
// room.FindByName has handled multi-word input since the parser seam landed.
//
// The truncation is invisible whenever the first word is distinctive, which is
// why it survived: "bandit" still prefix-matches "Bandit Scout". It bites when
// two creatures share a first word, and then it does not fail loudly -- it
// silently considers the WRONG one, whichever the room lists first.
//
// Found while investigating pre-deploy blocker 0a, whose repro includes
// `consider bandit scout -> You don't see them here.`
func TestConsider_MultiWordNameTargetsTheRightMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)

	const scoutId, archerId = 777, 778

	// The SCOUT is added first, so it is what a bare "bandit" resolves to.
	mobs.SetInstanceForTest(scoutId, considerTestMob(scoutId, "Bandit Scout", room.RoomId))
	defer mobs.SetInstanceForTest(scoutId, nil)
	room.AddMob(scoutId)
	defer room.RemoveMob(scoutId)

	mobs.SetInstanceForTest(archerId, considerTestMob(archerId, "Bandit Archer", room.RoomId))
	defer mobs.SetInstanceForTest(archerId, nil)
	room.AddMob(archerId)
	defer room.RemoveMob(archerId)

	// Preconditions: the room really does resolve a bare "bandit" to the scout,
	// and the full name really does reach the archer. Without both, the
	// assertion below would pass for the wrong reason.
	if _, id := room.FindByName("bandit"); id != scoutId {
		t.Fatalf(`precondition: FindByName("bandit") = %d, want the scout %d`, id, scoutId)
	}
	if _, id := room.FindByName("bandit archer"); id != archerId {
		t.Fatalf(`precondition: FindByName("bandit archer") = %d, want the archer %d`, id, archerId)
	}

	// So `consider bandit archer` must reach the ARCHER. Truncated to "bandit"
	// it reaches the scout instead, and the player is told about the wrong
	// creature with no indication anything went wrong.
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Consider("bandit archer", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	joined := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.NotContains(t, joined, "You don't see them here.",
		"consider must resolve a multi-word target")
	assert.Contains(t, joined, "Bandit Archer",
		"consider bandit archer must consider the ARCHER")
	assert.NotContains(t, joined, "Bandit Scout",
		"consider bandit archer must not silently consider the scout")
}
