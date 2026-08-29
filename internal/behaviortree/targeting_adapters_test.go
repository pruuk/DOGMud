package behaviortree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRegistryNamesUnchanged pins the authored surface. Behaviour YAML
// references these names; renaming one silently breaks every tree using it.
func TestRegistryNamesUnchanged(t *testing.T) {
	for _, name := range []string{
		"attack",
		"target_random_player_in_room",
		"target_weakest_mob_in_room",
	} {
		_, ok := actionRegistry[name]
		assert.True(t, ok, "actionRegistry must still contain %q", name)
	}
}

// TestTargetSettersStayUndelayed pins actions.go:87-91. The target-setters are
// deliberately absent from delayedActions: a perception delay would open a
// window where idle ticks re-fire before the target takes effect.
func TestTargetSettersStayUndelayed(t *testing.T) {
	for _, name := range []string{
		"target_random_player_in_room",
		"target_weakest_mob_in_room",
	} {
		assert.False(t, delayedActions[name],
			"%q must not be perception-delayed", name)
	}
	assert.True(t, delayedActions["attack"], "attack stays delayed")
}
