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

// #940: give. Quest gear given to another player (or another player's pet)
// fades; given to an NPC that did not ask for it, it comes back; a companion
// (a mob the giver charmed, or the giver's own pet) takes it. The fixtures are
// in quest_given_ways_out_test.go.

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

// An NPC that did not ask for quest gear considers it and hands it back; it
// never keeps it and the gear does not fade. A companion takes it, and a
// player_give tree handler behaves as it does for any item.
func TestQuestGear_GiveToNPC(t *testing.T) {
	skeleton := func(t *testing.T, room *rooms.Room) *mobs.Mob {
		_, id := room.FindByName("skeleton")
		m := mobs.GetInstance(id)
		require.NotNil(t, m)
		m.Character.Items = nil
		m.Character.Stats.Strength.ValueAdj = 100
		loadSkeletonTree(t, skeletonIgnoresGives)
		events.DrainQueuedInputsForTest(m.InstanceId)
		return m
	}
	t.Run("a player_give handler takes quest gear as today", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeleton(t, room)
		loadSkeletonTree(t, skeletonKeepsGifts)
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, "The skeleton clutches it.")
		assert.NotContains(t, out, "hands back")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 1, carriesSword(m.Character.Items))
	})
	t.Run("quest gear comes back", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeleton(t, room)
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
	})
	t.Run("bought copy stays with the NPC", func(t *testing.T) {
		user, room := questGearHolder(t, boughtCopy())
		m := skeleton(t, room)
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), "hands back")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 1, carriesSword(m.Character.Items))
		inputs := strings.Join(events.DrainQueuedInputsForTest(m.InstanceId), "\n")
		assert.Contains(t, inputs, "considers the")
		assert.Contains(t, inputs, "gearup")
	})
	t.Run("a companion takes quest gear", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		m := skeleton(t, room)
		m.Character.Charm(user.UserId, 100, ``)
		t.Cleanup(func() { m.Character.RemoveCharm() })
		_, err := Give("iron sword skeleton", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), "hands back")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		require.Equal(t, 1, carriesSword(m.Character.Items))
		assert.True(t, m.Character.Items[0].IsQuestGiven(), "the mark rides with the copy")
	})
}

func TestQuestGear_GiveToPet(t *testing.T) {
	dog := func(name string) pets.Pet { return pets.Pet{Name: name, Type: "dog", Capacity: 2} }
	t.Run("own pet is a companion and takes it", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		user.Character.Pet = dog("Rex")
		_, err := Give("iron sword pet", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), "fades away")
		assert.Equal(t, 1, carriesSword(user.Character.Pet.Items))
	})
	t.Run("own pet with no room: it would land on the ground, so it fades", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		user.Character.Pet = dog("Rex")
		user.Character.Pet.Capacity = 0
		_, err := Give("iron sword pet", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), fadeLine)
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(room.Items))
	})
	t.Run("another player's pet: quest gear fades", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		bob := users.GetByUserId(2)
		bob.Character.Pet = dog("Fido")
		_, err := Give("iron sword fido", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), fadeLine)
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(bob.Character.Pet.Items))
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
