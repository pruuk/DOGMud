package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFocusQuestId_ValidLastQuest(t *testing.T) {
	c := &Character{LastQuestId: 71, QuestProgress: map[int]string{65: "discover", 71: "start"}}
	assert.Equal(t, 71, c.GetFocusQuestId())
}

func TestGetFocusQuestId_StaleLastQuest_FallsBackToLowestActive(t *testing.T) {
	// LastQuestId points at a quest no longer in progress (the smoketester case).
	c := &Character{LastQuestId: 87, QuestProgress: map[int]string{
		2: "end", 71: "start", 65: "discover", 30: "end",
	}}
	// Lowest-id non-"end" quest = 65.
	assert.Equal(t, 65, c.GetFocusQuestId())
}

func TestGetFocusQuestId_ZeroLastQuest_FallsBack(t *testing.T) {
	c := &Character{LastQuestId: 0, QuestProgress: map[int]string{67: "start", 71: "start"}}
	assert.Equal(t, 67, c.GetFocusQuestId())
}

func TestGetFocusQuestId_AllComplete_LowestId(t *testing.T) {
	c := &Character{LastQuestId: 0, QuestProgress: map[int]string{2: "end", 30: "end"}}
	assert.Equal(t, 2, c.GetFocusQuestId())
}

func TestGetFocusQuestId_NoQuests(t *testing.T) {
	c := &Character{LastQuestId: 87, QuestProgress: map[int]string{}}
	assert.Equal(t, 0, c.GetFocusQuestId())
}

func TestClearQuestToken_ClearsStaleFocus(t *testing.T) {
	// Repeatable-quest completion (or explicit removal) of the focused quest
	// must not leave LastQuestId dangling at the removed quest.
	c := &Character{LastQuestId: 87, QuestProgress: map[int]string{87: "start", 65: "discover"}}
	c.ClearQuestToken("87-start")
	_, still := c.QuestProgress[87]
	assert.False(t, still, "quest 87 removed from progress")
	assert.Equal(t, 0, c.LastQuestId, "LastQuestId cleared when it pointed at the removed quest")
	assert.Equal(t, "discover", c.QuestProgress[65], "other quests untouched")
}

func TestClearQuestToken_KeepsUnrelatedFocus(t *testing.T) {
	// Clearing a quest that is NOT the focus leaves the focus intact.
	c := &Character{LastQuestId: 65, QuestProgress: map[int]string{87: "start", 65: "discover"}}
	c.ClearQuestToken("87-start")
	assert.Equal(t, 65, c.LastQuestId, "focus on a different quest is preserved")
}

// stubSecretQuests makes the listed quest ids secret for one test.
func stubSecretQuests(t *testing.T, ids ...int) {
	t.Helper()
	prev := questIsSecret
	secret := map[int]bool{}
	for _, id := range ids {
		secret[id] = true
	}
	questIsSecret = func(questId int) bool { return secret[questId] }
	t.Cleanup(func() { questIsSecret = prev })
}

func TestGetFocusQuestId_NeverASecretQuest(t *testing.T) {
	// 2026-10-17 playtest: hint showed "Newcomer's Path" (secret quest 21)
	// because granting its token set LastQuestId and the fallback never
	// skipped secret quests.
	stubSecretQuests(t, 21)

	// LastQuestId at the secret quest: focus goes to the visible active one.
	c := &Character{LastQuestId: 21, QuestProgress: map[int]string{21: "start", 31: "start"}}
	assert.Equal(t, 31, c.GetFocusQuestId())

	// Fallback: the secret quest has the lowest id but is skipped.
	c = &Character{LastQuestId: 0, QuestProgress: map[int]string{21: "start", 28: "start"}}
	assert.Equal(t, 28, c.GetFocusQuestId())

	// Only a secret quest in progress: no focus at all.
	c = &Character{LastQuestId: 21, QuestProgress: map[int]string{21: "start"}}
	assert.Equal(t, 0, c.GetFocusQuestId())

	// The input map is not mutated by the filtering.
	assert.Contains(t, c.QuestProgress, 21)
}

func TestGiveQuestToken_SecretQuestKeepsFocus(t *testing.T) {
	stubSecretQuests(t, 21)
	c := &Character{LastQuestId: 28, QuestProgress: map[int]string{28: "start"}}
	assert.True(t, c.GiveQuestToken("21-start"))
	assert.Equal(t, 28, c.LastQuestId, "a secret quest token must not take the focus")
	assert.True(t, c.GiveQuestToken("31-start"))
	assert.Equal(t, 31, c.LastQuestId, "a visible quest token still takes the focus")
}

func TestGetFocusQuestId_SkipsCompletedLastQuest(t *testing.T) {
	// LastQuestId points at a quest that just completed ("end"). Focus must
	// move to the active quest, not stay on the finished one (the Two Roads ->
	// Find Your Footing handoff case).
	c := &Character{LastQuestId: 29, QuestProgress: map[int]string{29: "end", 31: "start"}}
	assert.Equal(t, 31, c.GetFocusQuestId())
}
