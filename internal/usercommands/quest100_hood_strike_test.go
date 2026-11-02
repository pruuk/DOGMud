package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Quest 100's hint tells a student: hood, west, unhood, attack brute. This
// walks the same sequence in a sealed cave, from hiding, so every claim the
// hint rests on is the sight code's and not the author's:
//
//   - a hood closed while hidden keeps the student hidden,
//   - a hooded student sees nothing and cannot pick a target,
//   - that refusal costs nothing, the student stays hidden,
//   - opening the hood keeps the student hidden and gives full sight,
//   - and the attack that follows is an opening strike from hiding.
func TestQuest100_HoodedStudentOpensTheHoodAndStrikesFromHiding(t *testing.T) {
	user, room := hoodFixture(t)
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", SkyLight: rooms.SkyLightPtr(0.0), Indoor: true, MovementCost: 1},
	}))
	room.Biome = "cave"
	// The target stands here, so the refusal below is the dark's alone.
	room.AddMob(100)
	skeleton := mobs.GetInstance(100)
	require.NotNil(t, skeleton)
	skeleton.Character.RoomId = room.RoomId
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	wireRevealCascadeForTest(user.Character)
	hideForAmbushTest(t, user.Character)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.IsHidden(), "closing the hood must not reveal the student")
	require.False(t, user.Character.EmitsLight(), "a closed hood sheds no light")
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room),
		"a hooded student in a sealed cave sees nothing")

	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.False(t, user.Character.IsInCombat(), "a hooded student cannot pick a target")
	require.True(t, user.Character.IsHidden(), "the refused attack must cost no cover")
	refused := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.NotEmpty(t, refused, "the refusal must be spoken")

	_, err = Unhood("", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.IsHidden(), "opening the hood must not reveal the student")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(user.Character, room),
		"the lantern's full strength gives clear sight, so the name resolves")

	handled, err = Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.True(t, user.Character.IsInCombat(), "the attack must start the fight")
	require.True(t, user.Character.CombatPhase.OpeningUnspent(),
		"the attack must be an opening strike from hiding, the quest's ambush event")
}
