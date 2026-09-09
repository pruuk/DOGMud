package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// TestTauntLinesAreAnonymizable guards the property that actually matters about
// the {sourcetype} and {targettype} tokens: rhetoric.yaml drops them straight
// into `<ansi fg="...">`, so they must be real ANSI aliases that
// messaging.Anonymize recognises.
//
// They were "mob" and "user" until 2026-09-09, which are not aliases at all.
// The target's name therefore rendered uncoloured, and Anonymize could not see
// the tag, so a taunt line delivered to an observer who cannot see would have
// leaked the target's real name.
//
// The store golden cannot catch this: it feeds GetTauntTriad its own stand-ins
// rather than the values the command passes.
func TestTauntLinesAreAnonymizable(t *testing.T) {
	seedTauntStoreForAnonTest(t)

	const (
		sourceName = "Taunterly"
		targetName = "Targetticus"
	)

	// The exact type strings usercommands.Taunt passes for a MOB target.
	triad := combat.GetTauntTriad(combat.TauntHit, sourceName, targetName,
		"username", "mobname", "ModerateWounds", narration.SequencePicker())

	if triad.ToRoom == "" {
		t.Fatal("fixture produced no room line")
	}

	anon := messaging.Anonymize(triad.ToRoom)

	if strings.Contains(anon, sourceName) {
		t.Errorf("the taunter's name survived anonymization: %q", anon)
	}
	if strings.Contains(anon, targetName) {
		t.Errorf("the target's name survived anonymization: %q", anon)
	}
}

// TestTauntLinesAreAnonymizableForAPlayerTarget covers the other branch, where
// the target is a player and the alias is username rather than mobname.
func TestTauntLinesAreAnonymizableForAPlayerTarget(t *testing.T) {
	seedTauntStoreForAnonTest(t)

	const (
		sourceName = "Taunterly"
		targetName = "Targetticus"
	)

	triad := combat.GetTauntTriad(combat.TauntHit, sourceName, targetName,
		"username", "username", "ModerateWounds", narration.SequencePicker())

	anon := messaging.Anonymize(triad.ToRoom)

	if strings.Contains(anon, sourceName) || strings.Contains(anon, targetName) {
		t.Errorf("a name survived anonymization: %q", anon)
	}
}

// seedTauntStoreForAnonTest installs a room line shaped exactly like the
// authored ones: both names wrapped in an ANSI tag whose colour comes from a
// token.
func seedTauntStoreForAnonTest(t *testing.T) {
	t.Helper()
	restore := combat.SeedTauntMessagesForTest(map[combat.TauntIntensity]*combat.TauntMessages{
		combat.TauntHit: {
			ToAttacker: []string{`You sneer at <ansi fg="{targettype}">{target}</ansi>!`},
			ToDefender: []string{`<ansi fg="{sourcetype}">{source}</ansi> sneers at you!`},
			ToRoom:     []string{`<ansi fg="{sourcetype}">{source}</ansi> sneers at <ansi fg="{targettype}">{target}</ansi>!`},
		},
	})
	t.Cleanup(restore)
}
