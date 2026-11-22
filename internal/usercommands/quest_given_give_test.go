package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940: give. Quest gear given to another player fades. Given to an NPC that
// did not ask for it, or to a companion that is not the giver's own (another
// player's charmed mob or pet), it is considered and handed back; so it is by
// the giver's own pet when the pet has no room. Only the giver's own charmed
// mob or own pet keeps it. The fixtures are in quest_given_ways_out_test.go.

func TestQuestGear_GiveToPlayer(t *testing.T) {
	t.Run("quest gear fades", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		bob := users.GetByUserId(2)
		bob.Character.Items = nil
		_, err := Give("iron sword bobrick", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, fadeLine)
		assert.NotContains(t, out, "You give")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(bob.Character.Items), "the other player gets nothing")
	})
	t.Run("bought copy moves", func(t *testing.T) {
		user, room := questGearHolder(t, boughtCopy())
		bob := users.GetByUserId(2)
		bob.Character.Items = nil
		_, err := Give("iron sword bobrick", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), "You give the")
		assert.Equal(t, 1, carriesSword(bob.Character.Items))
	})
}

// skeletonIgnoresGives pins the Skeleton's tree to one with no player_give
// branch, so the give falls through to the ordinary path whatever tree file
// the binary's working directory happens to reach.
const skeletonIgnoresGives = `tree:
  type: selector
  children:
    - type: sequence
      event: mob_idle
      children:
        - type: action
          do: send_user_text
          text: "idle"
`

// skeletonKeepsGifts is a player_give handler that keeps whatever it is
// given: the tree owns the give, as it does today.
const skeletonKeepsGifts = `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: action
          do: send_user_text
          text: "The skeleton clutches it."
`

// skeletonReturnsGifts is a player_give handler that hands every gift back
// with return_item, as the shopkeeper, questgiver and guard captain
// archetypes do.
const skeletonReturnsGifts = `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: action
          do: return_item
`

// giftsAccepted drains the queued GiftAccepted events, which feed the
// gift-to-opinion seeder.
func giftsAccepted() int { return len(events.DrainQueuedByTypeForTest("GiftAccepted")) }

// skeletonForGive is the seeded Skeleton with an empty pack, no player_give
// branch and a clean queue.
func skeletonForGive(t *testing.T, room *rooms.Room) *mobs.Mob {
	t.Helper()
	_, id := room.FindByName("skeleton")
	m := mobs.GetInstance(id)
	require.NotNil(t, m)
	m.Character.Items = nil
	m.Character.Stats.Strength.ValueAdj = 100
	loadSkeletonTree(t, skeletonIgnoresGives)
	events.DrainQueuedInputsForTest(m.InstanceId)
	giftsAccepted()
	events.DrainQueuedByTypeForTest("GiftOffered")
	events.DrainQueuedItemOwnershipForTest(1)
	return m
}

// An NPC that did not ask for quest gear considers it and hands it back; it
// never keeps it, the gear does not fade, and nothing is raised for a gift
// that came back. A player_give tree handler behaves as it does for any item.
func TestQuestGear_GiveToNPC(t *testing.T) {
	t.Run("quest gear comes back", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeletonForGive(t, room)
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, "considers the")
		assert.Contains(t, out, "hands back the")
		assert.NotContains(t, out, "fades away")
		require.Equal(t, 1, carriesSword(user.Character.Items), "the giver keeps it")
		assert.True(t, user.Character.Items[0].IsQuestGiven(), "still quest gear")
		assert.Equal(t, 0, carriesSword(m.Character.Items), "the NPC never keeps it")
		assert.Empty(t, events.DrainQueuedInputsForTest(m.InstanceId), "no gearup queued")
		assert.Equal(t, 0, giftsAccepted(), "no opinion for a gift handed back")
		assert.Empty(t, events.DrainQueuedItemOwnershipForTest(user.UserId), "no ownership or item_gain noise")
		theirs := sentTo(users.GetByUserId(2))
		assert.Contains(t, theirs, "considers the")
		assert.Contains(t, theirs, "Iron Sword</ansi> back.", "the room sees it come back")
	})
	t.Run("bought copy stays with the NPC", func(t *testing.T) {
		user, room := questGearHolder(t, boughtCopy())
		m := skeletonForGive(t, room)
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), "hands back")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 1, carriesSword(m.Character.Items))
		inputs := strings.Join(events.DrainQueuedInputsForTest(m.InstanceId), "\n")
		assert.Contains(t, inputs, "considers the")
		assert.Contains(t, inputs, "gearup")
		assert.Equal(t, 1, giftsAccepted())
	})
	t.Run("a player_give handler takes quest gear as today", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeletonForGive(t, room)
		loadSkeletonTree(t, skeletonKeepsGifts)
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, "The skeleton clutches it.")
		assert.NotContains(t, out, "hands back")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 1, carriesSword(m.Character.Items))
		assert.Equal(t, 1, giftsAccepted(), "an ordinary give's events")
	})
	t.Run("a tree that returns it raises no gift", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeletonForGive(t, room)
		loadSkeletonTree(t, skeletonReturnsGifts)
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), "hands back the")
		require.Equal(t, 1, carriesSword(user.Character.Items), "the tree handed it back")
		assert.Equal(t, 0, carriesSword(m.Character.Items))
		assert.Equal(t, 0, giftsAccepted(), "no opinion for a gift handed back")
		assert.Empty(t, events.DrainQueuedByTypeForTest("GiftOffered"))
		for _, e := range events.DrainQueuedItemOwnershipForTest(user.UserId) {
			assert.True(t, e.Gained && e.Reason == "tree_give", "only the return's own gain, got %+v", e)
		}
	})
	t.Run("the giver's own companion takes quest gear", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeletonForGive(t, room)
		m.Character.Charm(user.UserId, 100, ``)
		t.Cleanup(func() { m.Character.RemoveCharm() })
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), "hands back")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		require.Equal(t, 1, carriesSword(m.Character.Items))
		assert.True(t, m.Character.Items[0].IsQuestGiven(), "the mark rides with the copy")
	})
	t.Run("another player's companion hands it back", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeletonForGive(t, room)
		m.Character.Charm(2, 100, ``)
		t.Cleanup(func() { m.Character.RemoveCharm() })
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), "hands back the")
		assert.Equal(t, 1, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(m.Character.Items))
	})
}

func TestQuestGear_GiveToPet(t *testing.T) {
	dog := func(name string) pets.Pet { return pets.Pet{Name: name, Type: "dog", Capacity: 2} }
	t.Run("own pet is a companion and takes it", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		user.Character.Pet = dog("Rex")
		_, err := Give("iron sword pet", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), "hands back")
		assert.Equal(t, 1, carriesSword(user.Character.Pet.Items))
	})
	t.Run("own pet with no room hands it back", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		user.Character.Pet = dog("Rex")
		user.Character.Pet.Capacity = 0
		_, err := Give("iron sword pet", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, "hands back the")
		assert.NotContains(t, out, "fades away")
		assert.Equal(t, 1, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(room.Items))
	})
	t.Run("another player's pet hands it back", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		bob := users.GetByUserId(2)
		bob.Character.Pet = dog("Fido")
		_, err := Give("iron sword fido", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, "considers the")
		assert.Contains(t, out, "hands back the")
		assert.Equal(t, 1, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(bob.Character.Pet.Items))
		assert.Contains(t, sentTo(bob), "Iron Sword</ansi> back.", "the room sees it come back")
	})
	t.Run("another player's pet: a bought copy moves", func(t *testing.T) {
		user, room := questGearHolder(t, boughtCopy())
		bob := users.GetByUserId(2)
		bob.Character.Pet = dog("Fido")
		_, err := Give("iron sword fido", user, room, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, carriesSword(bob.Character.Pet.Items))
	})
}
