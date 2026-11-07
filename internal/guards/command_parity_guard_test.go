package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/stretchr/testify/require"

	// The modules register commands of their own at init, as they do at boot.
	_ "github.com/GoMudEngine/GoMud/modules"
)

// TestCommandParityHasOnlyItsStandingReminder runs the boot's command parity
// audit on the real registries, modules included. Owner ruling 2026-11-07:
// every command that only players or only mobs have is on an allowlist with
// its reason, and sit and rest are one command under two names for both, so
// startup logs no parity warning. The single exception is throw, kept off
// the allowlist on purpose as the reminder that a mob throw is owed with the
// ranged-weapon system (see the note in internal/actions/divergences.go).
func TestCommandParityHasOnlyItsStandingReminder(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	gaps := actions.AuditCommandParity(usercommands.GetAllUserCommands(), mobcommands.GetAllMobCommands())
	require.Equal(t, []string{
		`user command "throw" has no mob equivalent and is not in the user-only allowlist`,
	}, gaps)
}

// The audit can fail: a one-sided command nobody allowlisted is reported.
func TestCommandParityReportsAnUnlistedGap(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	gaps := actions.AuditCommandParity([]string{`say`, `zz-unlisted`}, []string{`say`})
	require.Equal(t, []string{
		`user command "zz-unlisted" has no mob equivalent and is not in the user-only allowlist`,
	}, gaps)
}
