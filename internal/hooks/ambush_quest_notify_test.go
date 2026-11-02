package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 100, From the Dark): a player's opening strike from
// hiding is a quest step. The round carries the fact on
// AttackResult.WasSurpriseAttack (see surpriseCandidate), so the melee path
// raises `command ambush` from applyCombatProgression, aimed at the defender.
const (
	ambushProbeQuest = 99701
	ambushProbeToken = "99701-start"
)

func registerAmbushProbe(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: ambushProbeQuest,
		Name:    "Ambush Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "ambush",
			Target:  quests.TargetMob,
			Actions: []quests.ActionDef{{Grant: ambushProbeToken}},
		}},
	})
}

func TestSurpriseRound_RaisesTheAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(1, true),
		DefenderWasAttacked: true,
	})

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.True(t, u.Character.HasQuest(ambushProbeToken),
		"a player's surprise round must raise command ambush aimed at the mob")
}

func TestOrdinaryRound_RaisesNoAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WeaponHits:          surpriseWeaponHits(1, true),
		DefenderWasAttacked: true,
	})

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.False(t, u.Character.HasQuest(ambushProbeToken),
		"an ordinary round is no ambush")
}

// Owner ruling 2026-10-30: the event fires on the opener, landed or not.
func TestMissedSurpriseRound_StillRaisesTheAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(0, false),
		DefenderWasAttacked: true,
	})

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.True(t, u.Character.HasQuest(ambushProbeToken),
		"an opener that missed is still spent, so it still raises command ambush")
}

// Only a player has quests. A hidden mob opening on a player raises nothing,
// and the player it struck is not credited with an ambush.
func TestMobSurpriseRound_RaisesNoAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)

	// The fixture's mob swings at its player: the roles are swapped on purpose.
	runProgressionPhaseForTest(t, def, atk, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(1, true),
		DefenderWasAttacked: true,
	})

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.False(t, u.Character.HasQuest(ambushProbeToken),
		"a mob's surprise round must raise nothing")
}

// The real path, on a mob the opener kills. Death is resolved in Phase 8,
// after Phase 5 raised the event, so the mob is still there to be its target.
func TestLethalOpener_StillRaisesTheAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)
	atk.GetCharacter().SetAggro(0, def.GetMobInstanceId(), characters.SurpriseAttack)
	require.True(t, atk.GetCharacter().CombatPhase.OpeningUnspent(), "fixture failed to arm a surprise attack")
	def.GetCharacter().Health = 1

	runOneCombatRoundForTest(t, atk, def) // forceCrit: every swing is a clean hit

	require.LessOrEqual(t, def.GetCharacter().Health, 0, "the fixture's opener must kill, or this proves nothing")
	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.True(t, u.Character.HasQuest(ambushProbeToken),
		"an opener that kills must still raise command ambush aimed at the mob")
}

// The real path, over two rounds: the opener is spent on the first, so the
// event fires once for the engagement.
func TestSurpriseEngagement_RaisesTheAmbushQuestEventOnce(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	registerAmbushProbe(t)
	atk.GetCharacter().SetAggro(0, def.GetMobInstanceId(), characters.SurpriseAttack)
	u := users.GetByUserId(1)
	require.NotNil(t, u)

	runOneCombatRoundForTest(t, atk, def)
	require.True(t, u.Character.HasQuest(ambushProbeToken), "the opening round raises command ambush")
	u.Character.ClearQuestToken(ambushProbeToken)

	runOneCombatRoundForTest(t, atk, def)
	require.False(t, u.Character.HasQuest(ambushProbeToken),
		"the round after the opener is ordinary fighting and raises nothing")
}

// The real path, for an attacker who never hid.
func TestUnhiddenEngagement_RaisesNoAmbushQuestEvent(t *testing.T) {
	atk, def := newCombatPairForTest(t) // engaged with DefaultAttack
	registerAmbushProbe(t)

	runOneCombatRoundForTest(t, atk, def)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.False(t, u.Character.HasQuest(ambushProbeToken),
		"an attacker who was not hidden has no opener to raise")
}
