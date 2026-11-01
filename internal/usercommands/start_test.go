package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The veteran route says the character is "already changed", so it must really
// hold quest 30 at end when the command returns: every gate in the coulee reads
// that token. A plain events.Quest cannot do it (IsTokenAfter refuses `end` on
// empty progress for a quest of several steps), and if it could, it would pay
// the rite's reward lines to someone who skipped the rite. So anything queued
// here must be MarkerOnly, which HandleQuestUpdate stops at before any banner
// or reward.
func TestAutoAwaken_OpensTheCharacterWithoutPayingTheRite(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 30, Name: "The Awakening",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "rite"}, {Id: "woken"}, {Id: "end"}},
	}))

	u := users.GetByUserId(1)
	u.Character.QuestProgress = nil
	u.Character.LastQuestId = 0

	// Why the handler's token path is not the answer, pinned on a scratch
	// character so a change to IsTokenAfter is seen here.
	other := users.NewTestUser(9301, "scratch", "Scratch", 99301)
	require.False(t, other.Character.GiveQuestToken("30-end"),
		"a multi-step quest refuses end on empty progress")

	restore := events.SwapListenersForTest()
	defer restore()
	var queued []events.Quest
	events.RegisterListener(events.Quest{}, func(e events.Event) events.ListenerReturn {
		if q, ok := e.(events.Quest); ok && q.UserId == u.UserId {
			queued = append(queued, q)
		}
		return events.Continue
	})

	autoAwaken(u)
	events.ProcessEvents()

	assert.Equal(t, "end", u.Character.QuestProgress[30], "the veteran must hold quest 30 at end")
	assert.True(t, u.Character.HasQuest("30-end"), "the gates read HasQuest(30-end)")
	assert.Zero(t, u.Character.LastQuestId, "a skipped rite must not take the quest focus")
	for _, q := range queued {
		assert.True(t, q.MarkerOnly, "%s would pay the rite's banner and reward lines", q.QuestToken)
	}
}

func TestOnboardingRoute(t *testing.T) {
	assert.Equal(t, routeNewbie, onboardingRoute("1"))
	assert.Equal(t, routeMudVet, onboardingRoute("2"))
	assert.Equal(t, routeVeteran, onboardingRoute("3"))
	assert.Equal(t, routeMudVet, onboardingRoute(""))
	assert.Equal(t, routeMudVet, onboardingRoute("garbage"))
}
