package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/require"
)

// newShoutRecordActor builds a bare character with the rhetoric and charisma
// the magnitude formula reads, mirroring newRhetoricActor's fixture shape.
func newShoutRecordActor(rhetoric int, charisma int) *characters.Character {
	char := characters.New()
	char.Skills[string(skills.Rhetoric)] = rhetoric
	char.Stats.Charisma.ValueAdj = charisma
	return char
}

// TestApplyWarcryEffectAppliesOneRecord pins that the warcry applier now
// writes exactly one record, holding the exact round count and a damage
// multiplier of 1 + bonus, rather than a condition plus a separate buff.
func TestApplyWarcryEffectAppliesOneRecord(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()

	char := newShoutRecordActor(30, 100)
	bonus, duration := ApplyWarcryEffect(char)

	require.GreaterOrEqual(t, bonus, 0.05)
	require.LessOrEqual(t, bonus, 0.20)
	require.Equal(t, 25, duration)
	require.Equal(t, 25, char.Buffs.TriggersLeft(buffs.BuffIdWarcry))
	require.InDelta(t, 1.0+bonus, char.Buffs.Effect(buffs.EffectDamageMult), 1e-9)
	require.Len(t, char.Buffs.List, 1, "warcry must apply exactly one buff record")
	require.Empty(t, char.Conditions, "warcry must not also write a legacy combat condition")
}

// TestApplyRallyEffectAppliesOneRecord mirrors the warcry pin for rally's
// defense multiplier.
func TestApplyRallyEffectAppliesOneRecord(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()

	char := newShoutRecordActor(30, 100)
	bonus, duration := ApplyRallyEffect(char)

	require.GreaterOrEqual(t, bonus, 0.05)
	require.LessOrEqual(t, bonus, 0.20)
	require.Equal(t, 25, duration)
	require.Equal(t, 25, char.Buffs.TriggersLeft(buffs.BuffIdRally))
	require.InDelta(t, 1.0+bonus, char.Buffs.Effect(buffs.EffectDefenseMult), 1e-9)
	require.Len(t, char.Buffs.List, 1, "rally must apply exactly one buff record")
	require.Empty(t, char.Conditions, "rally must not also write a legacy combat condition")
}
