package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #249: a poison shows as "poisoned" beside a name and a bleed showed as
// nothing. The bleeding flag reads "bleeding": Blood Boil's 143 and a combat
// bleed (122) alike (owner call 2026-10-10).
func TestGetAdjectives_BleedingFlagReadsBleeding(t *testing.T) {
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	c := New()
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdBleeding, 4, -3, "test"))

	adj := c.GetAdjectives()
	assert.Contains(t, adj, "bleeding")
	assert.NotContains(t, adj, "poisoned")
}
