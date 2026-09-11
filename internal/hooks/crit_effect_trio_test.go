package hooks

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sweepPrefix = `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> `

// sweepCrit builds the three SWEEP lines exactly as applyCritEffects words
// them (combat_shared_helpers.go), with defender first and attacker second.
func sweepCrit(defender, attacker string) CritEffectResult {
	return CritEffectResult{
		DefenderMsg: sweepPrefix + `You dodge and sweep their legs out! They crash to the ground!`,
		AttackerMsg: fmt.Sprintf(sweepPrefix+`%s dodges and sweeps your legs! You crash to the ground!`, defender),
		RoomMsg:     fmt.Sprintf(sweepPrefix+`%s dodges and sweeps %s to the ground!`, defender, attacker),
	}
}

func TestSweep_AttackerInTheDarkIsNotToldTheDefendersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	atk := actions.NewUserActorInRoom(users.GetByUserId(2), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Bobrick"))

	attacker := drainPlain(2)
	assert.Equal(t, 1, countContaining(attacker, "SWEEP! Something dodges and sweeps your legs!"))
	assert.Equal(t, 0, countContaining(attacker, "Aliceia"))
}

func TestSweep_RoomLineIsVisual(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	assert.Equal(t, 0, countContaining(drainPlain(2), "SWEEP"),
		"an observer who cannot see must not be told about the sweep at all")

	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	observer := drainPlain(2)
	assert.Equal(t, 1, countContaining(observer, "SWEEP! A figure dodges and sweeps a figure to the ground!"))
	assert.Equal(t, 0, countContaining(observer, "Aliceia"))
	assert.Equal(t, 0, countContaining(observer, "Skeleton"))
}

func TestSweep_LitRoomNamesBoth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia dodges and sweeps Skeleton to the ground!"))
}

// The wiring: dispatchCritAndMessaging must route crit effects through the
// seam. A parry crit fires riposte with no contest (counter_tier_test.go), so
// this is deterministic. Before the fix the room line went out on the audio
// channel and reached the unsighted observer.
func TestCritDispatch_RiposteRoomLineSparesAnObserverInTheDark(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	roundTallies = newCombatTallies()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	res := vbLandingResult()
	res.ParryCritDetected = true
	dispatchCritAndMessaging(atk, def, res)

	assert.Equal(t, 0, countContaining(drainPlain(2), "RIPOSTE"))
}

// The three tests above assert against hand-written copies of the crit lines.
// This one feeds a REAL applyCritEffects result through the seam, so the
// shipped wording is what gets hidden. A parry crit fires riposte with no
// contest, which makes it deterministic.
func TestCritDispatch_RealRiposteTextHidesTheDefendersName(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	defender := users.GetByUserId(1)
	attacker := users.GetByUserId(2)
	drainPlain(1)
	drainPlain(2)

	crit := applyCritEffects(attacker.Character, defender.Character,
		combat.AttackResult{ParryCritDetected: true}, room)
	require.True(t, crit.Riposte, "precondition: a parry crit ripostes")
	require.Contains(t, crit.AttackerMsg, "Aliceia",
		"precondition: the shipped riposte line names the defender")

	sendCritEffectTrio(actions.NewUserActorInRoom(attacker, room),
		actions.NewUserActorInRoom(defender, room), room, crit)

	attackerLines := drainPlain(2)
	assert.Equal(t, 1, countContaining(attackerLines, "RIPOSTE"))
	assert.Equal(t, 0, countContaining(attackerLines, "Aliceia"),
		"the attacker cannot see who riposted")
}
