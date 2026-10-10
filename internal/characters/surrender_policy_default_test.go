package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #284: a save with no surrender_policy key loaded {AutoTap, 0}, which no
// player can choose (the parser takes 1-100) and which status printed as
// "auto-tap-below 0". Validate gives it the new-character default.
func TestValidate_MissingSurrenderPolicyGetsTheDefault(t *testing.T) {
	c := New()
	c.SurrenderPolicy = SurrenderPolicy{}
	require.NoError(t, c.Validate())
	assert.Equal(t, DefaultPlayerSurrenderPolicy, c.SurrenderPolicy)
	assert.Equal(t, "auto-tap-below 15", c.SurrenderPolicy.String())
}

// A policy the player chose is kept.
func TestValidate_ChosenSurrenderPolicyIsKept(t *testing.T) {
	c := New()
	c.SurrenderPolicy = SurrenderPolicy{Mode: SurrenderNever}
	require.NoError(t, c.Validate())
	assert.Equal(t, SurrenderPolicy{Mode: SurrenderNever}, c.SurrenderPolicy)
}
