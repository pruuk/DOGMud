package conditions

import (
	"fmt"
	"slices"
)

// A family is a set of conditions that replace one another instead of
// stacking (messaging M6 slice 1, section 3; owner rulings R4 and R6): the
// shield spells' wards, and the heal spells' heals. Landing a record whose
// spec names a family removes every other record of that family on the same
// holder first, so the newest stands alone. A record with no family (a
// potion, a salve, an item, a mutation) is never touched, so it keeps
// stacking with a ward or a heal.
const (
	FamilyWard = "ward"
	FamilyHeal = "heal"
)

// AllFamilies is every family the engine understands. The load refuses any
// other name: a misspelt family would let the record stack silently.
var AllFamilies = []string{FamilyWard, FamilyHeal}

// validateFamily refuses a family the engine does not declare.
func (b *ConditionSpec) validateFamily() error {
	if b.Family == "" || slices.Contains(AllFamilies, b.Family) {
		return nil
	}
	return fmt.Errorf("conditionId %d (%s) names unknown family %q; see conditions.AllFamilies", b.ConditionId, b.Name, b.Family)
}

// HasFamily reports whether any held, unexpired record belongs to family.
// The mob AI's "already shielded" and "already healing" test.
func (bs *Conditions) HasFamily(family string) bool {
	if family == "" {
		return false
	}
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && spec.Family == family {
			return true
		}
	}
	return false
}

// FamilyRivals lists the held, unexpired records an add of conditionId would
// replace: every other id of the same family. Empty for a spec with no family
// and for the same id (a refresh). The apply hook reads it BEFORE the add, to
// tell the holder which ward or heal gave way.
func (bs *Conditions) FamilyRivals(conditionId int) []*Condition {
	spec := GetConditionSpec(conditionId)
	if spec == nil || spec.Family == "" {
		return nil
	}
	var out []*Condition
	for _, b := range bs.List {
		if b.ConditionId == conditionId || b.Expired() {
			continue
		}
		if other := GetConditionSpec(b.ConditionId); other != nil && other.Family == spec.Family {
			out = append(out, b)
		}
	}
	return out
}

// discardFamilyRivals deletes every other live record of spec's family with
// no end line (Discard): the hook tells the replacement line in its place. An
// expired rival is left alone for the prune pass, so its own end line still
// fires. Called by the add primitives after their refusal checks, so a refused
// add (poison immunity) keeps the rival. A permanent record (AddCondition(id,
// true)) in a family is discarded like any other, so do not tag a family on a
// spec that items or mutations grant permanently.
func (bs *Conditions) discardFamilyRivals(spec *ConditionSpec) {
	if spec.Family == "" {
		return
	}
	for _, b := range slices.Clone(bs.List) {
		if b.ConditionId == spec.ConditionId || b.Expired() {
			continue
		}
		if other := GetConditionSpec(b.ConditionId); other != nil && other.Family == spec.Family {
			bs.Discard(b.ConditionId)
		}
	}
}
