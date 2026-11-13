package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `help spell <name>` and the spell fallback rendered help/spell.template
// over a SpellData VALUE, but the template calls HelpOrHarmString,
// TargetTypeString and DefenceNames, which have pointer receivers. The
// render failed with a template error and players got "No help found"
// (found by the plan 4 help survey, 2026-11-11). Every path must pass the
// pointer.
func TestGetHelpContents_SpellRendersThroughTheSpellTemplate(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"probe-bolt": {
			SpellId:     "probe-bolt",
			Name:        "Probe Bolt",
			Description: "A probing bolt.",
			AttackType:  combatvocab.AttackSpell,
			DamageType:  combatvocab.DamageMental,
			Targeting:   combatvocab.TargetSingle,
		},
	})()
	useDogmudTemplates(t)

	for _, topic := range []string{"spell probe-bolt", "spell Probe Bolt"} {
		out, err := GetHelpContents(topic, 80)
		require.NoError(t, err, topic)
		assert.NotContains(t, out, "TEMPLATE ERROR", topic)
		assert.Contains(t, out, "Probe Bolt", topic)
		// The three rows that come from pointer-receiver methods.
		assert.Contains(t, out, `<ansi fg="spell-harmful">Harmful</ansi>`, "%s: HelpOrHarmString", topic)
		assert.Contains(t, out, `Single Target`, "%s: TargetTypeString", topic)
		assert.Contains(t, out, `<ansi fg="white-bold">quell</ansi>`, "%s: DefenceNames", topic)
	}
}
