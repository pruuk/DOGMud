package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// castStartScene has Aliceia (user 1) begin a plain self-targeted spell with
// Bobrick (user 2) watching in the same lit room, and returns what Bobrick
// read. hidden makes Aliceia a sneaking caster Bobrick cannot perceive.
func castStartScene(t *testing.T, hidden bool) []string {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	alice, _, room := speechWrapperScene(t)

	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"teststart-plain": {
			SpellId: "teststart-plain", Name: "Test Start Plain",
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm,
			Targeting: combatvocab.TargetSelf,
			Schools:   []string{spells.SchoolManifestation},
			BaseFolds: 4, Cost: 5,
		},
	}))
	alice.Character.Activity = activity.NewMachine()
	alice.Character.ConvictionMax.Value, alice.Character.Conviction = 4000, 4000
	alice.Character.Skills = map[string]int{"manifestation": 5}
	alice.Character.SpellBook = map[string]int{"teststart-plain": 1}
	if hidden {
		r := state.TransitionReason{Trigger: "cast_start_room_line_test"}
		alice.Character.Awareness = awareness.NewMachine()
		require.NoError(t, alice.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
		alice.Character.Awareness.ResolveConcealment(true, r)
		require.True(t, alice.Character.IsHidden(), "fixture: the caster is hidden")
		require.False(t, users.GetByUserId(2).Character.Perceives(alice.Character),
			"fixture: the reader must not perceive the caster")
	}

	handled, err := Cast("teststart-plain", alice, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, alice.Character.Activity.IsCasting(), "fixture: the cast began")
	return speechWrapperHeard(2)
}

// Messaging M6 slice 4: the player cast start sends the room ONE line,
// through NarrateSpellPhase (the "closes their eyes" send beside it named a
// hidden caster to every sighted reader and doubled the line for the rest).
func TestCastStart_RoomReadsOneLine(t *testing.T) {
	got := castStartScene(t, false)
	require.Len(t, got, 1, "one room line, not two: %v", got)
	require.Contains(t, got[0], "Aliceia")
}

func TestCastStart_HiddenCasterIsNotNamedToANonPerceiver(t *testing.T) {
	got := castStartScene(t, true)
	require.Len(t, got, 1, "only the unnamed murmur: %v", got)
	require.NotContains(t, got[0], "Aliceia")
	require.NotContains(t, got[0], "closes their eyes")
}
