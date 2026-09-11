package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const castSightInfraredBuffId = 7801

// castSightActor is a PLAYER caster: stubActor with a user id, a name and a
// record of what it was told.
type castSightActor struct {
	*stubActor
	userId int
	name   string
	sent   []string
}

func (a *castSightActor) IsPlayer() bool  { return true }
func (a *castSightActor) GetUserId() int  { return a.userId }
func (a *castSightActor) GetName() string { return a.name }
func (a *castSightActor) SendText(_ messaging.Category, msg string) {
	a.sent = append(a.sent, msg)
}

func (a *castSightActor) told(substr string) bool {
	for _, s := range a.sent {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// castSightScene stands Caster (7811), Witness (7812) and Other (7813) in one
// room of the given biome, and seeds a help and a harm spell.
func castSightScene(t *testing.T, biome string) (*castSightActor, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		castSightInfraredBuffId: {BuffId: castSightInfraredBuffId, Name: "Test Infrared", Flags: []buffs.Flag{buffs.InfraredVision}},
	}))
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"sight-heal": {SpellId: "sight-heal", Name: "Sight Heal", Type: spells.HelpSingle, BaseFolds: 2, Cost: 5},
		"sight-bolt": {SpellId: "sight-bolt", Name: "Sight Bolt", Type: spells.HarmSingle, BaseFolds: 2, Cost: 5},
	}))
	caster := users.NewTestUser(7811, "caster", "Caster", 97811)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7811: caster,
		7812: users.NewTestUser(7812, "witness", "Witness", 97812),
		7813: users.NewTestUser(7813, "other", "Other", 97813),
	}))
	room := &rooms.Room{RoomId: 7810, Biome: biome}
	room.AddPlayer(7811)
	room.AddPlayer(7812)
	room.AddPlayer(7813)
	actor := &castSightActor{stubActor: newStubActor(caster.Character, room), userId: 7811, name: "Caster"}
	return actor, room
}

func giveCasterInfrared(t *testing.T, a *castSightActor) {
	t.Helper()
	require.NoError(t, a.GetCharacter().AddBuff(castSightInfraredBuffId, true))
}

func requireRefused(t *testing.T, a *castSightActor, r CastResult, line string) {
	t.Helper()
	assert.False(t, r.Initiated)
	assert.True(t, r.NoTarget)
	assert.True(t, r.RefusalExplained, "the refusal is narrated, so the generic line must not follow")
	assert.True(t, a.told(line), "caster was told %q, want a line containing %q", a.sent, line)
	assert.True(t, a.GetCharacter().CooldownReady("special-move"), "a refused cast spends nothing")
}

func TestCastSight_ClearSightNamesAndShapes(t *testing.T) {
	a, _ := castSightScene(t, "city")
	r := InitiateCast(a, "sight-heal", "witness")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7812}, r.TargetUserIds)

	a2, _ := castSightScene(t, "city")
	r = InitiateCast(a2, "sight-heal", "2.shape")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7813}, r.TargetUserIds, "figures are the other players in room order")
}

func TestCastSight_ShapesOnlyRefusesANameWithAHint(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	requireRefused(t, a, InitiateCast(a, "sight-heal", "witness"), "You can only make out shapes here.")
	assert.True(t, a.told("cast sight-heal shape"))
}

func TestCastSight_ShapesOnlyAimsAtAShape(t *testing.T) {
	for _, name := range []string{"shape", "2.shape", "shape#2"} {
		a, _ := castSightScene(t, "cave")
		giveCasterInfrared(t, a)
		r := InitiateCast(a, "sight-heal", name)
		require.True(t, r.Initiated, "%q should initiate", name)
		want := 7812
		if name != "shape" {
			want = 7813
		}
		assert.Equal(t, []int{want}, r.TargetUserIds, "%q", name)
	}
}

func TestCastSight_NoSightRefusesEveryTargetedCast(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-heal", "witness"), "You don't see them here.")

	a, _ = castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-heal", "shape"), "You can't see anything to aim at.")

	a, _ = castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-bolt", ""), "You can't see anything to aim at.")
}

func TestCastSight_SelfCastNeedsNoSight(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	r := InitiateCast(a, "sight-heal", "")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7811}, r.TargetUserIds)
}

func TestCastSight_AHiddenCreatureIsNotAFigure(t *testing.T) {
	a, _ := castSightScene(t, "city")
	viewerTestHide(t, users.GetByUserId(7812).Character)
	r := InitiateCast(a, "sight-heal", "shape")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7813}, r.TargetUserIds, "the hidden witness is skipped")
}

func TestCastSight_MobCasterIsUnaffected(t *testing.T) {
	a, room := castSightScene(t, "cave")
	mobCaster := newStubActor(a.GetCharacter(), room) // IsPlayer false
	r := InitiateCast(mobCaster, "sight-heal", "witness")
	require.True(t, r.Initiated, "mobs perceiving darkness is slice F")
}
