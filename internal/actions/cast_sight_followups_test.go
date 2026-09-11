package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The blind review behind the slice A cast-sight commit found these. Each is a
// way the caster learned something their sight does not give them, or was
// refused something ruling 4 grants them.

// A protected mob refuses the cast by name. The caster who aimed at a shape
// never saw that name, so the refusal must not hand it over.
func TestCastSight_ProtectedMobRefusalDoesNotNameIt(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	defer seedRoomMob(t, room, 7820, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()

	r := InitiateCast(a, "sight-bolt", "3.shape")

	assert.True(t, r.NoTarget)
	assert.False(t, r.Initiated)
	assert.False(t, a.told("Caravan Guard"), "the refusal named a mob the caster only sees as a shape: %q", a.sent)
	assert.True(t, a.told("a figure"), "want the shapes-only wording, got %q", a.sent)
}

// With clear sight the name is the caster's to hear, so the refusal keeps it.
func TestCastSight_ClearSightRefusalStillNamesTheMob(t *testing.T) {
	a, room := castSightScene(t, "city")
	defer seedRoomMob(t, room, 7820, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()

	r := InitiateCast(a, "sight-bolt", "caravan guard")

	assert.True(t, r.NoTarget)
	assert.True(t, a.told("Caravan Guard"), "got %q", a.sent)
}

// Ruling 4: a self-cast needs no sight. The exemption was an exact,
// case-sensitive match, so in the dark these spellings were refused while the
// identical input self-healed in a lit room.
func TestCastSight_SelfCastByNameNeedsNoSight(t *testing.T) {
	for _, name := range []string{"Caster", "caster", "cast"} {
		a, _ := castSightScene(t, "cave")
		r := InitiateCast(a, "sight-heal", name)
		require.True(t, r.Initiated, "%q should self-cast in the dark", name)
		assert.Equal(t, []int{7811}, r.TargetUserIds, "%q", name)
	}
}

// `all.shape` names no single figure, so castShapeIndex reads 0 for it. Without
// castNamesAShape the refusal called it a typed name.
func TestCastSight_AllShapeIsAShapeNotAName(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-heal", "all.shape"), "You can't see anything to aim at.")
}

// Figures are the players the caster makes out in room order, THEN the mobs.
// The original test put no mob in the room, so swapping the two loops stayed
// green.
func TestCastSight_FiguresArePlayersThenMobs(t *testing.T) {
	a, room := castSightScene(t, "city")
	defer seedRoomMob(t, room, 7820, "Steppe Hare", nil)()

	r := InitiateCast(a, "sight-bolt", "1.shape")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7812}, r.TargetUserIds, "figure 1 is the first other player")
	assert.Empty(t, r.TargetMobInstanceIds)

	a2, room2 := castSightScene(t, "city")
	defer seedRoomMob(t, room2, 7821, "Steppe Hare", nil)()

	r = InitiateCast(a2, "sight-bolt", "3.shape")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7821}, r.TargetMobInstanceIds, "figure 3 is the mob, after both players")
	assert.Empty(t, r.TargetUserIds)
}
