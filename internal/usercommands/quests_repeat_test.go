package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const repeatTestQuestId = 900053

func repeatTestQuest(limit int, secret bool) *quests.Quest {
	return &quests.Quest{
		QuestId:        repeatTestQuestId,
		Name:           "Wolf Bounty",
		Description:    "Cull the wolves.",
		Repeatable:     true,
		CooldownRounds: 100,
		RepeatLimit:    limit,
		Secret:         secret,
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Hunt the wolves."},
			{Id: "end", Description: "Done."},
		},
	}
}

func TestFinishedRepeatRows_Status(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)

	c := characters.New()
	c.QuestCompletions = map[int]int{repeatTestQuestId: 1}

	t.Run("cooldown active", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
		c.SetQuestCooldown(repeatTestQuestId, 100)
		rows := finishedRepeatRows(c, map[int]string{}, false)
		require.Len(t, rows, 1)
		assert.Equal(t, "Not offered again yet.", rows[0].Description)
		assert.True(t, rows[0].Repeatable)
		assert.Equal(t, "Wolf Bounty", rows[0].Name)
	})

	t.Run("cooldown over", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
		util.SetRoundCountForTest(1200)
		rows := finishedRepeatRows(c, map[int]string{}, false)
		require.Len(t, rows, 1)
		assert.Equal(t, "Ready to take on again.", rows[0].Description)
	})

	t.Run("lifetime cap reached", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(1, false)))
		rows := finishedRepeatRows(c, map[int]string{}, false)
		require.Len(t, rows, 1)
		assert.Equal(t, "Done for good.", rows[0].Description)
	})
}

func TestFinishedRepeatRows_SkipsInProgressAndSecret(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	c := characters.New()
	c.QuestCompletions = map[int]int{repeatTestQuestId: 2}

	t.Run("in progress", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
		rows := finishedRepeatRows(c, map[int]string{repeatTestQuestId: "start"}, false)
		assert.Empty(t, rows)
	})

	t.Run("secret", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, true)))
		assert.Empty(t, finishedRepeatRows(c, map[int]string{}, false))
		assert.Len(t, finishedRepeatRows(c, map[int]string{}, true), 1)
	})

	// A shelved repeatable is held, just set aside: it gets its set-aside
	// row, not a finished one (#375 phase 5).
	t.Run("shelved", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
		c3 := characters.New()
		c3.QuestCompletions = map[int]int{repeatTestQuestId: 1}
		c3.ShelvedQuests = map[int]string{repeatTestQuestId: "start"}
		assert.Empty(t, finishedRepeatRows(c3, map[int]string{}, false))
	})

	t.Run("never completed", func(t *testing.T) {
		t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
		c2 := characters.New()
		assert.Empty(t, finishedRepeatRows(c2, map[int]string{}, false))
	})
}

// A player whose only finished quests are repeatables is pointed at
// `quests all`, not told they have no quests.
func TestQuestsCommand_OnlyFinishedRepeatablesPointsAtQuestsAll(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)
	user, room := aimScene(t, aimFull)
	useDogmudTemplates(t)
	t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
	user.Character.QuestProgress = map[int]string{}
	user.Character.QuestCompletions = map[int]int{repeatTestQuestId: 1}

	_, err := Quests("", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	assert.NotContains(t, told, "You have no quests yet.")
	assert.Contains(t, told, "quests all")
}

// `quests all+` shows a finished repeatable's status row, not a 0% row for
// the placeholder it gives quests the player holds no progress on.
func TestQuestsCommand_AllPlusShowsAFinishedRepeatablesStatus(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)
	user, room := aimScene(t, aimFull)
	useDogmudTemplates(t)
	t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))
	user.Role = users.RoleAdmin
	user.Character.QuestProgress = map[int]string{}
	user.Character.QuestCompletions = map[int]int{repeatTestQuestId: 1}
	user.Character.SetQuestCooldown(repeatTestQuestId, 100)

	_, err := Quests("all+", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	t.Log("\n" + told)
	assert.Contains(t, told, "Not offered again yet.")
	assert.NotContains(t, told, "Cull the wolves.", "no placeholder description row")
	rows := 0
	for _, line := range strings.Split(told, "\n") {
		if strings.Contains(line, "Wolf Bounty") {
			rows++
			assert.Contains(t, line, "100%", line)
		}
	}
	assert.Equal(t, 1, rows, "one row for the quest")
}

func TestQuestsCommand_TagsRepeatable(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)
	user, room := aimScene(t, aimFull)
	useDogmudTemplates(t)
	t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false)))

	// In progress: the tag shows in plain `quests`.
	user.Character.QuestCompletions = nil
	user.Character.QuestProgress = map[int]string{repeatTestQuestId: "start"}
	_, err := Quests("", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	assert.Contains(t, told, "Wolf Bounty (repeatable)")
	assert.Contains(t, told, "Hunt the wolves.")

	// Finished: completion cleared the progress and counted one run.
	user.Character.QuestProgress = map[int]string{}
	user.Character.QuestCompletions = map[int]int{repeatTestQuestId: 1}
	user.Character.SetQuestCooldown(repeatTestQuestId, 100)

	_, err = Quests("", user, room, 0)
	require.NoError(t, err)
	assert.NotContains(t, aimTold(1), "Wolf Bounty")

	_, err = Quests("all", user, room, 0)
	require.NoError(t, err)
	told = aimTold(1)
	assert.Contains(t, told, "Wolf Bounty (repeatable)")
	assert.Contains(t, told, "Not offered again yet.")
	t.Log("\n" + told)

	// The name cell stays 41 wide, so a tagged and an untagged row line up.
	t.Cleanup(quests.SeedQuestsForTest(repeatTestQuest(0, false), &quests.Quest{
		QuestId: 900054, Name: "Plain Errand", Description: "Do it.",
		Steps: []quests.QuestStep{{Id: "start", Description: "Begin."}, {Id: "end", Description: "Done."}},
	}))
	user.Character.QuestProgress = map[int]string{900054: "start"}
	_, err = Quests("all", user, room, 0)
	require.NoError(t, err)
	told = aimTold(1)
	t.Log("\n" + told)
	barCol := -1
	for _, line := range strings.Split(told, "\n") {
		if !strings.Contains(line, "%") {
			continue
		}
		i := strings.IndexAny(line, "█░")
		if strings.Contains(line, "Wolf Bounty") || strings.Contains(line, "Plain Errand") {
			col := len([]rune(line[:i]))
			if barCol == -1 {
				barCol = col
			}
			assert.Equal(t, barCol, col, line)
		}
	}
}
