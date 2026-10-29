package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func cullQuest() *quests.Quest {
	return &quests.Quest{
		QuestId: 53, Name: "Cull the Wash", Repeatable: true, CooldownRounds: 300,
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Garve has asked you to thin out the bandits in the wash again: a bandit scout first, then a bandit squatter."},
			{Id: "end"},
		},
		Rewards: quests.QuestReward{Gold: 6, SkillInfo: "weapon-combat:1"},
	}
}

func TestCardBody_ObjectiveRewardAndRepeat(t *testing.T) {
	body := cardBody(cullQuest(), quests.RewardNames{}, false)
	require.Equal(t, []string{
		"Garve has asked you to thin out the bandits in the wash again: a bandit scout first, then a bandit squatter.",
		"Reward: 6 gold and training in weapon combat.",
		"You can take this on again after a while.",
	}, body)
}

// A repeat run pays no training the character already has, so the card
// does not promise it.
func TestCardBody_SkillAlreadyMetLeavesTrainingOut(t *testing.T) {
	user := users.NewTestUser(7103, "trained", "Trainia", 7103)
	require.Contains(t, cardBody(cullQuest(), cardNames(user.Character), false),
		"Reward: 6 gold and training in weapon combat.")
	user.Character.TrainSkill("weapon-combat", 1)
	require.Contains(t, cardBody(cullQuest(), cardNames(user.Character), false),
		"Reward: 6 gold.")
}

func TestCardBody_LastRun(t *testing.T) {
	body := cardBody(cullQuest(), quests.RewardNames{}, true)
	require.Equal(t, "This is the last time this work will be offered to you.", body[len(body)-1])
}

func TestCardBody_NotRepeatableNoRewardNoObjective(t *testing.T) {
	q := &quests.Quest{QuestId: 9, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}}
	require.Empty(t, cardBody(q, quests.RewardNames{}, false))
}

func TestCardBody_NilQuest(t *testing.T) {
	require.Empty(t, cardBody(nil, quests.RewardNames{}, false))
}

func TestCardBody_SummaryKeepsItsOwnStop(t *testing.T) {
	q := cullQuest()
	q.Repeatable = false
	q.Rewards.Summary = "Garve will make it worth your while."
	require.Equal(t, "Reward: Garve will make it worth your while.", cardBody(q, quests.RewardNames{}, false)[1])
}

func TestPlayerState_SetsQuestOpen(t *testing.T) {
	user := users.NewTestUser(7102, "qopen", "Qopenia", 7102)
	require.NotNil(t, PlayerState(user).QuestOpen)
}

// Shelved reads the character's shelf, so a resumed offer follows owner
// ruling 2 (#375 phase 5).
func TestPlayerState_ShelvedReadsTheShelf(t *testing.T) {
	user := users.NewTestUser(7103, "qshelf", "Qshelfia", 7103)
	user.Character.QuestProgress = map[int]string{42: "track"}
	ps := PlayerState(user)
	require.NotNil(t, ps.Shelved)
	require.False(t, ps.Shelved(42))
	require.True(t, user.Character.ShelveQuest(42))
	require.True(t, ps.Shelved(42))
}
