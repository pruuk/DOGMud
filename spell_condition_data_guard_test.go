package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"gopkg.in/yaml.v3"
)

// Messaging M6 slice 1: the shipped spells and the conditions they land.
// Read from the YAML on disk, so a data edit that breaks a rule fails here
// and names the file.

// shippedSpells loads every dogmud spell, keyed by spell id.
func shippedSpells(t *testing.T) map[string]*spells.SpellData {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "spells", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no spell files found: %v", err)
	}
	out := map[string]*spells.SpellData{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var s spells.SpellData
		if err := yaml.Unmarshal(raw, &s); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[s.SpellId] = &s
	}
	return out
}

// shippedConditions loads every dogmud condition, keyed by id.
func shippedConditions(t *testing.T) map[int]*conditions.ConditionSpec {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "conditions", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no condition files found: %v", err)
	}
	out := map[int]*conditions.ConditionSpec{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var c conditions.ConditionSpec
		if err := yaml.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[c.ConditionId] = &c
	}
	return out
}

func sortedSpellIds(all map[string]*spells.SpellData) []string {
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// conditionIdReaders are the effect types whose applier reads condition_ids
// (internal/hooks: applySpellConditionEffect, applySpellShield,
// applySpellHeal). On any other effect type the list is dead data that
// promises the player something the spell never does: Chrysalis Cocoon's
// [52] and Mass Mend's [33] were exactly that.
var conditionIdReaders = []string{"condition", "shield", "heal"}

func TestSpellConditionIdsAreReadByTheirEffectType(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	var problems []string
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		if len(s.ConditionIds) == 0 {
			continue
		}
		if !slices.Contains(conditionIdReaders, s.EffectType) {
			problems = append(problems, fmt.Sprintf("%s: effect_type %q never reads condition_ids %v", id, s.EffectType, s.ConditionIds))
		}
		for _, cid := range s.ConditionIds {
			if conds[cid] == nil {
				problems = append(problems, fmt.Sprintf("%s: condition %d does not exist", id, cid))
			}
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d spell condition_ids problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}

// familyFor is the family each applier's own condition must belong to, and
// the effect it must read from the applier's magnitude.
var familyFor = map[string]struct {
	family string
	reads  conditions.EffectKind
}{
	"shield": {conditions.FamilyWard, conditions.EffectMitigationFlat},
}

// R3, R4 and R7: each shield spell lands its own condition, one per spell,
// in the ward family, reading the spell's strength from its magnitude.
func TestShieldAndHealSpellsLandTheirOwnCondition(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	owner := map[int]string{}
	var problems []string
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		want, ok := familyFor[s.EffectType]
		if !ok {
			continue
		}
		if len(s.ConditionIds) != 1 {
			problems = append(problems, fmt.Sprintf("%s: a %s spell names exactly one condition, has %v", id, s.EffectType, s.ConditionIds))
			continue
		}
		cid := s.ConditionIds[0]
		c := conds[cid]
		if c == nil {
			continue // reported by TestSpellConditionIdsAreReadByTheirEffectType
		}
		if c.Family != want.family {
			problems = append(problems, fmt.Sprintf("%s: condition %d (%s) is family %q, want %q", id, cid, c.Name, c.Family, want.family))
		}
		if v, ok := c.Effects[want.reads]; !ok || !v.UsesMagnitude {
			problems = append(problems, fmt.Sprintf("%s: condition %d (%s) must read %s from the spell's magnitude", id, cid, c.Name, want.reads))
		}
		if prev, dup := owner[cid]; dup {
			problems = append(problems, fmt.Sprintf("%s: condition %d is also %s's; each spell has its own", id, cid, prev))
		}
		owner[cid] = id
	}
	if len(problems) > 0 {
		t.Fatalf("%d shield or heal spell problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}

// R7: three wards, weakest to strongest, each blocking more kinds of damage
// than the last. Strength and cost rise with the kinds.
func TestWardRosterHoldsItsOrdering(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	roster := []struct {
		spellId string
		kinds   []conditions.EffectKind
	}{
		{"conviction-ward", []conditions.EffectKind{conditions.EffectMitigationFlat}},
		{"conviction-bulwark", []conditions.EffectKind{conditions.EffectMitigationFlat, conditions.EffectMitigationMagical}},
		{"chrysalis-cocoon", []conditions.EffectKind{conditions.EffectMitigationFlat, conditions.EffectMitigationMagical, conditions.EffectMitigationConviction}},
	}
	every := []conditions.EffectKind{conditions.EffectMitigationFlat, conditions.EffectMitigationMagical, conditions.EffectMitigationConviction}
	prevMag, prevCost := 0, 0
	for _, w := range roster {
		s := all[w.spellId]
		if s == nil || len(s.ConditionIds) != 1 || conds[s.ConditionIds[0]] == nil {
			t.Fatalf("%s: missing, or not landing exactly one shipped condition", w.spellId)
		}
		c := conds[s.ConditionIds[0]]
		for _, k := range every {
			_, has := c.Effects[k]
			if has != slices.Contains(w.kinds, k) {
				t.Errorf("%s: ward %d (%s) blocks %s = %v, want %v", w.spellId, c.ConditionId, c.Name, k, has, !has)
			}
		}
		if s.EffectMagnitude <= prevMag || s.Cost <= prevCost {
			t.Errorf("%s: magnitude %d and cost %d must both exceed the weaker ward's %d and %d",
				w.spellId, s.EffectMagnitude, s.Cost, prevMag, prevCost)
		}
		prevMag, prevCost = s.EffectMagnitude, s.Cost
	}
}
