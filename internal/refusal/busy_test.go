package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestWhileBusy(t *testing.T) {
	f := &fakeRecipient{}
	WhileBusy(f, "change equipment")
	assert.Equal(t, `You can't change equipment while you're busy. Try <ansi fg="command">cancel</ansi>.`,
		one(t, f, messaging.CategoryRefusal))
	for _, doing := range []string{"change equipment", "use that ability", "muster a warcry", "throw anything", "pick a lock", "do that"} {
		fits80(t, WhileBusyText(doing))
	}
}

func TestWhileUnderAttack(t *testing.T) {
	f := &fakeRecipient{}
	WhileUnderAttack(f, "steal")
	assert.Equal(t, "You can't steal while under attack.", one(t, f, messaging.CategoryRefusal))
}

func TestTargetAsleepText(t *testing.T) {
	assert.Equal(t, `<ansi fg="mobname">Mira</ansi> is fast asleep. You could make some noise. Try <ansi fg="command">shout wake up</ansi>.`,
		TargetAsleepText("Mira"))
	fits80(t, TargetAsleepText("Bartholomew Finch"))
}
