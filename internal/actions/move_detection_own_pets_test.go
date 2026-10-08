package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/require"
)

// Playtest finding (movement parity 4b): a hiding or sneaking PLAYER was
// exposed by their own charmed mobs and AI companions. A companion is moved
// into the new room before its owner's arrival roll and rolled against them
// there, and the sneak command rolled the owner's pets as observers ("You try
// to blend into the shadows but Rocky the flesh golem notices you."). A
// player's own charmed mobs (GetCharmIds, which every companion path also
// tracks) are now its allies for both rolls, as a mob mover's owner side
// already was. Another player's pet still rolls.

// ownPet places a sharp charmed mob in w.dest owned by ownerId.
func ownPet(t *testing.T, w *detectWorld, owner *characters.Character, ownerId, petId int) *characters.Character {
	t.Helper()
	_, pet := w.place(t, "mob", petId, "Rocky")
	pet.Charm(ownerId, -1, ``)
	if owner != nil {
		owner.TrackCharmed(petId, true)
	}
	pet.Stats.Perception.ValueAdj = 1000
	return pet
}

// A clumsy player sneaking into a room holding only their own sharp pet stays
// hidden and still sneaking.
func TestEntryDetection_OwnPetDoesNotSpotSneakingPlayer(t *testing.T) {
	w := newDetectWorld(t)
	mover, mc := w.place(t, "player", 9810, "Owner")
	ownPet(t, w, mc, 9810, 9850)
	mc.Stats.Dexterity.ValueAdj = 0
	hideForMove(t, mc)
	mc.SetMiscData(`sneaking`, true)

	got := EntryDetection(mover, w.dest, true)

	require.True(t, got.StillSneaking, "your own pet never gives you away")
	require.True(t, mc.IsHidden())
}

// Another player's sharp pet still spots a clumsy sneaking player.
func TestEntryDetection_OtherPlayersPetStillSpotsSneaker(t *testing.T) {
	w := newDetectWorld(t)
	mover, mc := w.place(t, "player", 9810, "Sneak")
	ownPet(t, w, nil, 9811, 9850) // owner 9811 is elsewhere
	mc.Stats.Dexterity.ValueAdj = 0
	hideForMove(t, mc)
	mc.SetMiscData(`sneaking`, true)

	got := EntryDetection(mover, w.dest, true)

	require.False(t, got.StillSneaking, "someone else's pet still notices you")
	require.False(t, mc.IsHidden())
}

// A clumsy player hiding beside their own sharp pet is not noticed by it.
func TestSneak_OwnPetDoesNotNoticePlayer(t *testing.T) {
	w := newDetectWorld(t)
	actor, mc := w.place(t, "player", 9810, "Owner")
	ownPet(t, w, mc, 9810, 9850)
	mc.Stats.Dexterity.ValueAdj = 0
	mc.Stamina = 100
	mc.StaminaMax.Value = 100

	got := Sneak(actor)

	require.Nil(t, got.SpottedBy, "your own pet never notices you")
	require.True(t, got.Success)
	require.True(t, mc.IsHidden())
}

// A clumsy player hiding beside another player's sharp pet is noticed by it.
func TestSneak_OtherPlayersPetStillNoticesPlayer(t *testing.T) {
	w := newDetectWorld(t)
	actor, mc := w.place(t, "player", 9810, "Sneak")
	ownPet(t, w, nil, 9811, 9850) // owner 9811 is elsewhere
	mc.Stats.Dexterity.ValueAdj = 0
	mc.Stamina = 100
	mc.StaminaMax.Value = 100

	got := Sneak(actor)

	require.False(t, got.Success, "someone else's pet still notices you")
	require.NotNil(t, got.SpottedBy)
	require.Equal(t, "Rocky", got.SpottedBy.Name)
}
