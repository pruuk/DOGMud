package conditions

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestCatsEyeDraughtGrantsNightVision guards a flag spelling. The draught's
// condition listed `night-vision`, but the constant every sight check reads is
// `nightvision`, and nothing normalises or validates condition flag names at load.
// So the draught granted no night vision at all, while its item description
// promised that "the darkness becomes transparent".
func TestCatsEyeDraughtGrantsNightVision(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/conditions/65-cats_eye_draught.yaml")
	require.NoError(t, err)
	var spec ConditionSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))
	require.Equal(t, 65, spec.ConditionId, "read the wrong file, so this test proves nothing")
	assert.Contains(t, spec.Flags, NightVision)
}

// TestEmpathicShroudBreaksOnCombat: Empathic Shroud is a real hide (#444) and
// breaks on combat the way the sneak's record 9 does. Without cancel-on-combat
// the combat strip (CancelCombatConditions) leaves 31 live on a revealed
// holder, and the next Validate hides them again mid-fight.
func TestEmpathicShroudBreaksOnCombat(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/conditions/31-empathic_shroud.yaml")
	require.NoError(t, err)
	var spec ConditionSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))
	require.Equal(t, ConditionIdEmpathicShroud, spec.ConditionId, "read the wrong file, so this test proves nothing")
	assert.Contains(t, spec.Flags, Hidden)
	assert.Contains(t, spec.Flags, CancelIfCombat)
}
