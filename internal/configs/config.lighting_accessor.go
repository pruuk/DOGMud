package configs

// Lighting is every knob the graded light model reads, in one small struct.
//
// 🔑 This exists for a measured reason. GetBalanceConfig() copies a 424-field
// struct under two read locks and benchmarks at 99.75 ns, against 8.23 ns for
// the small GetTimingConfig(). Fifteen call sites were paying that copy to read
// a single int, and Room.LightLevel() is called from per-round loops. The knobs
// stay declared on Balance, so the yaml schema is unchanged; only the read path
// is narrowed.
type Lighting struct {
	BlindBelow            int
	DimBelow              int
	ExitsAbove            int
	DazzleAbove           int
	DefaultVisionStrength int
	// RealMinimum is Balance.LightRealMinimum: the least a room with any real
	// light reads before darkness (lighting plan 6).
	RealMinimum int

	DoublingStep  float64
	WorldLatitude float64
	EquinoxNoon   float64
	Starlight     float64
	MoonsFull     float64

	MoonWeightSwiftmoon float64
	MoonWeightWanderer  float64
	MoonWeightEye       float64

	SpellStrengthBase, SpellStrengthStatDivisor, SpellStrengthSkillDivisor float64
	SpellDurationBase, SpellDurationStatDivisor, SpellDurationSkillDivisor float64

	NightVisionSpellBase, NightVisionSpellStatDivisor, NightVisionSpellSkillDivisor float64
	InfraSpellBase, InfraSpellStatDivisor, InfraSpellSkillDivisor                   float64

	DarknessSpellStrengthBase, DarknessSpellStrengthStatDivisor, DarknessSpellStrengthSkillDivisor float64
	DarknessSpellDurationBase, DarknessSpellDurationStatDivisor, DarknessSpellDurationSkillDivisor float64

	InfraReachCap     int
	InfraPenaltyFloor float64
	// DarkCap is Balance.DarknessCombatPenalty, carried here because the
	// infravision dark cap (messaging.infraDarkCap) is expressed against it.
	DarkCap float64
}

// GetLightingConfig returns the lighting knobs without copying Balance.
func GetLightingConfig() Lighting {
	ensureConfigValidated()

	configDataLock.RLock()
	defer configDataLock.RUnlock()

	b := &configData.Balance
	return Lighting{
		BlindBelow:            int(b.LightBlindBelow),
		DimBelow:              int(b.LightDimBelow),
		ExitsAbove:            int(b.LightExitsAbove),
		DazzleAbove:           int(b.LightDazzleAbove),
		DefaultVisionStrength: int(b.LightDefaultVisionStrength),
		RealMinimum:           int(b.LightRealMinimum),

		DoublingStep:  float64(b.LightDoublingStep),
		WorldLatitude: float64(b.WorldLatitude),
		EquinoxNoon:   float64(b.LightEquinoxNoon),
		Starlight:     float64(b.LightStarlight),
		MoonsFull:     float64(b.LightMoonsFull),

		MoonWeightSwiftmoon: float64(b.LightMoonWeightSwiftmoon),
		MoonWeightWanderer:  float64(b.LightMoonWeightWanderer),
		MoonWeightEye:       float64(b.LightMoonWeightEye),

		SpellStrengthBase:         float64(b.LightSpellStrengthBase),
		SpellStrengthStatDivisor:  float64(b.LightSpellStrengthStatDivisor),
		SpellStrengthSkillDivisor: float64(b.LightSpellStrengthSkillDivisor),
		SpellDurationBase:         float64(b.LightSpellDurationBase),
		SpellDurationStatDivisor:  float64(b.LightSpellDurationStatDivisor),
		SpellDurationSkillDivisor: float64(b.LightSpellDurationSkillDivisor),

		NightVisionSpellBase:         float64(b.LightNightVisionSpellBase),
		NightVisionSpellStatDivisor:  float64(b.LightNightVisionSpellStatDivisor),
		NightVisionSpellSkillDivisor: float64(b.LightNightVisionSpellSkillDivisor),
		InfraSpellBase:               float64(b.LightInfraSpellBase),
		InfraSpellStatDivisor:        float64(b.LightInfraSpellStatDivisor),
		InfraSpellSkillDivisor:       float64(b.LightInfraSpellSkillDivisor),
		InfraReachCap:                int(b.LightInfraReachCap),
		InfraPenaltyFloor:            float64(b.LightInfraPenaltyFloor),
		DarkCap:                      float64(b.DarknessCombatPenalty),

		DarknessSpellStrengthBase:         float64(b.LightDarknessSpellStrengthBase),
		DarknessSpellStrengthStatDivisor:  float64(b.LightDarknessSpellStrengthStatDivisor),
		DarknessSpellStrengthSkillDivisor: float64(b.LightDarknessSpellStrengthSkillDivisor),
		DarknessSpellDurationBase:         float64(b.LightDarknessSpellDurationBase),
		DarknessSpellDurationStatDivisor:  float64(b.LightDarknessSpellDurationStatDivisor),
		DarknessSpellDurationSkillDivisor: float64(b.LightDarknessSpellDurationSkillDivisor),
	}
}
