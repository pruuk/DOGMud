package hooks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
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
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	assert.Equal(t, 0, countContaining(drainPlain(2), "SWEEP"),
		"an observer who cannot see must not be told about the sweep at all")

	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(heatEyesConditionId, true))
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
	// Pins room 1 fully lit regardless of the ambient test round (see
	// combat_blind_warning_test.go).
	room.Lamp = rooms.LampPtr(90)
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

// #263: a riposte answers the parry the swing's lines describe, so it reads
// below them. It printed above ("RIPOSTE!" before "Bandit Scout awkwardly
// intercepts your Sling!"): the crit trio went out before the swing's own
// lines.
func TestCritDispatch_RiposteReadsAfterTheSwingItAnswers(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	roundTallies = newCombatTallies()
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	drainPlain(1)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	res := vbLandingResult()
	res.ParryCritDetected = true
	res.MessagesToTarget = []combat.TaggedMessage{{Category: messaging.CategoryDefault, Text: "SWINGMARK you parry the blow."}}
	dispatchCritAndMessaging(atk, def, res)

	lines := drainPlain(1)
	swing, riposte := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "SWINGMARK") && swing < 0 {
			swing = i
		}
		if strings.Contains(l, "RIPOSTE") && riposte < 0 {
			riposte = i
		}
	}
	require.GreaterOrEqual(t, swing, 0, "precondition: the swing line reached the defender: %q", lines)
	require.GreaterOrEqual(t, riposte, 0, "precondition: the riposte reached the defender: %q", lines)
	assert.Less(t, swing, riposte, "the riposte must read after the swing it answers: %q", lines)
}

// #607: the riposted swinger's line says the counter-strike hits them. With
// companions beside them, "turns the parry into a counter-strike" left them
// to guess whom it struck; the room line already names its target.
func TestApplyCritEffects_RiposteLineSaysItHitsTheSwinger(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	defender := users.GetByUserId(1)
	attacker := users.GetByUserId(2)

	crit := applyCritEffects(attacker.Character, defender.Character,
		combat.AttackResult{ParryCritDetected: true}, room)
	require.True(t, crit.Riposte, "precondition: a parry crit ripostes")
	assert.Contains(t, crit.AttackerMsg, "against you")
	assert.Contains(t, crit.RoomMsg, "against "+attacker.Character.Name)
}
