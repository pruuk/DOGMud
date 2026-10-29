package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// DoQuestStepMigration treats the shelf like QuestProgress (#375 phase 5): a
// removed step resets to start, a deleted quest is dropped, a live step stays.
func TestMigrateQuestSteps_ReachesTheShelf(t *testing.T) {
	t.Cleanup(quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 97201, Name: "Kept", Steps: []quests.QuestStep{{Id: "start"}, {Id: "track"}, {Id: "end"}}},
	))
	u := NewTestUser(97201, "mq", "Migrate", 97201)
	u.Character.QuestProgress = map[int]string{97201: "track"}
	u.Character.ShelvedQuests = map[int]string{97201: "gone", 97299: "start"}

	require.True(t, migrateQuestSteps(u))
	require.Equal(t, "track", u.Character.QuestProgress[97201], "a live step stays")
	require.Equal(t, map[int]string{97201: "start"}, u.Character.ShelvedQuests,
		"a removed shelved step resets to start; a deleted quest is dropped")

	require.False(t, migrateQuestSteps(u), "nothing left to change")

	// The active map gets the same treatment. A removed step used to be
	// wiped: quests.GetQuest is nil for a live quest on an unknown step, so
	// the reset branch never ran.
	u.Character.QuestProgress = map[int]string{97201: "gone", 97299: "start"}
	require.True(t, migrateQuestSteps(u))
	require.Equal(t, map[int]string{97201: "start"}, u.Character.QuestProgress)

	u.Character.QuestProgress = nil
	u.Character.ShelvedQuests = map[int]string{97299: "start"}
	require.True(t, migrateQuestSteps(u), "a shelf alone is still migrated")
	require.Nil(t, u.Character.ShelvedQuests, "an empty shelf is dropped")
}
