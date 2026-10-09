package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Slice F's playtest read "You recoil from striking Stone Beetle Queen (dead)!"
// in an unlit cave while every other line said "something". The three recoil
// lines were direct SendText calls that never reached the seam.

func TestRecoil_AttackerInTheDarkIsNotToldTheDefendersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(1)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	emitReturnDamageText(atk, def, 5)

	attacker := drainPlain(1)
	assert.Equal(t, 1, countContaining(attacker, "You recoil from striking something!"))
	assert.Equal(t, 0, countContaining(attacker, "Skeleton"))
}

func TestRecoil_DefenderInTheDarkIsNotToldTheAttackersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(1)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	emitReturnDamageText(atk, def, 5)

	defender := drainPlain(1)
	assert.Equal(t, 1, countContaining(defender, "Something recoils from striking you!"))
	assert.Equal(t, 0, countContaining(defender, "Skeleton"))
}

func TestRecoil_RoomLineIsVisualAndHidesNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	emitReturnDamageText(atk, def, 5)
	assert.Equal(t, 0, countContaining(drainPlain(2), "recoils"),
		"an observer who cannot see must not be told about the recoil at all")

	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(heatEyesConditionId, true))
	emitReturnDamageText(atk, def, 5)
	observer := drainPlain(2)
	assert.Equal(t, 1, countContaining(observer, "A figure recoils from striking a figure!"))
	assert.Equal(t, 0, countContaining(observer, "Aliceia"))
	assert.Equal(t, 0, countContaining(observer, "Skeleton"))
}

// #454 review F9 (H1 leftover): mobDisplayName printed the mob's adjective
// span, so a lit recoil line read "You recoil from striking Skeleton
// (charmed)!". Narration names carry no state tags (#453); neither do the
// spell lines, which name a mob through the same helper.
func TestRecoil_LitLineCarriesNoAdjectiveSpan(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	mob := mobs.GetInstance(100)
	mob.Character.SetAdjective(`charmed`, true)
	drainPlain(1)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mob, room)
	emitReturnDamageText(atk, def, 5)

	attacker := drainPlain(1)
	assert.Equal(t, 1, countContaining(attacker, "You recoil from striking Skeleton!"), "%q", attacker)
	assert.Equal(t, 0, countContaining(attacker, "charmed"))

	spellName := spellEffectName(def, room, 1)
	assert.Contains(t, spellName, "Skeleton")
	assert.NotContains(t, spellName, "charmed", "a spell line names a mob with its state tag")
}

// The spell defence identity of a player is a narration name too: no
// adjective span and no " and <pet>" ("Aliceia and Fang dodges").
func TestSpellDefenceIdentity_PlayerCarriesNoPetOrAdjectives(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	u := users.GetByUserId(1)
	u.Character.Pet = pets.Pet{Name: "Fang", Type: "dog"}
	u.Character.SetAdjective(`charmed`, true)
	got := spellDefenceIdentity(u.Character, u, room)
	assert.Contains(t, got, "Aliceia")
	assert.NotContains(t, got, "Fang", "a narrated spell line names the caster's pet")
	assert.NotContains(t, got, "charmed")
}

func TestRecoil_LitRoomNamesBothAndSparesTheParticipantsTheRoomLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	room.Lamp = rooms.LampPtr(90)
	drainPlain(1)
	drainPlain(2)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	emitReturnDamageText(atk, def, 5)

	attacker := drainPlain(1)
	assert.Equal(t, 1, countContaining(attacker, "You recoil from striking Skeleton!"))
	assert.Equal(t, 0, countContaining(attacker, "Aliceia recoils"), "the room line must not reach the attacker")
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia recoils from striking Skeleton!"))
}
