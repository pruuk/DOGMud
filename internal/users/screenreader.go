package users

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// ScreenReaderCombatLines tell a player whose screen reader setting just
// turned on that combat text was set to light, and how to undo it. xterm's
// screen reader mode stops announcing after 20 lines of one burst, so full
// combat text would be cut off. Two lines, each under 80 columns, because
// login text reaches telnet unwrapped.
var ScreenReaderCombatLines = []string{
	`Combat text is set to light, so your screen reader can keep up.`,
	`Type set combatverbosity full to see every blow.`,
}

// ScreenReaderOnNotices tell a player that their connection turned the
// account's screen reader setting on, and how to turn it off. Two lines,
// each under 80 columns, because login text reaches telnet unwrapped.
var ScreenReaderOnNotices = []string{
	`Screen reader mode is on for this character.`,
	`Type set screenreader to turn it off.`,
}

// TurnOnReportedScreenReader turns u's setting on for a connection that
// reported a screen reader, and returns the lines to tell the player: the
// notices, then the combat lines when combat text changed. It returns none
// when the setting was already on. A caller must own u: before login
// publishes it, or on MainWorker.
func (u *UserRecord) TurnOnReportedScreenReader() []string {
	if u.ScreenReader {
		return nil
	}
	lines := slices.Clone(ScreenReaderOnNotices)
	if u.TurnOnScreenReader() {
		lines = append(lines, ScreenReaderCombatLines...)
	}
	return lines
}

// TurnOnScreenReader is the only way an account's screen reader setting
// turns on (the creation answer, a connection's report at login, and set
// screenreader). It sets the flag and saves light combat text, firing a
// UserSettingChanged for each. It reports whether the combat level changed,
// so the caller can send ScreenReaderCombatLines. When the setting is
// already on it does nothing and reports false. Both values are saved with
// the account at its next save (autosave, logout or creation), as every set
// command's are. At creation the user has no id yet (UserId 0), so the
// events are dropped by their listeners; the full Char push at spawn carries
// the setting to the client instead.
func (u *UserRecord) TurnOnScreenReader() bool {
	if u.ScreenReader {
		return false
	}
	u.ScreenReader = true
	events.AddToQueue(events.UserSettingChanged{UserId: u.UserId, Name: `screenreader`})
	if u.GetCombatVerbosity() == messaging.VerbosityLight {
		return false
	}
	u.CombatVerbosity = messaging.VerbosityLight.String()
	events.AddToQueue(events.UserSettingChanged{UserId: u.UserId, Name: `combatverbosity`})
	return true
}
