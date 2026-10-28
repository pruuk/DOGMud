package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	resumeShotId   = 91201 // valued, like the Pouch of Shot
	resumeNoticeId = 91202 // worth nothing, like the Protection Notice
	resumeUserId   = 9121
	resumeRoomId   = 91203
)

func resumeFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{QuestId: 950, Name: "First Shot",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "shoot"}, {Id: "end"}}}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		resumeShotId:   {ItemId: resumeShotId, Name: "pouch of shot", Type: items.Object, Subtype: items.Mundane, Value: 40},
		resumeNoticeId: {ItemId: resumeNoticeId, Name: "notice", Type: items.Object, Subtype: items.Mundane},
	}))
	u := users.NewTestUser(resumeUserId, "resume", "Resume", uint64(resumeUserId))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{resumeUserId: u}))
	events.DrainQueuedMessagesForTest(resumeUserId)
	events.DrainQueuedItemOwnershipForTest(resumeUserId)
	t.Cleanup(func() {
		events.DrainQueuedMessagesForTest(resumeUserId)
		events.DrainQueuedItemOwnershipForTest(resumeUserId)
	})
	return u
}

func holds(u *users.UserRecord, itemId int) bool {
	for _, itm := range u.Character.GetAllCarriedItems() {
		if itm.ItemId == itemId {
			return true
		}
	}
	return false
}

// questLog is the player's quest event log, where each grant banner is
// recorded alongside the line sent.
func questLog(u *users.UserRecord) string {
	var out []string
	for e := range u.EventLog.Items {
		if e.Category == "quest" {
			out = append(out, e.What)
		}
	}
	return strings.Join(out, "\n")
}

// A grant chain for a resumed quest hands over no item of value and no gold
// (owner ruling 2, spec section 1); a worthless missing item still comes.
func TestGameBridge_ResumedHandsOverNothingOfValue(t *testing.T) {
	u := resumeFixture(t)
	u.Character.QuestProgress = map[int]string{950: "shoot"}
	require.True(t, u.Character.ShelveQuest(950))
	gold := u.Character.Gold

	b := NewGameBridge(u, 1)
	b.GrantQuest("950-start")
	require.Equal(t, "shoot", u.Character.GetQuestProgress()[950])
	require.True(t, b.Resumed())
	require.False(t, u.Character.TakeResumed(950), "GrantQuest consumed the resume mark")
	log := questLog(u)
	require.Contains(t, log, "You take up")
	require.NotContains(t, log, "given a new quest")

	require.NoError(t, b.GiveItem(resumeShotId))
	require.False(t, holds(u, resumeShotId), "a valued item does not come back")
	require.NoError(t, b.GiveItem(resumeNoticeId))
	require.True(t, holds(u, resumeNoticeId), "a worthless missing item does")
	require.NoError(t, b.GiveItem(resumeNoticeId))
	n := 0
	for _, itm := range u.Character.GetAllCarriedItems() {
		if itm.ItemId == resumeNoticeId {
			n++
		}
	}
	require.Equal(t, 1, n, "a worthless item already carried is not given twice")
	b.GiveGold(10)
	require.Equal(t, gold, u.Character.Gold, "no gold on a resume")

	first := NewGameBridge(u, 1)
	require.False(t, first.Resumed())
	require.NoError(t, first.GiveItem(resumeShotId))
	require.True(t, holds(u, resumeShotId), "outside a resume, unchanged")
}

// A first grant through GrantQuest is not a resume and keeps its banner.
func TestGameBridge_FirstGrantIsNotResumed(t *testing.T) {
	u := resumeFixture(t)
	b := NewGameBridge(u, 1)
	b.GrantQuest("950-start")
	require.False(t, b.Resumed())
	log := questLog(u)
	require.Contains(t, log, "given a new quest")
	require.NotContains(t, log, "You take up")
}

// resumeEngine is quest 950 with quest 50's handout shape: entering a room
// grants 950-start, and a quest_granted trigger on 950-start gated only on
// missing_item gives the valued pouch and some gold.
func resumeEngine() *Engine {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 950,
		Name:    "First Shot",
		Steps:   []quests.QuestStep{{Id: "start"}, {Id: "shoot"}, {Id: "end"}},
		Triggers: []TriggerDef{
			{Event: "room_enter", Room: resumeRoomId, Actions: []ActionDef{{Grant: "950-start"}}},
			{Event: "quest_granted", QuestToken: "950-start",
				Conditions: Conditions{MissingItem: resumeShotId},
				Actions:    []ActionDef{{GiveItem: resumeShotId}, {GiveGold: 10}}},
		},
	})
	return e
}

// The engine's quest_granted chain runs on the bridge whose GrantQuest
// resumed the quest, so the chain's handout follows ruling 2.
func TestEngine_ResumedGrantChainGivesNothingOfValue(t *testing.T) {
	u := resumeFixture(t)
	u.Character.QuestProgress = map[int]string{950: "shoot"}
	require.True(t, u.Character.ShelveQuest(950))
	gold := u.Character.Gold

	e := resumeEngine()
	b := NewGameBridge(u, resumeRoomId)
	e.Notify("room_enter", EventDetails{UserId: resumeUserId, RoomId: resumeRoomId}, b, b)
	require.Equal(t, "shoot", u.Character.GetQuestProgress()[950])
	require.False(t, holds(u, resumeShotId), "no pouch on a resume")
	require.Equal(t, gold, u.Character.Gold, "no gold on a resume")
	require.Contains(t, questLog(u), "You take up")

	// The same chain on a first accept hands both over.
	v := users.NewTestUser(resumeUserId+1, "first", "First", uint64(resumeUserId+1))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{resumeUserId + 1: v}))
	t.Cleanup(func() { events.DrainQueuedMessagesForTest(resumeUserId + 1) })
	vGold := v.Character.Gold
	fb := NewGameBridge(v, resumeRoomId)
	e.Notify("room_enter", EventDetails{UserId: resumeUserId + 1, RoomId: resumeRoomId}, fb, fb)
	require.Equal(t, "start", v.Character.GetQuestProgress()[950])
	require.True(t, holds(v, resumeShotId), "a first accept gets the pouch")
	require.Equal(t, vGold+10, v.Character.Gold, "a first accept gets the gold")
}
