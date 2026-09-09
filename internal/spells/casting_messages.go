package spells

import (
	"fmt"
	"os"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// CastingMessages holds the varied atmospheric messages for the casting system.
type CastingMessages struct {
	AlreadyCasting []string `yaml:"already_casting"`
	CastStarted    []string `yaml:"cast_started"`
	// CastContinuing is the per-round line while folds are still being laid
	// down. It exists because the round loop used to reuse CastStarted, which
	// told the player the cast was *beginning* again on every round of a
	// multi-fold spell, and duplicated the real start line on the round right
	// after the cast was initiated.
	CastContinuing       []string `yaml:"cast_continuing"`
	ConcentrationSlipped []string `yaml:"concentration_slipped"`
}

var (
	castingMessages       *CastingMessages
	castingMessagesOnce   sync.Once
	castingMessagesLoaded bool
)

// minCastingVariants is casting's floor.
//
// THREE, not defence's five. casting-messages.yaml ships pools of 3, 3, 3 and
// 4, so defence's minimum would fail boot on shipped data without improving a
// single line of text. The validator's job is catching a REGRESSION (a pool
// emptied, a key renamed), not setting a content quality bar.
//
// The real content problem here is separate and filed as M6 work:
// cast_started fires on EVERY cast and has 3 variants, against 10 to 14 per
// pool for defence, so a caster sees a repeat every third spell.
const minCastingVariants = 3

// Validate enforces that every pool exists and is deep enough to be worth
// randomising.
//
// This store had no validator at all until 2026-09-09, because it is not in
// fileloader and therefore had no Validate() hook to implement. Each pool is a
// single-role narration.Variants, so the core's own rules apply to it.
func (cm *CastingMessages) Validate() error {
	pools := []struct {
		name string
		pool []string
	}{
		{"already_casting", cm.AlreadyCasting},
		{"cast_started", cm.CastStarted},
		{"cast_continuing", cm.CastContinuing},
		{"concentration_slipped", cm.ConcentrationSlipped},
	}
	for _, p := range pools {
		if err := narration.ValidateVariants(narration.Variants{Actor: p.pool}, minCastingVariants); err != nil {
			return fmt.Errorf("casting-messages %s: %w", p.name, err)
		}
	}
	return nil
}

// loadCastingMessages loads the YAML file once and caches the result.
//
// It PANICS on a missing, unparseable or invalid file, matching how defence
// and combat-messages behave (internal/items/itemspec.go:788, :795).
//
// There used to be a defaultCastingMessages() fallback here that silently
// substituted hardcoded Go text. It was DELETED on 2026-09-09: a fallback that
// shadows shipped data means a YAML typo changes what players read and nobody
// finds out, which is the same hazard as reading a balance number from a Go
// default instead of config.yaml. Boot-fail and a silent fallback cannot both
// be the policy, and boot-fail is the one the rest of the loaders use.
func loadCastingMessages() *CastingMessages {
	castingMessagesOnce.Do(func() {
		path := string(configs.GetFilePathsConfig().DataFiles) + `/casting-messages.yaml`
		data, err := os.ReadFile(path)
		if err != nil {
			panic(errors.Wrap(err, "reading "+path))
		}
		var cm CastingMessages
		if err := yaml.Unmarshal(data, &cm); err != nil {
			panic(errors.Wrap(err, "parsing "+path))
		}
		if err := cm.Validate(); err != nil {
			panic(errors.Wrap(err, "validating "+path))
		}
		castingMessages = &cm
		castingMessagesLoaded = true
	})
	return castingMessages
}

// GetCastMessage picks a message from the named category, substituting
// {spell} with spellName.
//
// category must be one of: "already_casting", "cast_started",
// "cast_continuing", "concentration_slipped".
//
// spellName is the player-facing DISPLAY name (spellInfo.Name), never the
// spellid. Passing the id leaks an internal identifier into player output,
// which is exactly what the round loop used to do.
//
// This is a DEGENERATE case for the narration core: one role (the caster), no
// band, one token. It renders through the same seam as the defence triad.
//
// The optional picker exists for the snapshot harness. Production passes none
// and gets narration.DefaultPicker, which routes through the engine's util.Rand
// seam. This file used stdlib math/rand directly until 2026-09-07, which made
// it unreachable by any seam and therefore unsnapshottable.
func GetCastMessage(category, spellName string, picker ...narration.Picker) string {
	cm := castingMessages
	if !castingMessagesLoaded {
		cm = loadCastingMessages()
	}

	var pool []string
	switch category {
	case "already_casting":
		pool = cm.AlreadyCasting
	case "cast_started":
		pool = cm.CastStarted
	case "cast_continuing":
		pool = cm.CastContinuing
	case "concentration_slipped":
		pool = cm.ConcentrationSlipped
	}

	// An unrecognised category is a CALLER bug, not a data problem, so it
	// cannot be caught by the validator and still needs a sentence: a caller
	// printing "" would show the player nothing at all.
	if len(pool) == 0 {
		return "Something stirs with " + spellName + "."
	}

	var pick narration.Picker
	if len(picker) > 0 {
		pick = picker[0]
	}

	return narration.Render(
		narration.Variants{Actor: pool},
		map[string]string{"{spell}": spellName},
		pick,
	).Actor
}
