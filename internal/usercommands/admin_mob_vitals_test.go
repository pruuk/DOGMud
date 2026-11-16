package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// Movement parity 4b: a playtester watches a mob tire through `mob schedule`.
// Stamina is the whole price of a step since #401, so it is the only line.
func TestMobVitalsLine(t *testing.T) {
	m := &mobs.Mob{InstanceId: 5}
	m.Character.Stamina = 37
	m.Character.StaminaMax.Value = 120
	line := mobVitalsLine(m)
	require.True(t, strings.Contains(line, "stamina:         37 / 120"), line)
	require.False(t, strings.Contains(line, "action points"), line)
}
