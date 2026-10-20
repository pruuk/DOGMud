package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func countBackpack(u *users.UserRecord, itemId int) int {
	n := 0
	for _, itm := range u.Character.GetAllBackpackItems() {
		if itm.ItemId == itemId {
			n++
		}
	}
	return n
}

func mobHolds(m *mobs.Mob, itemId int) bool {
	for _, itm := range m.Character.Items {
		if itm.ItemId == itemId {
			return true
		}
	}
	return false
}

// giveItem stores a fresh copy of itemId and returns the stored copy, as
// give.go finds it with FindInBackpack.
func giveItem(t *testing.T, u *users.UserRecord, itemId int) items.Item {
	t.Helper()
	require.True(t, u.Character.StoreItem(items.New(itemId)))
	for _, itm := range u.Character.GetAllBackpackItems() {
		if itm.ItemId == itemId {
			return itm
		}
	}
	t.Fatalf("item %d not stored", itemId)
	return items.Item{}
}

func TestGiveResolve_EligibleHandInPlaysTheTopic(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	itm := giveItem(t, user, 10001)
	room := mustRoom(t, user)

	require.True(t, conversation.Give(user, room, mob, itm))
	mine, theirs := drained(1), drained(2)
	say := strings.Index(mine, `You say, "<ansi fg="saytext">Here is the purse`)
	give := strings.Index(mine, "You give the")
	reply := strings.Index(mine, "My thanks.")
	require.GreaterOrEqual(t, say, 0, mine)
	require.Greater(t, give, say, mine)
	require.Greater(t, reply, give, mine)
	require.Contains(t, theirs, "Here is the purse you were looking for.")
	require.Contains(t, theirs, "gave their")
	require.Contains(t, theirs, "My thanks.")
	require.Equal(t, 0, countBackpack(user, 10001))
	require.False(t, mobHolds(mob, 10001), "the topic takes the item; the mob does not keep it")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// Giving and choosing the topic from the menu print the same lines.
func TestGiveResolve_SameOutputAsChoosingTheTopic(t *testing.T) {
	user, mob, done := fixtureWith(t, rewardFixtureYAML)
	defer done()
	room := mustRoom(t, user)

	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, room, mob)
	drained(1)
	drained(2)
	require.True(t, user.RouteMenuInput("1", true))
	chosenMine, chosenTheirs := drained(1), drained(2)
	require.False(t, user.HasOpenMenu(), "no topic is left after the hand-in")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))

	itm := giveItem(t, user, 10001)
	require.True(t, conversation.Give(user, room, mob, itm))
	givenMine, givenTheirs := drained(1), drained(2)
	require.Equal(t, chosenMine, givenMine)
	require.Equal(t, chosenTheirs, givenTheirs)
}

func TestGiveResolve_WantedButIneligibleIsHandedBack(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	user.Character.QuestProgress = map[int]string{9: "end"}
	itm := giveItem(t, user, 10001)

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	require.Contains(t, drained(1), "hands it back")
	require.Equal(t, 1, countBackpack(user, 10001))
	require.False(t, mobHolds(mob, 10001))
	require.Empty(t, questTokensFor(1))
	require.False(t, user.HasOpenMenu())
}

func TestGiveResolve_UnwantedItemIsNotHandled(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	itm := giveItem(t, user, 30001)

	require.False(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	require.Equal(t, 1, countBackpack(user, 30001))
	require.Empty(t, drained(1))
	require.Empty(t, drained(2))
	require.Empty(t, questTokensFor(1))
}

func TestGiveResolve_TwiceCannotCompleteTwice(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	room := mustRoom(t, user)
	first := giveItem(t, user, 10001)
	giveItem(t, user, 10001)
	require.Equal(t, 2, countBackpack(user, 10001))

	require.True(t, conversation.Give(user, room, mob, first))
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
	// The queued event is what records this in the game.
	user.Character.QuestProgress = map[int]string{9: "end"}
	drained(1)

	var second items.Item
	for _, itm := range user.Character.GetAllBackpackItems() {
		if itm.ItemId == 10001 {
			second = itm
		}
	}
	require.True(t, conversation.Give(user, room, mob, second))
	require.Contains(t, drained(1), "hands it back")
	require.Equal(t, 1, countBackpack(user, 10001))
	require.Empty(t, questTokensFor(1))
}

// give is a plain command: an open prompt does not stop the hand-in, but no
// menu opens over the prompt afterwards.
func TestGiveResolve_OpenPromptStillResolvesButOpensNoMenu(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	itm := giveItem(t, user, 10001)
	user.StartPrompt("build", "")
	defer user.ClearPrompt()

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	require.Contains(t, drained(1), "My thanks.")
	require.False(t, user.HasOpenMenu())
	require.NotNil(t, user.GetPrompt(), "the prompt is untouched")
}

// Charmed NPCs offer no topics (controller ruling, as the marker does).
func TestGiveResolve_CharmedMobIsNotHandled(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	mob.Character.Charmed = characters.NewCharm(2, characters.CharmPermanent, "")
	itm := giveItem(t, user, 10001)

	require.False(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	require.Equal(t, 1, countBackpack(user, 10001))
	require.Empty(t, questTokensFor(1))
}
