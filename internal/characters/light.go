package characters

import "github.com/GoMudEngine/GoMud/internal/conditions"

// LightTerms is every light-scale term this character adds to its room right
// now, one per held light record (lighting plan 5a).
func (c *Character) LightTerms() []float64 {
	var out []float64
	for _, rec := range c.Conditions.LightSources() {
		if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); ok {
			out = append(out, v)
		}
	}
	return out
}

// DarknessTerms is every darkness term this character takes from its room
// right now, one per held darkness record (lighting plan 5d). LightTerms'
// twin: a darkness is never one of LightTerms, so it never makes a
// character EmitsLight.
func (c *Character) DarknessTerms() []float64 {
	var out []float64
	for _, rec := range c.Conditions.DarknessSources() {
		if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); ok {
			out = append(out, v)
		}
	}
	return out
}

// EmitsLight reports whether this character sheds any light right now. A
// shut hood or a source trimmed to nothing does not count. It replaced the
// retired lightsource flag.
func (c *Character) EmitsLight() bool {
	return len(c.LightTerms()) > 0
}
