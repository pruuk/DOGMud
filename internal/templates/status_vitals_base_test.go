package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusTemplatePath is the sheet `status` actually renders.
func statusTemplatePath() string {
	return filepath.Join("..", "..", "_datafiles", "world", "dogmud",
		"templates", "character", "status.template")
}

// The status sheet and the prompt bar have to measure the same pool.
//
// They did not. The sheet divided by the RAW max while renderVitalBar coloured
// itself against the reachable one, so a 2026-08-15 playtest could read "low"
// on the sheet and a comfortable bar in the same breath, on the same character,
// about the same health. Two readouts of one number must not have two
// denominators.
//
// The line is asserted as text because the denominator is authored in the data
// file, not in Go: nothing else in the build would notice it being changed back.
func TestStatusTemplate_MeasuresVitalsAgainstTheReachablePool(t *testing.T) {
	raw, err := os.ReadFile(statusTemplatePath())
	require.NoError(t, err, "the status sheet must be readable from the templates package")

	var vitalsLine string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "vitalQuality") {
			vitalsLine = line
			break
		}
	}
	require.NotEmpty(t, vitalsLine, "no vitalQuality row found in the status sheet")

	for _, pool := range []string{"health", "stamina", "conviction"} {
		assert.Contains(t, vitalsLine, `.Character.EffectivePoolMaxNamed "`+pool+`"`,
			"the %s cell must be measured against the pool the character can reach, "+
				"the same denominator the prompt bar uses", pool)
	}

	for _, rawMax := range []string{
		".Character.HealthMax.Value",
		".Character.StaminaMax.Value",
		".Character.ConvictionMax.Value",
	} {
		assert.NotContains(t, vitalsLine, rawMax,
			"the vitals row must not divide by the raw max: reservation is unreachable "+
				"pool, so a fully recovered reserved character would never read full")
	}
}

// The behavioural half. A character recovered as far as they can recover reads
// "full", where dividing by the raw max called the same character "moderate"
// forever and disagreed with the prompt bar's own gauge.
func TestVitalQuality_FullMeansAsRecoveredAsYouCanBe(t *testing.T) {
	c := &characters.Character{}
	c.HealthMax.Value = 440
	c.Companions = nil
	c.Health = 264 // the reachable ceiling with 176 reserved

	// Stand in for the 176 points of reservation the playtest character carried.
	c.ConvictionMax.Value = 440
	c.Conviction = 264
	c.Companions = []characters.CompanionInfo{{ConvictionReserve: 176}}

	require.Equal(t, 264, c.EffectivePoolMaxNamed("conviction"),
		"fixture: the reachable conviction ceiling")

	out, err := ProcessText(
		`{{ vitalQuality .Character.Conviction (.Character.EffectivePoolMaxNamed "conviction") 13 }}`,
		map[string]any{"Character": c},
	)
	require.NoError(t, err)

	assert.Contains(t, out, "full",
		"a character at their reachable ceiling must read full, not be told they are "+
			"partway down a pool they can never fill: got %q", out)
	assert.NotContains(t, out, "moderate")
}
