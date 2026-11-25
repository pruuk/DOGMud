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
