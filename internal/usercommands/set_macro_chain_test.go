package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A macro holds at most Network.MaxPendingInputs commands, the one limit
// every command chain shares; blank pieces do not count, and are not kept.
func TestSetMacro_ChainIsCappedByThePendingInputLimit(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 3
	configs.SetConfigForTest(t, cfg)

	u := users.NewTestUser(98720, "macroset", "Macroset", 0)
	events.DrainQueuedMessagesForTest(u.UserId)
	t.Cleanup(func() { events.DrainQueuedMessagesForTest(u.UserId) })

	_, err := cmdSetMacro(u, `=1`, []string{`n;e; ;s;`})
	require.NoError(t, err)
	require.Equal(t, `n;e;s`, u.Macros[`=1`], "three commands fit; blank pieces are dropped")
	events.DrainQueuedMessagesForTest(u.UserId)

	_, err = cmdSetMacro(u, `=2`, []string{`n;e;s;w`})
	require.NoError(t, err)
	require.NotContains(t, u.Macros, `=2`, "four commands are refused")
	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], refusal.CommandChainTooLongText(`macro`, 3))
}

// A macro may not name a macro, spaces before the = included: the game trims
// a line before it looks for a macro, so " =1" would make =1 call itself.
// Pieces are stored trimmed.
func TestSetMacro_NamingAMacroIsRefusedEvenAfterSpaces(t *testing.T) {
	u := users.NewTestUser(98721, "macroself", "Macroself", 0)
	events.DrainQueuedMessagesForTest(u.UserId)
	t.Cleanup(func() { events.DrainQueuedMessagesForTest(u.UserId) })

	_, err := cmdSetMacro(u, `=1`, []string{`look; =1`})
	require.NoError(t, err)
	require.NotContains(t, u.Macros, `=1`, "a macro naming a macro is refused")
	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], `You cannot reference macros inside of a macro`)

	_, err = cmdSetMacro(u, `=2`, []string{` look ;  north`})
	require.NoError(t, err)
	require.Equal(t, `look;north`, u.Macros[`=2`], "stored trimmed")
}
