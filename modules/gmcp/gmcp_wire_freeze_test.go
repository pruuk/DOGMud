package gmcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The web client and Mudlet packages read these GMCP JSON field names. Slice 3
// of the conditions unification renamed the builder field names; this pins
// them as renamed. Quest enums carry both the trigger-condition vocabulary
// (`conditions`) and the condition id picker (`statusConditions`). See
// internal/guards/wire_freeze_test.go.
func TestWireFreeze_GMCPJSONFieldNames(t *testing.T) {
	keysOf := func(v any) map[string]any {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		m := map[string]any{}
		require.NoError(t, json.Unmarshal(b, &m))
		return m
	}
	assert.Contains(t, keysOf(itemUpdateReq{}), "conditionIds")
	assert.Contains(t, keysOf(itemUpdateReq{}), "wornConditionIds")
	assert.Contains(t, keysOf(mobUpdateReq{}), "conditionIds")
	assert.Contains(t, keysOf(mobEnums{}), "conditions")
	assert.Contains(t, keysOf(questEnums{}), "conditions")
	assert.Contains(t, keysOf(questEnums{}), "statusConditions")
	assert.Contains(t, keysOf(GMCPCondition{}), "duration_cur")
}
