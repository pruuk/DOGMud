package spells

import (
	"fmt"
	"os"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
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
	// CastStartedRoom and CastContinuingRoom are the room's line for a cast
	// start and a fold round when the spell authors no *_observer line
	// (messaging M6 slice 4, owner ruling R7). {actor} is the caster.
	CastStartedRoom    []string `yaml:"cast_started_room"`
	CastContinuingRoom []string `yaml:"cast_continuing_room"`
	// CastStartedUnseen is what a reader who does not perceive a hidden
	// caster hears when the cast starts (owner ruling R5). It names nobody
	// and does not say who is targeted.
	CastStartedUnseen []string `yaml:"cast_started_unseen"`
}

var (
	castingMessages       *CastingMessages
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
		{"cast_started_unseen", cm.CastStartedUnseen},
	}
	for _, p := range pools {
		if err := narration.ValidateVariants(narration.Variants{Actor: p.pool}, minCastingVariants, narration.RoleActor); err != nil {
			return fmt.Errorf("casting-messages %s: %w", p.name, err)
		}
	}
	// The two room pools hold {actor} lines, so they validate as Observer lines.
	roomPools := []struct {
		name string
		pool []string
	}{
		{"cast_started_room", cm.CastStartedRoom},
		{"cast_continuing_room", cm.CastContinuingRoom},
	}
	for _, p := range roomPools {
		if err := narration.ValidateVariants(narration.Variants{Observer: p.pool}, minCastingVariants, narration.RoleObserver); err != nil {
			return fmt.Errorf("casting-messages %s: %w", p.name, err)
		}
	}
	return nil
}

// LoadCastingMessages reads and validates the store. Called once at startup
// from main.go, alongside the other message-store loaders.
//
// It PANICS on a missing, unparseable or invalid file, matching how defence
// and combat-messages behave (internal/items/itemspec.go:788, :795).
//
// There used to be a defaultCastingMessages() fallback that silently
// substituted hardcoded Go text when the YAML was absent. It was DELETED on
// 2026-09-09: a fallback that shadows shipped data means a typo in the YAML
// changes what players read and nobody finds out, the same hazard as reading a
// balance number from a Go default instead of config.yaml. Its content moved
// to _datafiles/world/default/casting-messages.yaml, where an author can edit
// it.
//
// 🔑 LOADING IS SEPARATE FROM RENDERING ON PURPOSE. This used to be a lazy
// sync.Once inside GetCastMessage, which meant "boot-fail" was really "crash on
// the first cast", and it crashed in any context that never booted a world at
// all: a unit test calling Cast() resolves the data path relative to ITS
// working directory, so the file is simply not there. Loading here makes the
// failure happen at startup, where it belongs, and leaves GetCastMessage safe
// to call from anywhere.
func LoadCastingMessages() {
	start := time.Now()

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
	mudlog.Info("spells.LoadCastingMessages()", "categories", 7, "Time Taken", time.Since(start))
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
	// Not loaded means no world was booted, which happens in unit tests that
	// exercise the cast path without world data. Returning the generic sentence
	// is correct there: there is no authored text to shadow, so this is not the
	// Go-default hazard that deleting defaultCastingMessages closed. A booted
	// server always has this loaded, or it panicked in LoadCastingMessages.
	if !castingMessagesLoaded || castingMessages == nil {
		return "Something stirs with " + spellName + "."
	}
	cm := castingMessages

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

// CastRoomPool is the room's generic line for a phase whose spell authors no
// *_observer line (owner ruling R7): cast_started_room for the cast,
// cast_continuing_room for a fold round, none for magic. With no world
// loaded (unit tests) it returns the one sentence each pool started from,
// for the reason GetCastMessage gives: there is no authored text to shadow.
func CastRoomPool(p Phase) []string {
	if castingMessagesLoaded && castingMessages != nil {
		switch p {
		case PhaseCast:
			if len(castingMessages.CastStartedRoom) > 0 {
				return castingMessages.CastStartedRoom
			}
		case PhaseWait:
			if len(castingMessages.CastContinuingRoom) > 0 {
				return castingMessages.CastContinuingRoom
			}
		}
	}
	switch p {
	case PhaseCast:
		return []string{"{actor} begins weaving a spell."}
	case PhaseWait:
		return []string{"{actor} weaves magic with focused intent."}
	}
	return nil
}

// CastStartedUnseenLine picks the line a reader who does not perceive a
// hidden caster hears at cast start (owner ruling R5). A nil picker is passed
// through to narration.Render as GetCastMessage does.
func CastStartedUnseenLine(picker ...narration.Picker) string {
	pool := []string{"You hear a faint murmur nearby, though you see no one."}
	if castingMessagesLoaded && castingMessages != nil && len(castingMessages.CastStartedUnseen) > 0 {
		pool = castingMessages.CastStartedUnseen
	}
	var pick narration.Picker
	if len(picker) > 0 {
		pick = picker[0]
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pick).Actor
}
