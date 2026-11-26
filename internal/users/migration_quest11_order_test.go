package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func seedBranchedQuest11(t *testing.T) {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 11, Name: "The Windwarden's Dilemma",
		Flags: []quests.QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []quests.QuestStep{
			{Id: "start"},
			{Id: "sylara", Branch: "sylara"},
			{Id: "totem", Branch: "sylara"},
			{Id: "rhett", Branch: "rhett"},
			{Id: "samples", Branch: "rhett"},
			{Id: "end"},
		},
	}))
}

// #958: a save on quest 11's retired `sided` step meets DoQuestStepMigration
// first when the boot finds no user index (main.go runs DoUserMigrations
// before any login, and SearchOfflineUsers reads the raw file, skipping
// LoadUser's MigrateQuest11SidedStep). The save goes back to `start` with its
// flag kept, and LoadUser's migration then leaves it there. On its side's
// path the stand reopens to that side only.
func TestQuest11SidedSave_StepMigrationFirst(t *testing.T) {
	seedBranchedQuest11(t)
	u := NewTestUser(97411, "q11", "Sided", 97411)
	u.Character.QuestProgress = map[int]string{11: "sided"}
	u.Character.QuestFlags = map[string]string{"11-branch": "rhett"}

	require.True(t, migrateQuestSteps(u))
	require.Equal(t, "start", u.Character.QuestProgress[11])
	require.Equal(t, "rhett", u.Character.GetQuestFlag("11-branch"), "the flag is kept")

	u.Character.MigrateQuest11SidedStep()
	require.Equal(t, "start", u.Character.QuestProgress[11], "LoadUser's migration leaves start alone")

	require.False(t, u.Character.GiveQuestToken("11-sylara"), "the other side stays shut")
	require.True(t, u.Character.GiveQuestToken("11-rhett"), "its own side reopens")
}

// An alt comes into play through SwapToAlt as well as LoadUser, and gets the
// same quest progress migration: an alt saved on `11-sided` lands on its
// side's step.
func TestSwapToAlt_MigratesQuest11SidedStep(t *testing.T) {
	withUserDataDir(t)
	seedBranchedQuest11(t)
	u := NewTestUser(97413, "q11alt", "Mainchar", 97413)
	alt := characters.Character{
		Name:          "Sidedalt",
		QuestProgress: map[int]string{11: "sided"},
		QuestFlags:    map[string]string{"11-branch": "rhett"},
	}
	require.True(t, characters.SaveAlts(u.UserId, []characters.Character{alt}))

	require.True(t, u.SwapToAlt("Sidedalt"))
	require.Equal(t, "Sidedalt", u.Character.Name)
	require.Equal(t, "rhett", u.Character.QuestProgress[11], "the swapped-in alt lands on its side's step")
}

// A copyover reloads online players through loadUserById, not LoadUser, so it
// must run the same quest migration or a player online on 11-sided across a
// copyover deploy holds a step the quest no longer has.
func TestCopyoverReload_MigratesQuest11SidedStep(t *testing.T) {
	withUserDataDir(t)
	seedBranchedQuest11(t)
	u := NewTestUser(97414, "q11copy", "Copyover", 97414)
	u.Character.QuestProgress = map[int]string{11: "sided"}
	u.Character.QuestFlags = map[string]string{"11-branch": "sylara"}
	require.NoError(t, SaveUser(u))

	got, err := loadUserById(u.UserId)
	require.NoError(t, err)
	require.Equal(t, "sylara", got.Character.QuestProgress[11], "the reloaded player lands on its side's step")
}

// With a user index on disk (every normal boot) only LoadUser meets the save:
// MigrateQuest11SidedStep puts it on its side's step, which the step
// migration then keeps.
func TestQuest11SidedSave_LoadUserMigrationFirst(t *testing.T) {
	seedBranchedQuest11(t)
	u := NewTestUser(97412, "q11b", "Sided", 97412)
	u.Character.QuestProgress = map[int]string{11: "sided"}
	u.Character.QuestFlags = map[string]string{"11-branch": "rhett"}

	u.Character.MigrateQuest11SidedStep()
	require.Equal(t, "rhett", u.Character.QuestProgress[11])
	require.False(t, migrateQuestSteps(u), "a live step is left alone")
	require.Equal(t, "rhett", u.Character.QuestProgress[11])
	require.True(t, u.Character.HasQuest("11-rhett"))
	require.False(t, u.Character.HasQuest("11-sylara"))
}
