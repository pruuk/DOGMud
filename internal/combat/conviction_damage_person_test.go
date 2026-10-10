package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #262: the bands say "their resolve" etc. The one who took the damage
// reads the same band about their own: "your resolve".
func TestGetConvictionDamageDescriptionToTarget_SecondPerson(t *testing.T) {
	for _, tc := range []struct {
		damage, max int
		want        string
	}{
		{2, 100, "a feeble jab at your resolve"},
		{10, 100, "a stinging insult"},
		{20, 100, "a rattling verbal assault"},
		{40, 100, "a crushing blow to your confidence"},
		{60, 100, "a devastating attack on your will"},
		{90, 100, "a soul-shattering tirade"},
		{10, 0, "a mild rebuke"},
	} {
		assert.Equal(t, tc.want, GetConvictionDamageDescriptionToTarget(tc.damage, tc.max), "%d of %d", tc.damage, tc.max)
	}
	// The onlooker form is unchanged.
	assert.Equal(t, "a devastating attack on their will", GetConvictionDamageDescription(60, 100))
}

// The retort: the counterer reads "their will", the countered "your will".
func TestBuildCounterTauntMessages_CounteredReadsYourWill(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(map[items.DefencePool]*items.DefenseMessageGroup{
		items.CounterPoolDefy: counterDefyMessageFixture(),
	})
	defer restore()
	counterer, countered := counterTauntFixtureChars("Selka", "Rurik")

	countererMsg, taunterMsg, _ := BuildCounterTauntMessages(counterer, countered, false, 120, 200)
	require.Contains(t, countererMsg, "a devastating attack on their will")
	require.Contains(t, taunterMsg, "a devastating attack on your will")
	require.NotContains(t, taunterMsg, "their will")
}

// The fallback lines when the counter-defy pool is not loaded.
func TestBuildCounterTauntMessages_FallbackCounteredReadsYourWill(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(nil)
	defer restore()
	counterer, countered := counterTauntFixtureChars("Selka", "Rurik")

	countererMsg, taunterMsg, _ := BuildCounterTauntMessages(counterer, countered, false, 120, 200)
	require.Contains(t, countererMsg, "a devastating attack on their will")
	require.Contains(t, taunterMsg, "a devastating attack on your will")
	require.NotContains(t, taunterMsg, "their will")
}

// A taunt renders {damage} once for all three audiences; the defender's
// line swaps in the second-person band (#262).
func TestTauntTriad_WithDefenderDamage(t *testing.T) {
	triad := TauntTriad{
		ToAttacker: "You sneer at Bob! (a feeble jab at their resolve)",
		ToDefender: "Ann sneers at you! (a feeble jab at their resolve)",
		ToRoom:     "Ann sneers at Bob!",
	}
	got := triad.WithDefenderDamage("a feeble jab at their resolve", "a feeble jab at your resolve")
	assert.Equal(t, "You sneer at Bob! (a feeble jab at their resolve)", got.ToAttacker)
	assert.Equal(t, "Ann sneers at you! (a feeble jab at your resolve)", got.ToDefender)
	assert.Equal(t, "Ann sneers at Bob!", got.ToRoom)
	assert.Equal(t, triad, triad.WithDefenderDamage("", ""), "no damage, no change")
}
