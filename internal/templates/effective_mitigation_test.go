package templates

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinShippedCurve pins the shipped mitigation curve on all three channels,
// restored on cleanup, so the bands below do not depend on the Go defaults
// happening to equal config.yaml.
func pinShippedCurve(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.PhysicalMitigationKnee, b.PhysicalMitigationCap = 0.50, 0.90
	b.PhysicalMitigationAim, b.PhysicalMitigationBend = 0.05, 0.40
	b.MagicalMitigationKnee, b.MagicalMitigationCap = 0.50, 0.90
	b.MagicalMitigationAim, b.MagicalMitigationBend = 0.05, 0.40
	b.ConvictionMitigationKnee, b.ConvictionMitigationCap = 0.50, 0.90
	b.ConvictionMitigationAim, b.ConvictionMitigationBend = 0.05, 0.40
	configs.SetConfigForTest(t, cfg)
}

// renderMitigationBand runs the real template engine over the status sheet's
// Defenses expression, so a typo in the registered name, an arity mismatch or
// a bad argument type surfaces here rather than on a player's `status`.
func renderMitigationBand(t *testing.T, c *characters.Character, channel string) (string, error) {
	t.Helper()
	out, err := ProcessText(
		`{{ mitigationQuality (effectiveMitigation .C "`+channel+`") }}`,
		map[string]any{"C": c},
	)
	return strings.TrimRight(out, "\r\n"), err
}

// #465: the status sheet describes the protection combat applies, not the
// raw sum. 75 raw points used to read "fortified" (the old 75% cap); the
// curve bends them to about 67%, which reads "heavy". 370 points reach the
// 90% cap and read "fortified". Below the knee nothing changes.
func TestEffectiveMitigation_StatusBandsReadTheCurve(t *testing.T) {
	pinShippedCurve(t)
	c := characters.New()
	c.Equipment.Body = items.Item{ItemId: 1, Spec: &items.ItemSpec{
		PhysicalMitigation:   75,
		MagicalMitigation:    30,
		ConvictionMitigation: 370,
	}}

	got, err := renderMitigationBand(t, c, "physical")
	require.NoError(t, err)
	assert.Equal(t, "heavy", got, "75 raw points bend to about 67%%")

	got, err = renderMitigationBand(t, c, "magical")
	require.NoError(t, err)
	assert.Equal(t, "moderate", got, "30 raw points are below the knee and read as before")

	got, err = renderMitigationBand(t, c, "conviction")
	require.NoError(t, err)
	assert.Equal(t, "fortified", got, "370 raw points reach the cap")
}

// A channel name the function does not know fails the render rather than
// silently describing some other channel.
func TestEffectiveMitigation_UnknownChannelFailsTheRender(t *testing.T) {
	_, err := renderMitigationBand(t, characters.New(), "physcial")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "physcial")
}
