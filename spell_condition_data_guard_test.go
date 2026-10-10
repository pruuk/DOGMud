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
// applySpellHeal, and applySpellDot through spellDotConditionId, #249). On
// any other effect type the list is dead data that promises the player
// something the spell never does: Chrysalis Cocoon's [52] and Mass Mend's
// [33] were exactly that. A dot's named record must tell its own start, as
// every other reader's must (TestSpellConditionsNarrateTheirOwnStart).
var conditionIdReaders = []string{"condition", "shield", "heal", "dot"}

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
	"heal":   {conditions.FamilyHeal, conditions.EffectRegenMult},
}

// R3, R4, R7 and R8: each shield and heal spell lands its own condition,
// one per spell, in the ward or heal family, reading the spell's strength
// (a ward's points, a heal's multiplier) from its magnitude.
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

// healIdentity is one heal spell as the player meets it: its multiplier
// (effect_magnitude), its authored duration (its condition's triggercount)
// and whether it lands on everyone (targeting area).
type healIdentity struct {
	mult, rounds int
	area         bool
}

func shippedHeal(t *testing.T, all map[string]*spells.SpellData, conds map[int]*conditions.ConditionSpec, id string) healIdentity {
	t.Helper()
	s := all[id]
	if s == nil || len(s.ConditionIds) != 1 || conds[s.ConditionIds[0]] == nil {
		t.Fatalf("%s: missing, or not landing exactly one shipped condition", id)
	}
	return healIdentity{mult: s.EffectMagnitude, rounds: conds[s.ConditionIds[0]].TriggerCount, area: string(s.Targeting) == "area"}
}

// R9: the heal roster's identities. Mend Flesh is long and gentle, Mend
// Wounds short and strong; Mend All is the short, light area heal,
// Communion of Flesh the long, steady one and Mass Mend the short, strong
// one at the top. Chrysalis Regeneration (33) and Vital Surge (32) are in
// the heal family; Regenerating (120), the mob feeding record, is not.
func TestHealRosterHoldsItsIdentities(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	flesh := shippedHeal(t, all, conds, "heal")
	wounds := shippedHeal(t, all, conds, "mend-wounds")
	mendAll := shippedHeal(t, all, conds, "mend-all")
	communion := shippedHeal(t, all, conds, "communion-of-flesh")
	mass := shippedHeal(t, all, conds, "mass-mend")

	if flesh.area || wounds.area || !mendAll.area || !communion.area || !mass.area {
		t.Errorf("targeting: Mend Flesh and Mend Wounds single, the other three area: %+v %+v %+v %+v %+v", flesh, wounds, mendAll, communion, mass)
	}
	if !(flesh.mult < wounds.mult && flesh.rounds > wounds.rounds) {
		t.Errorf("Mend Flesh must be gentler and longer than Mend Wounds: %+v vs %+v", flesh, wounds)
	}
	if !(communion.rounds > mendAll.rounds && communion.rounds > mass.rounds) {
		t.Errorf("Communion of Flesh must be the longest area heal: %+v vs %+v, %+v", communion, mendAll, mass)
	}
	if !(mass.mult > communion.mult && mass.mult > mendAll.mult) {
		t.Errorf("Mass Mend must be the strongest area heal: %+v vs %+v, %+v", mass, communion, mendAll)
	}
	total := func(h healIdentity) int { return h.mult * h.rounds }
	if !(total(mendAll) < total(communion) && total(mendAll) < total(mass)) {
		t.Errorf("Mend All must be the lightest area heal in all: %d vs %d, %d", total(mendAll), total(communion), total(mass))
	}
	for _, id := range []int{32, 33} {
		if c := conds[id]; c == nil || c.Family != conditions.FamilyHeal {
			t.Errorf("condition %d must be in the heal family", id)
		}
	}
	if c := conds[conditions.ConditionIdRegenerating]; c == nil || c.Family != "" {
		t.Errorf("condition %d (the feeding record) must stay out of the heal family", conditions.ConditionIdRegenerating)
	}
}

// R1, R2 and R11: a condition a spell lands tells its own start, one line
// per audience: start_actor for a caster who is someone else, start_actee
// for the holder and start_observer for the room
// (ConditionSpec.NarratesCastStart), so the spell's generic "takes effect"
// trio never has to stand in. A silent-start condition would bring the trio
// back, so none may be landed by a spell.
func TestSpellConditionsNarrateTheirOwnStart(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	var problems []string
	checked := 0
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		if !slices.Contains(conditionIdReaders, s.EffectType) {
			continue
		}
		for _, cid := range s.ConditionIds {
			c := conds[cid]
			if c == nil {
				continue // reported by TestSpellConditionIdsAreReadByTheirEffectType
			}
			checked++
			if !c.NarratesCastStart(false) {
				problems = append(problems, fmt.Sprintf("%s: condition %d (%s) must author start_actor, start_actee and start_observer and not be silent-start",
					id, cid, c.Name))
			}
		}
	}
	// A guard that checked nothing would pass vacuously.
	if checked < 20 {
		t.Fatalf("only %d spell-landed condition ids were checked, want at least 20", checked)
	}
	if len(problems) > 0 {
		t.Fatalf("%d spell-landed conditions cannot tell their own start:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}

// A dot spell lands one record (spellDotConditionId reads ConditionIds[0]),
// so a second id would be dead data (#249).
func TestDotSpellsNameAtMostOneCondition(t *testing.T) {
	all := shippedSpells(t)
	dots := 0
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		if s.EffectType != "dot" {
			continue
		}
		dots++
		if len(s.ConditionIds) > 1 {
			t.Errorf("%s: a dot spell lands one record, names %v", id, s.ConditionIds)
		}
	}
	// Blood Boil and Neural Toxin; a guard that found none would pass vacuously.
	if dots < 2 {
		t.Fatalf("only %d dot spells found, want at least 2", dots)
	}
}
