package rooms

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/exit"
)

func deckNames(l KnownExitList) []string {
	out := []string{}
	for _, ke := range l.Exits {
		out = append(out, ke.Name)
	}
	return out
}

// freshInstance swaps in an empty instance registry holding one instance
// whose entry room is entry, authorizing users.
func freshInstance(t *testing.T, entry int, users ...int) {
	t.Helper()
	orig := instanceRegistry
	t.Cleanup(func() { instanceRegistry = orig })
	instanceRegistry = NewInstanceRegistry()
	instanceRegistry.Add(&ZoneInstance{InstanceId: entry, EntryRoomId: entry, AuthorizedUsers: users})
}

func TestMayEnterInstance(t *testing.T) {
	entry := ephemeralRoomIdMinimum + 4343
	freshInstance(t, entry, 42)
	require.True(t, MayEnterInstance(99, 1), "a room outside any instance")
	require.True(t, MayEnterInstance(42, entry), "a player the instance authorized")
	require.False(t, MayEnterInstance(99, entry), "a stranger")
	require.True(t, MayEnterInstance(99, entry+1), "an ephemeral room no instance holds, as MoveToRoom allowed")
}

// Owner ruling 18: the deck labels a player's instance portal with its zone
// and offers it only to a player who may enter it; known itself is left
// whole, since the exits line lists every exit.
func TestForDeck_LabelsAnInstancePortalByItsZoneAndOffersOnlyEnterablePortals(t *testing.T) {
	entry := ephemeralRoomIdMinimum + 4242
	freshInstance(t, entry, 42, 43)
	known := KnownExitList{Exits: []KnownExit{
		{Name: "west", Kind: ExitCompass, RoomId: 1},
		{Name: "shrine", Kind: ExitNamed, RoomId: 2},
		{Name: "arena-42", Kind: ExitTemporary, RoomId: entry, Temp: exit.TemporaryRoomExit{RoomId: entry, UserId: 42}},
	}}

	deck, labels := ForDeck(known, 42)
	require.Equal(t, []string{"west", "shrine", "arena-42"}, deckNames(deck), "the opener")
	require.Equal(t, "arena", labels["arena-42"])
	require.NotContains(t, labels, "shrine", "an exit shown by its own name has no label")

	deck, _ = ForDeck(known, 43)
	require.Contains(t, deckNames(deck), "arena-42", "a party member the instance authorized")

	deck, labels = ForDeck(known, 99)
	require.NotContains(t, deckNames(deck), "arena-42", "a stranger: the portal would push them back")
	require.NotContains(t, labels, "arena-42")
	require.Len(t, known.Exits, 3, "known is left whole")
}

// R2: a party member's portal to the same zone gets a button beside the
// player's own; both read the zone; only the player's own is theirs.
func TestForDeck_EveryPortalAPlayerMayEnterGetsAButton(t *testing.T) {
	entry42, entry43 := ephemeralRoomIdMinimum+4242, ephemeralRoomIdMinimum+4343
	orig := instanceRegistry
	t.Cleanup(func() { instanceRegistry = orig })
	instanceRegistry = NewInstanceRegistry()
	instanceRegistry.Add(&ZoneInstance{InstanceId: entry42, EntryRoomId: entry42, AuthorizedUsers: []int{42, 43}})
	instanceRegistry.Add(&ZoneInstance{InstanceId: entry43, EntryRoomId: entry43, AuthorizedUsers: []int{43}})
	known := KnownExitList{Exits: []KnownExit{
		{Name: "arena-42", Kind: ExitTemporary, RoomId: entry42, Temp: exit.TemporaryRoomExit{RoomId: entry42, UserId: 42}},
		{Name: "arena-43", Kind: ExitTemporary, RoomId: entry43, Temp: exit.TemporaryRoomExit{RoomId: entry43, UserId: 43}},
	}}
	deck, labels := ForDeck(known, 43)
	require.Equal(t, []string{"arena-42", "arena-43"}, deckNames(deck), "43 may enter both")
	require.Equal(t, "arena", labels["arena-42"])
	require.Equal(t, "arena", labels["arena-43"])
	require.False(t, IsOwnPortal(known.Exits[0], 43))
	require.True(t, IsOwnPortal(known.Exits[1], 43))

	deck, _ = ForDeck(known, 42)
	require.Equal(t, []string{"arena-42"}, deckNames(deck), "42 was not authorized for 43's instance")
	require.False(t, IsOwnPortal(KnownExit{Name: "north", Kind: ExitCompass}, 42), "a room exit is nobody's portal")
}

func TestForDeck_ARoomWithNoPortalIsReturnedAsItIs(t *testing.T) {
	known := KnownExitList{Exits: []KnownExit{
		{Name: "north", Kind: ExitCompass, RoomId: 1},
		{Name: "plank", Kind: ExitTemporary, RoomId: 2, Temp: exit.TemporaryRoomExit{RoomId: 2}},
	}}
	deck, labels := ForDeck(known, 42)
	require.Equal(t, []string{"north", "plank"}, deckNames(deck))
	require.Nil(t, labels)
}
