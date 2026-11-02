package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// #375 phase 7: an opening strike from hiding that wins no swing (missed,
// dodged, parried or blocked) fires ambush_foiled on the mob it was aimed
// at, for its tree to answer. A clean opener fires nothing, nor does an
// opener that threw no swing, nor one whose partial damage killed the mob
// (its death line is the answer).
func stubHookMobEvents(t *testing.T) *[]mobs.EventContext {
	t.Helper()
	var fired []mobs.EventContext
	orig := mobs.MobEventTryMobBehavior
	mobs.MobEventTryMobBehavior = func(id int, ctx mobs.EventContext) bool {
		fired = append(fired, ctx)
		return true
	}
	t.Cleanup(func() { mobs.MobEventTryMobBehavior = orig })
	return &fired
}

func TestSurpriseRound_TurnedAsideFiresAmbushFoiled(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	def.GetCharacter().Health = 10
	fired := stubHookMobEvents(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(1, false),
		DefenderWasAttacked: true,
	})

	require.Equal(t, []mobs.EventContext{{EventType: "ambush_foiled", UserId: 1}}, *fired)
}

func TestSurpriseRound_CleanOpenerFiresNoAmbushFoiled(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	def.GetCharacter().Health = 10
	fired := stubHookMobEvents(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(1, true),
		DefenderWasAttacked: true,
	})

	require.Empty(t, *fired)
}

func TestSurpriseRound_NoSwingOrADeadMobFiresNoAmbushFoiled(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	fired := stubHookMobEvents(t)

	def.GetCharacter().Health = 10
	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{WasSurpriseAttack: true})
	require.Empty(t, *fired, "no swing was thrown")

	def.GetCharacter().Health = 0
	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(1, false),
		DefenderWasAttacked: true,
	})
	require.Empty(t, *fired, "the partial damage killed it")
}

func TestOrdinaryRound_FiresNoAmbushFoiled(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	def.GetCharacter().Health = 10
	fired := stubHookMobEvents(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		WeaponHits:          surpriseWeaponHits(1, false),
		DefenderWasAttacked: true,
	})

	require.Empty(t, *fired)
}
