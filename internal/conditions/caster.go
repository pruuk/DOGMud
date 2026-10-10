package conditions

import "github.com/GoMudEngine/GoMud/internal/state"

// Stamp records who applied a held record and from what, after an add door
// landed it (messaging M6 slice 1, section 1). The newest application owns
// the record: a re-application by a different caster takes it over, and a
// re-application with a zero caster (a potion re-applying a spell's record)
// leaves it with none. Source is stamped beside it, so every door, not only
// the magnitude one, carries the applier's source. A record not held is left
// alone.
func (bs *Conditions) Stamp(conditionId int, source string, caster state.ActorRef) {
	idx, ok := bs.conditionIds[conditionId]
	if !ok {
		return
	}
	bs.List[idx].Source = source
	bs.List[idx].Caster = caster
}

// plainCondition is Condition without its YAML methods, so MarshalYAML and
// UnmarshalYAML can hand the struct to the encoder without recursing.
type plainCondition Condition

// MarshalYAML writes a record with its caster's player half only. A mob's
// InstanceId is runtime only (mobs.Mob.InstanceId is `yaml:"-"`), so after a
// restart the same number names a different creature, or none: a saved mob
// caster would credit a stranger. A record whose caster was a mob is saved
// with no caster.
func (b Condition) MarshalYAML() (interface{}, error) {
	p := plainCondition(b)
	p.Caster.MobInstanceId = 0
	return p, nil
}

// UnmarshalYAML loads a record and drops any mob caster it carries, for a
// save written before MarshalYAML stripped it. The yaml.v2 form, which
// yaml.v3 also honours.
func (b *Condition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var p plainCondition
	if err := unmarshal(&p); err != nil {
		return err
	}
	*b = Condition(p)
	b.Caster.MobInstanceId = 0
	return nil
}
