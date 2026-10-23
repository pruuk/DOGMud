package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// ask never answers an NPC topic (#375 spec 4 item 7): one private redirect
// to talk, and nothing reaches the room.
func TestAsk_RedirectsToTalk(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, err := Ask("skeleton about bones", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "talk skeleton")
	require.NotContains(t, mine, "Old age.", "ask never answers a topic")
	require.Empty(t, theirs, "nothing happens in the room")
	require.False(t, user.Character.HasHeard("1.bones"))
}

// A bare `ask <npc>` with no words is the same redirect, not a usage prompt
// that teaches ask for NPCs.
func TestAsk_NameAloneRedirectsToTalk(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, err := Ask("skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "talk skeleton")
	require.Empty(t, theirs)
}

// The usage prompt names ask's one remaining use, a bonded companion (R13).
func TestAsk_UsagePromptNamesTheCompanion(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, err := Ask("", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.Contains(t, mine, "ask <companion> <words>")
	require.NotContains(t, mine, "<topic>")
}

// The redirect repeats the name the player typed, never the resolved "#id"
// that a shape aim resolves to.
func TestAsk_RedirectUsesTheTypedName(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Ask("2.shape quest", user, room, events.EventFlag(0))
	told := aimTold(1)
	require.Contains(t, told, "talk 2.shape")
	require.NotContains(t, told, "#")
}

// ask on the player's own companion still goes to the companion module
// (R13), with the to/about words stripped, and never redirects.
func TestAsk_OwnCompanionRoutesToTheModule(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	mob.Character.Charmed = characters.NewCharm(user.UserId, characters.CharmPermanent, "")
	var got string
	companionai.SetAskHandler(func(userId, mobInstanceId int, text string) bool {
		got = text
		return true
	})
	t.Cleanup(func() { companionai.SetAskHandler(nil) })
	_, err := Ask("skeleton about the bones", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Equal(t, "the bones", got)
	require.NotContains(t, drained(1), "talk skeleton")
}

// The companion NPC asker is conversation.AskForOwner (R13): the key the
// owner's command resolved plays that info topic, as the owner choosing it.
func TestCompanionAskNpc_PlaysTheTopicByKey(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	require.True(t, companionai.AskNpc(user.UserId, 100, "bones", true))
	require.Contains(t, drained(1), "Old age.")
	require.True(t, user.Character.HasHeard("1.bones"))
}
