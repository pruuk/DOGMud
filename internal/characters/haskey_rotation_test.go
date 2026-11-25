package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

// HasKey judges a known combination against the lock's current rotation
// (#781): SetLocked bumps RotationSeed, so the combination learned before the
// re-lock stops counting and the one at the new rotation counts. A physical
// key does not depend on the rotation.
func TestHasKey_JudgesTheCombinationAtTheLocksCurrentRotation(t *testing.T) {
	const lockId = `5-north`
	lock := gamelock.Lock{Difficulty: 4}
	t.Cleanup(util.RebootLocksUntilRotatedForTest(4, lockId))

	c := New()
	c.SetKey(`key-`+lockId, `20001`)
	c.SetKey(lockId, util.LockSequence(lockId, 4, 0))

	hasKey, hasSequence := c.HasKey(lockId, lock)
	assert.True(t, hasKey)
	assert.True(t, hasSequence, "the combination at rotation 0 does not count on an unrotated lock")

	lock.SetLocked()
	assert.Equal(t, uint64(1), lock.RotationSeed, "premise: SetLocked rotated the lock")

	hasKey, hasSequence = c.HasKey(lockId, lock)
	assert.True(t, hasKey, "the physical key stopped counting after the rotation")
	assert.False(t, hasSequence, "the combination from rotation 0 still counts after the rotation")

	c.SetKey(lockId, util.LockSequence(lockId, 4, 1))
	_, hasSequence = c.HasKey(lockId, lock)
	assert.True(t, hasSequence, "the combination at the current rotation does not count")
}
