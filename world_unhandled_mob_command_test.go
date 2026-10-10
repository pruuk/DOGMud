package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/stretchr/testify/assert"
)

// #302: an unhandled mob command used to emote "looks a little confused
// (east )" to the room. It is now a warn log, once per mob and command.
func TestNoteUnhandledMobCommand_LogsOncePerMobAndCommand(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	unhandledMobCommands.Clear()
	t.Cleanup(unhandledMobCommands.Clear)

	assert.True(t, noteUnhandledMobCommand(4242, "east", ""), "first time is logged")
	assert.False(t, noteUnhandledMobCommand(4242, "east", ""), "repeat is not")
	assert.True(t, noteUnhandledMobCommand(4242, "west", ""), "another command is logged")
	assert.True(t, noteUnhandledMobCommand(4343, "east", ""), "another mob is logged")
}
