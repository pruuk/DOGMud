package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestInputQueueFull(t *testing.T) {
	f := &fakeRecipient{}
	InputQueueFull(f)
	assert.Equal(t, `You're already doing as much as you can. Wait a moment.`, one(t, f, messaging.CategoryRefusal))
	fits80(t, InputQueueFullLine)
}

func TestAutomationLimit(t *testing.T) {
	f := &fakeRecipient{}
	AutomationLimit(f, "triggers", 100)
	assert.Equal(t, `You can keep at most 100 triggers. Remove one first.`, one(t, f, messaging.CategoryRefusal))
	fits80(t, AutomationLimitText("triggers", 1000000))
}

func TestCommandChainTooLong(t *testing.T) {
	f := &fakeRecipient{}
	CommandChainTooLong(f, "trigger", 10)
	assert.Equal(t, `A trigger can hold at most 10 commands. Split a long route into two.`, one(t, f, messaging.CategoryRefusal))
	fits80(t, CommandChainTooLongText("trigger", 1000))
}

func TestAutomationTooLong(t *testing.T) {
	f := &fakeRecipient{}
	AutomationTooLong(f, "trigger")
	assert.Equal(t, `That trigger is too long to save. Shorten it and try again.`, one(t, f, messaging.CategoryRefusal))
	fits80(t, AutomationTooLongText("trigger"))
}
