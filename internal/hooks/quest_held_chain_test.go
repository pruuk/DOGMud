package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 tier 1: a quest whose second step is an item the player already
// holds. Granting "start" must run the step start (Hook 1) inside the same
// Quest event, and the new-quest banner must print before the progress banner.
const heldChainQuestId = 999778
const heldChainItemId = 999778

const heldChainYAML = `
questid: 999778
name: Held Chain Quest
description: Fixture for the held-item step start.
steps:
  - id: start
    description: "Started."
  - id: deliver
    description: "Deliver."
  - id: end
    description: "Done."
triggers:
  - event: item_gain
    item: 999778
    conditions:
      has: ["999778-start"]
      missing: ["999778-deliver"]
    actions:
      - grant: "999778-deliver"
`

// heldChainSetup loads the fixture quest into both quest systems, then points
// the data files back at the shipped world so the banner templates render.
func heldChainSetup(t *testing.T) {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	tmp := t.TempDir()
	dir := filepath.Join(tmp, "quests")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "999778-held_chain_quest.yaml"), []byte(heldChainYAML), 0644))

	orig := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	quests.LoadDataFiles()
	questengine.LoadDataFiles()
	world, err := filepath.Abs(filepath.Join("..", "..", "_datafiles", "world", "dogmud"))
	require.NoError(t, err)
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": world}))
	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": orig}) })

	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		heldChainItemId: {ItemId: heldChainItemId, Name: "Test Crate", Type: items.Object, Weight: 0.1},
	}))
}

func heldChainUser(t *testing.T, userId int) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(userId, "heldchain", "HeldChain", 5001)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
	u.Character.Items = append(u.Character.Items, items.New(heldChainItemId))
	return u
}

func indexOfContaining(msgs []string, sub string) int {
	for i, m := range msgs {
		if strings.Contains(m, sub) {
			return i
		}
	}
	return -1
}

// The new-quest banner comes first; the held step's progress banner follows.
func TestHandleQuestUpdate_NewQuestBannerPrintsBeforeAHeldStepsProgress(t *testing.T) {
	heldChainSetup(t)
	u := heldChainUser(t, 781)

	HandleQuestUpdate(events.Quest{UserId: 781, QuestToken: "999778-start"})

	require.Equal(t, "deliver", u.Character.QuestProgress[heldChainQuestId], "the held item advanced the step")

	// The event log records each banner's text in the order it was sent.
	var log []string
	for _, e := range u.EventLog {
		log = append(log, e.What)
	}
	given := indexOfContaining(log, "Given a new quest")
	progress := indexOfContaining(log, "made progress")
	require.GreaterOrEqual(t, given, 0, "new-quest banner logged: %q", log)
	require.GreaterOrEqual(t, progress, 0, "progress banner logged: %q", log)
	require.Less(t, given, progress, "the quest is announced before its first step moves: %q", log)

	// On the wire: new-quest banner, the held line, then the progress banner.
	msgs := events.DrainQueuedMessagesForTest(781)
	require.Len(t, msgs, 3, "%q", msgs)
	require.Contains(t, msgs[1], "You already have the Test Crate.", "the held line sits between the banners: %q", msgs)
}

// The talk menu returns at priority 45, after the Quest event at priority 0:
// by then the grant and the held step start have both landed.
func TestTalkMenuAgain_RunsAfterTheGrantAndTheStepStart(t *testing.T) {
	heldChainSetup(t)
	u := heldChainUser(t, 782)
	t.Cleanup(events.SwapListenersForTest())

	var handled, hadStart, hadDeliver bool
	events.RegisterListener(events.Quest{}, HandleQuestUpdate)
	events.RegisterListener(events.TalkMenuAgain{}, func(e events.Event) events.ListenerReturn {
		handled = true
		hadStart = u.Character.HasQuest("999778-start")
		hadDeliver = u.Character.HasQuest("999778-deliver")
		return events.Continue
	})
	// The order offerCard produces: GiveQuest queues the Quest event, then the
	// card queues TalkMenuAgain.
	events.AddToQueue(events.Quest{UserId: 782, QuestToken: "999778-start"})
	events.AddToQueue(events.TalkMenuAgain{UserId: 782, MobInstanceId: 1}, events.TalkNextOfferPriority)
	events.ProcessEvents()

	require.True(t, handled, "TalkMenuAgain was handled")
	require.True(t, hadStart, "the quest was granted before the menu")
	require.True(t, hadDeliver, "the held step had started before the menu")
}

// RegisterListeners wires TalkMenuAgain to the conversation handler: with the
// real registration and nothing else, the event opens the NPC's talk menu.
func TestRegisterListeners_TalkMenuAgainOpensTheTalkMenu(t *testing.T) {
	defer seedAllRegistries()()
	t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{
		MobId: 1, Zone: "TestZone",
		Topics: []dialogue.Topic{{Key: "bones", Kind: dialogue.KindInfo, Label: "Why do your bones rattle?", Text: "Old age."}},
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Zone = "TestZone"
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	require.False(t, user.HasOpenMenu())
	// The talk menu needs the NPC seen by name: pin the room fully lit.
	room := rooms.LoadRoom(1)
	origLamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room.Lamp = origLamp })

	t.Cleanup(events.SwapListenersForTest())
	RegisterListeners()
	events.DoListeners(events.TalkMenuAgain{UserId: 1, MobInstanceId: 100})

	require.True(t, user.HasOpenMenu(), "RegisterListeners registered HandleTalkMenuAgain")
}
