package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

func drainSettingChanges(t *testing.T) []string {
	t.Helper()
	names := []string{}
	for _, e := range events.DrainQueuedByTypeForTest(`UserSettingChanged`) {
		names = append(names, e.(events.UserSettingChanged).Name)
	}
	return names
}

// Turning the setting on saves light combat text from full (stored empty)
// and from medium, and fires both setting changes.
func TestTurnOnScreenReader_SavesLightCombat(t *testing.T) {
	for _, start := range []string{``, `medium`} {
		drainSettingChanges(t)
		u := &UserRecord{UserId: 7, CombatVerbosity: start}
		require.True(t, u.TurnOnScreenReader(), start)
		require.True(t, u.ScreenReader)
		require.Equal(t, `light`, u.CombatVerbosity)
		// The drain returns heap order, not delivery order (events.go:893).
		require.ElementsMatch(t, []string{`screenreader`, `combatverbosity`}, drainSettingChanges(t))
	}
}

// Already light: the setting turns on and the combat level is not touched.
func TestTurnOnScreenReader_AlreadyLight(t *testing.T) {
	drainSettingChanges(t)
	u := &UserRecord{UserId: 7, CombatVerbosity: `light`}
	require.False(t, u.TurnOnScreenReader())
	require.True(t, u.ScreenReader)
	require.Equal(t, []string{`screenreader`}, drainSettingChanges(t))
}

// A reported screen reader: the notices, then the combat lines; nothing when
// the setting is already on.
func TestTurnOnReportedScreenReader(t *testing.T) {
	drainSettingChanges(t)
	u := &UserRecord{UserId: 7}
	require.Equal(t, append(append([]string{}, ScreenReaderOnNotices...), ScreenReaderCombatLines...), u.TurnOnReportedScreenReader())
	require.True(t, u.ScreenReader)
	light := &UserRecord{UserId: 7, CombatVerbosity: `light`}
	require.Equal(t, ScreenReaderOnNotices, light.TurnOnReportedScreenReader(), "already light: no combat lines")
	require.Empty(t, u.TurnOnReportedScreenReader(), "already on: nothing")
	drainSettingChanges(t)
}

// Already on: nothing changes and nothing fires.
func TestTurnOnScreenReader_AlreadyOn(t *testing.T) {
	drainSettingChanges(t)
	u := &UserRecord{UserId: 7, ScreenReader: true}
	require.False(t, u.TurnOnScreenReader())
	require.Equal(t, ``, u.CombatVerbosity)
	require.Empty(t, drainSettingChanges(t))
}
