package stats

type Statistics struct {
	Strength   StatInfo `yaml:"strength,omitempty"`   // Muscular strength (damage)
	Dexterity  StatInfo `yaml:"dexterity,omitempty"`  // Speed and agility (dodging)
	Perception StatInfo `yaml:"perception,omitempty"` // Awareness and intelligence (noticing things, memory, deduction)
	Vitality   StatInfo `yaml:"vitality,omitempty"`   // Health and stamina (health capacity)
	Willpower  StatInfo `yaml:"willpower,omitempty"`  // Mental fortitude and conviction
	Charisma   StatInfo `yaml:"charisma,omitempty"`   // Force of personality and social influence
}

// When saving to a file, we don't need to write all the properties that we calculate.
// Just keep track of "Training" because that's not calculated.
type StatInfo struct {
	Training int `yaml:"training,omitempty"` // How much it's been trained with Training Points spending
	Value    int `yaml:"-"`                  // Final calculated value
	ValueAdj int `yaml:"-"`                  // Always equals Value now; see Recalculate
	Racial   int `yaml:"-"`                  // Value provided by racial benefits
	Base     int `yaml:"base,omitempty"`     // Base stat value
	Mods     int `yaml:"-"`                  // How much it's modded by equipment, spells, etc.
	// BaseAuthored reports that the YAML this stat was decoded from actually
	// carried a `base:` key, as opposed to leaving it out. Species hydration
	// (characters.Validate) needs the distinction: it fills an unset Base from
	// the species record, and `Base == 0` alone cannot tell "unset" apart from
	// "deliberately zero". Two mob stats legitimately fold to exactly zero.
	//
	// Runtime-only, and `base:` keeps its omitempty, so a Base of 0 written
	// back out is indistinguishable from an absent one on the next load. That
	// is fine for the paths that exist today (mob templates are re-read from
	// their authored YAML; instance saves carry Training, not Base; player
	// rolls are never zero) but it is the sharp edge to know about.
	BaseAuthored bool `yaml:"-"`
}

// UnmarshalYAML decodes exactly as the default decoder would, and additionally
// records whether a `base:` key was present.
//
// This is the gopkg.in/yaml.v2 Unmarshaler signature, because that is what
// internal/fileloader (and therefore every mob, species and user file) decodes
// with. The handful of yaml.v3 call sites do not reach a stat whose base could
// be an authored zero -- playtestprofiles.cloneUser round-trips a player, whose
// rolled Base is never zero.
//
// The local type strips the method set so the nested unmarshal does not recurse
// back into here; the probe then re-reads the same node just to see whether the
// key existed at all.
func (si *StatInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plainStatInfo StatInfo
	var raw plainStatInfo
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*si = StatInfo(raw)

	var probe struct {
		Base *int `yaml:"base"`
	}
	if err := unmarshal(&probe); err != nil {
		return err
	}
	si.BaseAuthored = probe.Base != nil
	return nil
}

func (si *StatInfo) SetMod(mod ...int) {
	if len(mod) == 0 {
		si.Mods = 0
		return
	}
	si.Mods = 0
	for _, m := range mod {
		si.Mods += m
	}
}

// Recalculate previously ran a soft-cap compression on ValueAdj above
// StatSoftCap. Removed 2026-08-02: HealthMax/StaminaMax/ConvictionMax/
// ActionPointsMax are StatInfo too and shared this method, so the
// compression was silently shrinking every resource pool by ~40%
// (e.g. a true 530 HP played as 322), and the curve actually amplified
// rather than diminished for values 151-163. ValueAdj is kept, always
// equal to Value, only so the ~189 existing call sites keep compiling;
// collapsing it into Value is planned follow-up work. Do not reintroduce
// compression here.
func (si *StatInfo) Recalculate() {
	si.Racial = si.Base
	si.Value = si.Racial + si.Training + si.Mods
	si.ValueAdj = si.Value
}
