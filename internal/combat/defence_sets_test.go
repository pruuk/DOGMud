package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/assert"
)

// TestDefenceSetFor pins the roadmap table. The applicable defence set is a
// property of the attack channel, and this test IS the table -- if a row here
// disagrees with docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md, one of the two is
// wrong and it is probably the code.
func TestDefenceSetFor(t *testing.T) {
	tests := []struct {
		name     string
		channel  AttackChannel
		expected []string
	}{
		{
			name:     "melee gets all three physical defences",
			channel:  ChannelMelee,
			expected: []string{characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock},
		},
		{
			name:     "ranged drops parry -- you cannot parry an arrow",
			channel:  ChannelRanged,
			expected: []string{characters.DefenseDodge, characters.DefenseBlock},
		},
		{
			name:     "physical spells reuse dodge and block, no parry",
			channel:  ChannelSpellPhysical,
			expected: []string{characters.DefenseDodge, characters.DefenseBlock},
		},
		{
			name:     "mental spells are answered by quell alone",
			channel:  ChannelSpellMental,
			expected: []string{characters.DefenseQuell},
		},
		{
			name:     "social attacks are answered by defy alone",
			channel:  ChannelSocial,
			expected: []string{characters.DefenseDefy},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, DefenceSetFor(tc.channel))
		})
	}
}

// TestDefenceSetForUnknownChannel: an unrecognised channel must come back empty
// rather than panicking or, worse, silently defaulting to the melee set. An
// empty set means "uncontested", which RunContest already handles; a wrong
// non-empty set would hand the defender defences they should not have.
func TestDefenceSetForUnknownChannel(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Empty(t, DefenceSetFor(AttackChannel("not-a-channel")))
	})
	assert.Empty(t, DefenceSetFor(AttackChannel("")))
}

// TestDefenceSetForReturnsKnownDefenceNames guards the seam between this table
// and characters.GetDefenseScore. Every name emitted here must score non-zero
// for a character, or the defence silently enters the contest at 0 and always
// loses. GetDefenseScore's default arm returns 0, so a typo'd constant would
// never fail to compile -- only this assertion catches it.
func TestDefenceSetForReturnsKnownDefenceNames(t *testing.T) {
	known := map[string]bool{
		characters.DefenseDodge: true,
		characters.DefenseParry: true,
		characters.DefenseBlock: true,
		characters.DefenseQuell: true,
		characters.DefenseDefy:  true,
	}

	for _, channel := range []AttackChannel{
		ChannelMelee, ChannelRanged, ChannelSpellPhysical,
		ChannelSpellMental, ChannelSocial,
	} {
		set := DefenceSetFor(channel)
		assert.NotEmpty(t, set, "channel %q must have at least one defence", channel)
		for _, def := range set {
			assert.True(t, known[def],
				"channel %q emits unknown defence %q", channel, def)
			assert.NotEqual(t, characters.DefenseNone, def,
				"channel %q emits the empty sentinel", channel)
		}
	}
}
