package spells

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// seedRegistry populates the in-memory spell registry for tests without disk access.
func seedRegistry() {
	allSpells = map[string]*SpellData{
		"sparks": {
			SpellId:          "sparks",
			Name:             "Sparks",
			Type:             HarmSingle,
			Cost:             3,
			HealthCost:       0,
			Difficulty:       10,
			DamageMultiplier: 0.8,
			BaseFolds:        4,
			Schools:          []string{SchoolElemental},
		},
		"heal": {
			SpellId:         "heal",
			Name:            "Heal",
			Type:            HelpSingle,
			Cost:            5,
			HealthCost:      0,
			Difficulty:      15,
			EffectMagnitude: 3,
			BaseFolds:       6,
			Schools:         []string{SchoolVital},
		},
		"pyretic-surge": {
			SpellId:          "pyretic-surge",
			Name:             "Pyretic Surge",
			Type:             HarmSingle,
			Cost:             8,
			HealthCost:       2,
			Difficulty:       25,
			DamageMultiplier: 1.5,
			BaseFolds:        12,
			Schools:          []string{SchoolElemental, SchoolVital},
		},
		"conviction-ward": {
			SpellId:    "conviction-ward",
			Name:       "Conviction Ward",
			Type:       HelpSingle,
			Cost:       4,
			HealthCost: 0,
			Difficulty: 20,
			BaseFolds:  8,
			Schools:    []string{SchoolEnhancement},
		},
	}
}

// ─── FindSpell ──────────────────────────────────────────────────────────────

func TestFindSpell(t *testing.T) {
	seedRegistry()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"exact ID match", "sparks", "sparks"},
		{"exact ID match 2", "heal", "heal"},
		{"name match (lowercase)", "sparks", "sparks"},
		{"not found", "nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindSpell(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── GetSpell ───────────────────────────────────────────────────────────────

func TestGetSpell(t *testing.T) {
	seedRegistry()

	tests := []struct {
		name    string
		spellId string
		wantNil bool
	}{
		{"found", "sparks", false},
		{"not found", "nonexistent", true},
		{"found pyretic-surge", "pyretic-surge", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSpell(tt.spellId)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.spellId, got.SpellId)
			}
		})
	}
}

// ─── FindSpellByName ────────────────────────────────────────────────────────

func TestFindSpellByName(t *testing.T) {
	seedRegistry()

	tests := []struct {
		name    string
		input   string
		wantId  string
		wantNil bool
	}{
		{"exact name", "Sparks", "sparks", false},
		{"case-insensitive", "sparks", "sparks", false},
		{"prefix match", "Pyre", "pyretic-surge", false},
		{"not found", "zzz-no-match", "", true},
		{"exact name Heal", "Heal", "heal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindSpellByName(tt.input)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantId, got.SpellId)
			}
		})
	}
}

// ─── GetAllSpells ───────────────────────────────────────────────────────────

func TestGetAllSpells(t *testing.T) {
	seedRegistry()

	all := GetAllSpells()
	assert.Equal(t, 4, len(all))

	// Should be a copy — modifying it should not affect the registry
	all["test-extra"] = &SpellData{SpellId: "test-extra"}
	assert.Equal(t, 4, len(allSpells), "GetAllSpells should return a copy")
}

// ─── RequiredSkillFor ───────────────────────────────────────────────────────
//
// Replaces TestMaxFoldsForSkill. U10b-3 deleted the fold ladder that gated
// discovery: base_folds measures how long and involved a cast is, not how hard
// a spell is to learn, and using it as the gate let a spellcasting-0 novice
// discover difficulty-45 spells (Core Discharge, Core Drain) while making
// Charm, at base_folds 36 against a ceiling of 32, undiscoverable by anyone.
//
// ⚠️ A test binary does not load config.yaml, so the ratio here is the Go
// DEFAULT of 1.0 rather than whatever config.yaml ships. At 1.0 a spell's
// difficulty IS its required skill.
func TestRequiredSkillFor(t *testing.T) {
	tests := []struct {
		name       string
		difficulty int
		want       int
	}{
		{"difficulty 0 gates nothing", 0, 0},
		{"negative clamps to 0", -5, 0},
		{"difficulty 5", 5, 5},
		{"difficulty 25", 25, 25},
		{"Core Discharge / Core Drain", 45, 45},
		{"Charm", 60, 60},
		{"hardest authored spell", 75, 75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RequiredSkillFor(tt.difficulty))
		})
	}
}

// ─── GetTotalConvictionCost ─────────────────────────────────────────────────

func TestGetTotalConvictionCost(t *testing.T) {
	seedRegistry()

	spell := GetSpell("sparks") // cost = 3

	tests := []struct {
		name       string
		multiplier float64
		want       int
	}{
		{"1x multiplier", 1.0, 3},
		{"2x multiplier", 2.0, 6},
		{"0.5x multiplier", 0.5, 1},
		{"zero multiplier defaults to 1x", 0.0, 3},
		{"negative multiplier defaults to 1x", -1.0, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spell.GetTotalConvictionCost(tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── GetTotalHealthCost ─────────────────────────────────────────────────────

func TestGetTotalHealthCost(t *testing.T) {
	seedRegistry()

	spell := GetSpell("pyretic-surge") // healthcost = 2

	tests := []struct {
		name       string
		multiplier float64
		want       int
	}{
		{"1x multiplier", 1.0, 2},
		{"3x multiplier", 3.0, 6},
		{"zero multiplier defaults to 1x", 0.0, 2},
		{"negative multiplier defaults to 1x", -1.0, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spell.GetTotalHealthCost(tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}

	// Spell with no health cost
	sparks := GetSpell("sparks") // healthcost = 0
	assert.Equal(t, 0, sparks.GetTotalHealthCost(1.0))
}

// ─── GetSchoolsString ───────────────────────────────────────────────────────

func TestGetSchoolsString(t *testing.T) {
	seedRegistry()

	tests := []struct {
		name    string
		spellId string
		want    string
	}{
		{"single school", "sparks", "elemental"},
		{"multiple schools", "pyretic-surge", "elemental, vital"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spell := GetSpell(tt.spellId)
			assert.NotNil(t, spell)
			assert.Equal(t, tt.want, spell.GetSchoolsString())
		})
	}

	// Empty schools
	noSchool := &SpellData{Schools: nil}
	assert.Equal(t, "Unknown", noSchool.GetSchoolsString())
}

// ─── GetEligibleSpells ──────────────────────────────────────────────────────

func TestGetEligibleSpells(t *testing.T) {
	seedRegistry()

	// Player knows sparks; spellcasting 20 admits difficulty <= 20.
	known := map[string]int{"sparks": 1}
	eligible := GetEligibleSpells(known, 20)

	found := map[string]bool{}
	for _, id := range eligible {
		found[id] = true
	}
	assert.True(t, found["heal"], "heal (difficulty 15) should be eligible at skill 20")
	assert.True(t, found["conviction-ward"], "conviction-ward (difficulty 20) is exactly at the bar and should be eligible")
	assert.False(t, found["pyretic-surge"], "pyretic-surge (difficulty 25) should NOT be eligible at skill 20")
	assert.False(t, found["sparks"], "known spell should not appear")
}

// 🔴 THE TEST THAT ACTUALLY DISTINGUISHES THE TWO GATES.
//
// The case above passes under EITHER rule, because difficulty and base_folds
// happen to rank the fixture spells the same way -- it was written for the fold
// ladder and kept passing unchanged when U10b-3 replaced it. Agreeing for the
// wrong reason is not evidence, so this pins the difference directly.
//
// The fixture is the real Core Discharge shape: trivial to CAST (2 folds, well
// under the old ladder's floor of 4 at any skill) but genuinely hard to LEARN
// (difficulty 45). The old gate let a spellcasting-0 novice discover it. The
// difficulty gate does not.
func TestGetEligibleSpells_DifficultyGatesNotFolds(t *testing.T) {
	allSpells = map[string]*SpellData{
		"core-discharge": {
			SpellId:    "core-discharge",
			Name:       "Core Discharge",
			Type:       HarmArea,
			Difficulty: 45,
			BaseFolds:  2, // <= MaxFoldsForSkill(0), which was 4: the old gate ADMITTED this
			Schools:    []string{SchoolElemental},
		},
		"long-but-easy": {
			SpellId:    "long-but-easy",
			Name:       "Long But Easy",
			Type:       HarmSingle,
			Difficulty: 5,
			BaseFolds:  24, // > MaxFoldsForSkill(20), which was 10: the old gate REFUSED this
			Schools:    []string{SchoolElemental},
		},
	}

	found := map[string]bool{}
	for _, id := range GetEligibleSpells(map[string]int{}, 20) {
		found[id] = true
	}

	assert.False(t, found["core-discharge"],
		"difficulty 45 must not be discoverable at spellcasting 20; the old fold ladder admitted it at skill 0, which is the bug this replaced")
	assert.True(t, found["long-but-easy"],
		"difficulty 5 must be discoverable at spellcasting 20 despite 24 folds; folds measure cast length, not how hard a spell is to learn")
}

// ─── GetEligibleSpells quest-gated exclusion ─────────────────────────────────

func TestGetEligibleSpells_QuestGatedExcluded(t *testing.T) {
	// Build a registry that contains one regular spell and one quest-gated spell,
	// both well within the skill threshold so the only reason to exclude the
	// quest-gated one is the QuestRequired field.
	allSpells = map[string]*SpellData{
		"sparks": {
			SpellId:   "sparks",
			Name:      "Sparks",
			Type:      HarmSingle,
			BaseFolds: 4,
			Schools:   []string{SchoolElemental},
			// No QuestRequired → discoverable
		},
		"summon-steppe-spirit": {
			SpellId:       "summon-steppe-spirit",
			Name:          "Summon Steppe Spirit",
			Type:          HelpSingle,
			BaseFolds:     6,
			Schools:       []string{SchoolManifestation},
			QuestRequired: "12-end",
			// Quest-gated → must NEVER appear in eligible list
		},
	}

	// High skill → maxFolds = 32, so both spells are within range by folds.
	// Empty spellbook so neither is "known".
	known := map[string]int{}
	eligible := GetEligibleSpells(known, 100, SchoolElemental, SchoolManifestation)

	found := map[string]bool{}
	for _, id := range eligible {
		found[id] = true
	}

	assert.True(t, found["sparks"], "sparks (no quest required) should be discoverable")
	assert.False(t, found["summon-steppe-spirit"],
		"summon-steppe-spirit has QuestRequired set and must never appear in eligible list")
}

// ─── Categories field ────────────────────────────────────────────────────────

func TestSpellData_CategoriesYAMLRoundtrip(t *testing.T) {
	data := []byte(`
spellid: test-spell
name: Test Spell
categories:
  - self_defense
  - mental_defense
`)
	var sd SpellData
	if err := yaml.Unmarshal(data, &sd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sd.Categories) != 2 {
		t.Fatalf("want 2 categories, got %d", len(sd.Categories))
	}
	if sd.Categories[0] != "self_defense" || sd.Categories[1] != "mental_defense" {
		t.Fatalf("categories: %v", sd.Categories)
	}
}

func TestSpellData_CategoriesOmittedWhenEmpty(t *testing.T) {
	data := []byte(`spellid: test-spell
name: Test Spell
`)
	var sd SpellData
	if err := yaml.Unmarshal(data, &sd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sd.Categories != nil {
		t.Fatalf("want nil categories when field absent, got %v", sd.Categories)
	}
}
