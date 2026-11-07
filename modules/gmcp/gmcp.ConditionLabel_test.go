package gmcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The status panel shows only this word, so permanence must come from the
// record's flag. A timed record in its last round has 0 rounds left and used
// to read "sustained", the word for a permanent record (owner, 2026-11-06).
func TestConditionDurationLabel_PermanenceIsTheFlagNotZeroRounds(t *testing.T) {
	assert.Equal(t, "sustained", conditionDurationLabel(true, 0), "a permanent record is sustained")
	assert.Equal(t, "sustained", conditionDurationLabel(true, 50), "a permanent record is sustained whatever its counter")
	assert.Equal(t, "briefly", conditionDurationLabel(false, 0), "a timed record in its last round is briefly, not sustained")
	assert.Equal(t, "briefly", conditionDurationLabel(false, 3))
	assert.Equal(t, "for a while", conditionDurationLabel(false, 4))
	assert.Equal(t, "for a while", conditionDurationLabel(false, 10))
	assert.Equal(t, "extended", conditionDurationLabel(false, 11))
}
