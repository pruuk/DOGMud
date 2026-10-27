package questengine_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/assert"
)

// The bridge keys a count by the player's current step of the quest.
func TestGameBridge_BumpStepCount(t *testing.T) {
	u := newBridgeUser(1)
	u.Character.QuestProgress = map[int]string{53: "hunt"}
	var sc questengine.StepCounter = questengine.NewGameBridge(u, 1)
	assert.Equal(t, 1, sc.BumpStepCount(53))
	assert.Equal(t, 2, sc.BumpStepCount(53))
	assert.Equal(t, 2, u.Character.QuestStepCount("53-hunt"))
}
