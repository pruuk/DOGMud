package buffs

// Record ids the engine names in code. The YAML under
// _datafiles/world/dogmud/buffs/ is the definition; these constants exist so
// a producer or reader never spells a bare number. Slice 1 of the conditions
// unification (2026-09-12) added 117 to 123 when the ten combat conditions
// became records.
const (
	BuffIdWarcry            = 79
	BuffIdRally             = 80
	BuffIdOffBalance        = 117 // failed grapple exposure, one round
	BuffIdRecovering        = 118 // prone recovery penalty, one round
	BuffIdMinorShield       = 119
	BuffIdRegenerating      = 120
	BuffIdPoisoned          = 121 // the spell dot; Venom (39) and Spore Toxin (40) are their own records
	BuffIdBleeding          = 122
	BuffIdEnchantWithdrawal = 123
)
