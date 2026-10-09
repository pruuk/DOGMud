package conditions

// Record ids the engine names in code. The YAML under
// _datafiles/world/dogmud/conditions/ is the definition; these constants exist so
// a producer or reader never spells a bare number. Slice 1 of the conditions
// unification (2026-09-12) added 117 to 123 when the ten combat conditions
// became records.
const (
	// ConditionIdEmpathicShroud is the Empathic Shroud spell's hide. It is a
	// real hide (#444): characters.hideForStealthRecord enters Hidden on it.
	ConditionIdEmpathicShroud    = 31
	ConditionIdWarcry            = 79
	ConditionIdRally             = 80
	ConditionIdOffBalance        = 117 // failed grapple exposure, one round
	ConditionIdRecovering        = 118 // prone recovery penalty, one round
	ConditionIdMinorShield       = 119
	ConditionIdRegenerating      = 120
	ConditionIdPoisoned          = 121 // the spell dot; Venom (39) and Spore Toxin (40) are their own records
	ConditionIdBleeding          = 122
	ConditionIdEnchantWithdrawal = 123
)
