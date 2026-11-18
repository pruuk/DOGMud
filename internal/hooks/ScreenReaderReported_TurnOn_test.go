package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The hook turns a logged-in user's setting on from MainWorker, tells them,
// and does nothing the second time or for an unknown user.
func TestTurnOnReportedScreenReader(t *testing.T) {
	drain := func() {
		events.DrainQueuedByTypeForTest(`UserSettingChanged`)
		events.DrainQueuedByTypeForTest(`Message`)
	}
	drain()
	t.Cleanup(drain)

	u := users.NewTestUser(51, "sr", "Sr", 0)
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{51: u})
	t.Cleanup(cleanup)

	require.Equal(t, events.Continue, TurnOnReportedScreenReader(events.ScreenReaderReported{UserId: 51}))
	require.True(t, u.ScreenReader)
	require.Equal(t, `light`, u.CombatVerbosity)

	var texts []string
	for _, ev := range events.DrainQueuedByTypeForTest(`Message`) {
		if m, ok := ev.(events.Message); ok && m.UserId == 51 {
			texts = append(texts, m.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	for _, notice := range users.ScreenReaderOnNotices {
		require.Contains(t, joined, notice)
	}
	events.DrainQueuedByTypeForTest(`UserSettingChanged`)

	require.Equal(t, events.Continue, TurnOnReportedScreenReader(events.ScreenReaderReported{UserId: 51}))
	require.Empty(t, events.DrainQueuedByTypeForTest(`UserSettingChanged`), "already on: nothing changes")

	require.Equal(t, events.Continue, TurnOnReportedScreenReader(events.ScreenReaderReported{UserId: 99999}))
	require.Empty(t, events.DrainQueuedByTypeForTest(`UserSettingChanged`))
	require.Empty(t, events.DrainQueuedByTypeForTest(`Message`))
}
