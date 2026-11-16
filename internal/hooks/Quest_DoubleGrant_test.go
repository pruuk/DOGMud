package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// #660: a second grant of a step the player already holds is a no-op. The
// Awakening's start was granted twice (a room trigger, then the Landing
// script) and the new-quest banner printed twice. Any path that grants a held
// token (dialogue, an item's token, a mob's givequest, a behaviour tree) did
// the same, and a second end grant would pay the rewards again.
func TestHandleQuestUpdate_RegrantOfAHeldStepIsSilent(t *testing.T) {
	setupResumeQuest(t)
	u := resumeUser(t, 9507)

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9507, QuestToken: "999050-start"}))
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9507, QuestToken: "999050-start"}))
	require.Equal(t, 1, strings.Count(questLogText(u), "Given a new quest"), "the banner printed once per grant")

	// A grant of a step behind the one held is no-op too.
	u.Character.QuestProgress[999050] = "shoot"
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9507, QuestToken: "999050-start"}))
	require.Equal(t, "shoot", u.Character.GetQuestProgress()[999050])
	require.Equal(t, 1, strings.Count(questLogText(u), "Given a new quest"))
}

// The two producers that set the token before queueing the event (a
// behaviour tree's grant_quest and the quest engine's end step) mark it
// PreGranted, and the handler still runs its banner and rewards once.
func TestHandleQuestUpdate_PreGrantedEndStillCompletes(t *testing.T) {
	setupResumeQuest(t)
	u := resumeUser(t, 9508)
	u.Character.QuestProgress = map[int]string{999050: "shoot"}
	require.True(t, u.Character.GiveQuestToken("999050-end"), "precondition: the producer sets the token")

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9508, QuestToken: "999050-end", PreGranted: true}))
	require.Equal(t, 1, strings.Count(questLogText(u), "Completed a quest"))

	// The same end granted again, unmarked, completes nothing a second time.
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9508, QuestToken: "999050-end"}))
	require.Equal(t, 1, strings.Count(questLogText(u), "Completed a quest"))
}
