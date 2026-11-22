package acquire

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allReasons is every AcquireReason in acquire.go. The guard
// TestGainReasonsMatchTheAcquireSeam keeps acquire.go and the quest loader
// equal; this list is checked against the loader's below so a new reason
// cannot skip this test.
var allReasons = []AcquireReason{
	AcquireGet, AcquireGive, AcquireBuy, AcquireLoot, AcquireForage,
	AcquireHarvest, AcquireSteal, AcquireSearch, AcquireStorage, AcquireVault,
	AcquireAuction, AcquireTopic, AcquireCraft, AcquireSalvage, AcquireReward,
	AcquireQuestGive, AcquireTreeGive, AcquireMail, AcquireShieldSwap, AcquirePlant,
}

// #940 (owner ruling 2026-11-22): only a quest's hand-out is marked
// quest-given, a topic's givesItem and a quest trigger's give_item. Every
// other way in, the quest reward included, stores an unmarked copy.
func TestAcquireItemForChar_MarksOnlyQuestHandOuts(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	for _, reason := range allReasons {
		t.Run(string(reason), func(t *testing.T) {
			user := &users.UserRecord{UserId: 9401, Character: characters.New()}
			require.True(t, AcquireItem(user, newTestItem(94001), reason))
			require.Len(t, user.Character.Items, 1)
			want := reason == AcquireTopic || reason == AcquireQuestGive
			assert.Equal(t, want, user.Character.Items[0].IsQuestGiven(), "the stored copy")
			assert.Equal(t, want, MarksQuestGiven(reason))

			got := events.DrainQueuedItemOwnershipForTest(user.UserId)
			require.Len(t, got, 1)
			assert.Equal(t, want, got[0].Item.IsQuestGiven(), "the gain event carries the copy as stored")
		})
	}
}

// A quest reward is paid once and no NPC replaces it, so it is never marked
// (plan decision, 2026-11-22).
func TestAcquireReward_IsNotMarked(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	user := &users.UserRecord{UserId: 9402, Character: characters.New()}
	require.True(t, AcquireItemOrDrop(user, newTestItem(94002), AcquireReward))
	require.Len(t, user.Character.Items, 1)
	assert.False(t, user.Character.Items[0].IsQuestGiven())
}

// A quest's hand-out that falls at the receiver's feet is the same quest gear
// it would have been in their hands (#940).
func TestAcquireOrDrop_QuestHandOutFallsMarked(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	room := &rooms.Room{RoomId: 9403}
	defer rooms.SeedRoomsForTest(map[int]*rooms.Room{9403: room}, nil)()
	char := newWeakChar()
	char.RoomId = 9403

	require.False(t, AcquireOrDrop(char, 0, 0, newHeavyTestItem(94003), AcquireQuestGive))
	require.Len(t, room.Items, 1)
	assert.True(t, room.Items[0].IsQuestGiven())

	room.Items = nil
	require.False(t, AcquireOrDrop(char, 0, 0, newHeavyTestItem(94003), AcquireReward))
	require.Len(t, room.Items, 1)
	assert.False(t, room.Items[0].IsQuestGiven(), "a reward is never marked")
}

// The test's own list must name every reason, or a new one would go untested.
// quests.TriggerGainReasons is the loader's copy, which the guards package
// keeps equal to acquire.go's constants.
func TestAllReasonsListIsComplete(t *testing.T) {
	var mine []string
	for _, r := range allReasons {
		mine = append(mine, string(r))
	}
	assert.ElementsMatch(t, quests.TriggerGainReasons, mine, "update allReasons when a reason is added")
}
