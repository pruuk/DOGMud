package buffs

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestCatsEyeDraughtGrantsNightVision guards a flag spelling. The draught's
// buff listed `night-vision`, but the constant every sight check reads is
// `nightvision`, and nothing normalises or validates buff flag names at load.
// So the draught granted no night vision at all, while its item description
// promised that "the darkness becomes transparent".
func TestCatsEyeDraughtGrantsNightVision(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml")
	require.NoError(t, err)
	var spec BuffSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))
	require.Equal(t, 65, spec.BuffId, "read the wrong file, so this test proves nothing")
	assert.Contains(t, spec.Flags, NightVision)
}
