package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
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

func TestGiveResolve_RefreshesMarkersOnceOnlyWhenPlayed(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	room := mustRoom(t, user)
	itm := giveItem(t, user, 10001)
	events.DrainQueuedNpcMarkersChangedForTest(1)

	require.True(t, conversation.Give(user, room, mob, itm))
	require.Len(t, events.DrainQueuedNpcMarkersChangedForTest(1), 1, "hand-in played")

	user.Character.QuestProgress = map[int]string{9: "end"}
	again := giveItem(t, user, 10001)
	require.True(t, conversation.Give(user, room, mob, again))
	require.Empty(t, events.DrainQueuedNpcMarkersChangedForTest(1), "handed back changes nothing")
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

const shownItemFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: dagger
    kind: info
    label: "Look at this dagger."
    text: "Fine work, that."
    carriesItem: 10001
`

// A shown item (carriesItem) is answered by its topic and never leaves the
// player: give must not fall through to the ordinary give (R8).
func TestGiveResolve_ShownItemPlaysTheTopicAndIsKept(t *testing.T) {
	user, mob, done := fixtureWith(t, shownItemFixtureYAML)
	defer done()
	itm := giveItem(t, user, 10001)

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	mine := drained(1)
	require.Contains(t, mine, "Fine work, that.")
	require.NotContains(t, mine, "You give the", "a shown item is not handed over")
	require.Equal(t, 1, countBackpack(user, 10001), "the item stays with the player")
	require.False(t, mobHolds(mob, 10001))
}

func TestGiveResolve_WantedButIneligibleIsHandedBack(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	user.Character.QuestProgress = map[int]string{9: "end"}
	itm := giveItem(t, user, 10001)

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	require.Contains(t, drained(1), "hands it back")
	require.Empty(t, drained(2), "the hand-back is private")
	require.Equal(t, 1, countBackpack(user, 10001))
	require.False(t, mobHolds(mob, 10001))
	require.Empty(t, questTokensFor(1))
	require.False(t, user.HasOpenMenu())
}

// An offer that takes an item still pitches and asks first (R6): giving
// the item is the same as choosing the offer from the menu (R8).
func TestGiveResolve_OfferWantingTheItemShowsTheCard(t *testing.T) {
	user, mob, done := fixtureWith(t, itemOfferFixtureYAML)
	defer done()
	itm := giveItem(t, user, 10001)

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "Can I ride the ferry?")
	require.Contains(t, theirs, "Leave your sword with me as surety.")
	require.Contains(t, mine, "Accept")
	require.NotContains(t, mine, "You give the")
	require.True(t, user.HasOpenMenu(), "the card is open")
	require.Equal(t, 1, countBackpack(user, 10001), "nothing is taken before Accept")
	require.Empty(t, questTokensFor(1), "nothing is granted before Accept")

	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Contains(t, drained(1), `You give the <ansi fg="item">Iron Sword</ansi>`)
	require.Equal(t, 0, countBackpack(user, 10001))
	require.Equal(t, []string{"5-start"}, questTokensFor(1))
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

// The give command resolves topics before any transfer (#375 R8, Task 19):
// a hand-in plays as the topic and the item never reaches the mob.
func TestGiveCommand_HandInGoesThroughTheTopic(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001))) // seeded "Iron Sword"
	_, err := Give("iron sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Contains(t, drained(2), "My thanks.")
	require.Equal(t, 0, countBackpack(user, 10001), "the sword was handed in")
	require.False(t, mobHolds(mob, 10001), "a topic hand-in never transfers to the mob")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// A hand-in takes the instance the player named (#375 review M3): with two
// swords, `give 2.sword` hands over the second, named in the handover line,
// and the first stays in the pack.
func TestGiveCommand_HandInTakesTheNamedInstance(t *testing.T) {
	user, _, done := offerFixture(t)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001)))
	second := items.New(10001)
	second.Rename("notched sword")
	require.True(t, user.Character.StoreItem(second))
	var firstUUID string
	for _, itm := range user.Character.GetAllBackpackItems() {
		if itm.ItemId == 10001 && itm.UUID != second.UUID {
			firstUUID = itm.UUID.String()
		}
	}
	require.NotEmpty(t, firstUUID)

	_, err := Give("2.sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.Contains(t, mine, `You give the <ansi fg="item">Notched Sword</ansi>`)
	require.Contains(t, mine, "My thanks.")
	left := user.Character.GetAllBackpackItems()
	require.Len(t, left, 1)
	require.Equal(t, firstUUID, left[0].UUID.String(), "the first sword was taken in place of the named one")
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
}

// The same through an offer's card: Accept takes the instance the give named.
func TestGiveResolve_OfferCardTakesTheNamedInstance(t *testing.T) {
	user, mob, done := fixtureWith(t, itemOfferFixtureYAML)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001)))
	second := items.New(10001)
	second.Rename("notched sword")
	require.True(t, user.Character.StoreItem(second))
	var named items.Item
	for _, itm := range user.Character.GetAllBackpackItems() {
		if itm.UUID == second.UUID {
			named = itm
		}
	}
	require.Equal(t, second.UUID, named.UUID)

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, named))
	drained(1)
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Contains(t, drained(1), `You give the <ansi fg="item">Notched Sword</ansi>`)
	left := user.Character.GetAllBackpackItems()
	require.Len(t, left, 1)
	require.NotEqual(t, second.UUID, left[0].UUID)
}

// loadSkeletonTree installs a behavior tree for the Skeleton (mob 1) for the
// rest of the test.
func loadSkeletonTree(t *testing.T, tree string) {
	t.Helper()
	node, err := behaviortree.LoadTreeFromBytes([]byte(tree))
	require.NoError(t, err)
	t.Cleanup(behaviortree.GetEngine().SetMobTreeForTest(1, node))
}

// The purse topic is used up (9-end held), so the sword is wanted but no
// topic can take it. The tree's authored refusal plays in place of the
// generic hand-back line (#375 review M1: Arn's "See Drunn first").
const handBackRefusalTree = `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: condition
          check: item_matches
          item_id: 10001
        - type: condition
          check: player_has_quest
          quest: "9-end"
        - type: action
          do: send_user_text
          text: "You already brought me that."
        - type: action
          do: return_item
`

func TestGiveCommand_WantedButIneligiblePlaysTheTreesRefusal(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	loadSkeletonTree(t, handBackRefusalTree)
	user.Character.QuestProgress = map[int]string{9: "end"}
	require.True(t, user.Character.StoreItem(items.New(10001)))

	_, err := Give("iron sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.Contains(t, mine, "You already brought me that.")
	require.Contains(t, mine, "Skeleton hands back the Iron Sword.")
	require.NotContains(t, mine, "looks at the", "a second, generic hand-back line")
	require.NotContains(t, mine, "You give the", "the item never moved")
	require.Equal(t, 1, countBackpack(user, 10001))
	require.False(t, mobHolds(mob, 10001))
	require.Empty(t, questTokensFor(1))
}

// A hand-back credits nothing: a give branch that would pay, grant or take
// is skipped whole, before any of its speech, and the generic line plays.
func TestGiveCommand_HandBackTreeCannotCredit(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	loadSkeletonTree(t, `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: condition
          check: item_matches
          item_id: 10001
        - type: action
          do: send_user_text
          text: "Have a reward."
        - type: action
          do: give_gold
          amount: 50
        - type: action
          do: grant_quest
          quest: "8-start"
        - type: action
          do: return_item
`)
	user.Character.QuestProgress = map[int]string{9: "end"}
	user.Character.Gold = 0
	require.True(t, user.Character.StoreItem(items.New(10001)))

	_, err := Give("iron sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.NotContains(t, mine, "Have a reward.", "a crediting branch spoke")
	require.Contains(t, mine, "looks at the")
	require.Equal(t, 0, user.Character.Gold)
	require.False(t, user.Character.HasQuest("8-start"))
	require.Empty(t, questTokensFor(1))
	require.Equal(t, 1, countBackpack(user, 10001))
	require.False(t, mobHolds(mob, 10001))
}

// A catch-all give branch (no item_matches) is for ordinary gifts: it does
// not answer an item a topic wants (Sylara's "I have no use for this.").
func TestGiveCommand_HandBackSkipsACatchAllBranch(t *testing.T) {
	user, _, done := offerFixture(t)
	defer done()
	loadSkeletonTree(t, `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: action
          do: send_user_text
          text: "I have no use for this."
        - type: action
          do: return_item
`)
	user.Character.QuestProgress = map[int]string{9: "end"}
	require.True(t, user.Character.StoreItem(items.New(10001)))

	_, err := Give("iron sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.NotContains(t, mine, "I have no use for this.")
	require.Contains(t, mine, "looks at the")
	require.Equal(t, 1, countBackpack(user, 10001))
}

// A tree that only speaks still leaves the player told the item came back.
func TestGiveCommand_HandBackTreeWithoutReturnStillSaysItCameBack(t *testing.T) {
	user, _, done := offerFixture(t)
	defer done()
	loadSkeletonTree(t, `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: condition
          check: item_matches
          item_id: 10001
        - type: action
          do: send_user_text
          text: "The temple accepts coin."
`)
	user.Character.QuestProgress = map[int]string{9: "end"}
	require.True(t, user.Character.StoreItem(items.New(10001)))

	_, err := Give("iron sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine := drained(1)
	require.Contains(t, mine, "The temple accepts coin.")
	require.Contains(t, mine, "looks at the")
	require.Equal(t, 1, countBackpack(user, 10001))
}

// A wanted item given while no topic wanting it is eligible is handed back
// by the give command and never leaves the player (R8).
func TestGiveCommand_WantedButIneligibleIsHandedBack(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	user.Character.QuestProgress = map[int]string{9: "end"}
	require.True(t, user.Character.StoreItem(items.New(10001)))
	_, err := Give("iron sword skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Contains(t, drained(1), "hands it back")
	require.Equal(t, 1, countBackpack(user, 10001))
	require.False(t, mobHolds(mob, 10001))
}
