package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// readOnOneLineForTest gives user a reader width no test line reaches, for a
// test about a message's wording that would otherwise compare across a fold.
func readOnOneLineForTest(t *testing.T, user *users.UserRecord) {
	t.Helper()
	prev := user.LineWidth
	user.LineWidth = 500
	t.Cleanup(func() { user.LineWidth = prev })
}
