package quests

// The newcomer flag on The Awakening. Character creation sets it in code for a
// player new to MUDs, and no topic or trigger does, so dialoguecheck cannot find
// the setter in the world data.
const (
	NewcomerFlagKey   = "30-newcomer"
	NewcomerFlagValue = "new"
)

// EngineSetFlags returns the quest flag values only Go code sets, as key=value.
// The dialogue walker counts them as set. A flag set in code belongs here, or the
// topics it gates read as unreachable.
func EngineSetFlags() []string {
	return []string{NewcomerFlagKey + "=" + NewcomerFlagValue}
}
