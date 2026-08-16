package gmcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCharVitals_ZeroPoolsAreSentAsZero guards the whole Char.Vitals payload
// against `omitempty` coming back.
//
// Char.Vitals is a full-state snapshot, republished whenever a pool moves.
// Under `omitempty` a pool at exactly 0 dropped out of the JSON entirely, so a
// client that merges each payload over the last one kept rendering the
// previous non-zero reading at the precise moment the number mattered most. A
// playtest caught it live: the payload arrived as
// `{"hp":118,"hp_max":437,"stamina_max":429,...}` with no `stamina` key at all
// while stamina was genuinely exhausted.
//
// The assertion is deliberately on key PRESENCE rather than on value, because
// presence is the property that broke. A value check would pass against a
// re-tagged struct.
func TestCharVitals_ZeroPoolsAreSentAsZero(t *testing.T) {
	// Every numeric field left at its zero value: the worst case, and the one
	// `omitempty` erases completely.
	raw, err := json.Marshal(GMCPCharModule_Payload_Vitals{Toxicity: "clear"})
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	for _, key := range []string{
		"hp", "hp_max",
		"stamina", "stamina_max",
		"conviction", "conviction_max",
		"hp_reserved", "stamina_reserved", "conviction_reserved",
		"toxicity",
	} {
		val, present := decoded[key]
		assert.True(t, present,
			"%q must be present even at zero; do not re-add omitempty to it", key)
		if key != "toxicity" && present {
			assert.Equal(t, float64(0), val, "%q should serialise as 0", key)
		}
	}

	assert.Len(t, decoded, 10, "no field in Char.Vitals may be omitted")
}
