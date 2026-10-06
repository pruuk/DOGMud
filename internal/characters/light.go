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

// EmitsLight reports whether this character sheds any light right now. A
// shut hood or a source trimmed to nothing does not count. It replaced the
// retired lightsource flag.
func (c *Character) EmitsLight() bool {
	return len(c.LightTerms()) > 0
}
