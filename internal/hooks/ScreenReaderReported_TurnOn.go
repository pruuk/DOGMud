package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// TurnOnReportedScreenReader turns on the screen reader setting of a user
// whose reconnecting connection reported a screen reader, on MainWorker,
// and tells them so. The setting is never turned off here.
func TurnOnReportedScreenReader(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.ScreenReaderReported)
	if !ok {
		mudlog.Error("Event", "Expected Type", "ScreenReaderReported", "Actual Type", e.Type())
		return events.Cancel
	}
	u := users.GetByUserId(evt.UserId)
	if u == nil {
		return events.Continue
	}
	for _, line := range u.TurnOnReportedScreenReader() {
		u.SendText(messaging.CategorySystem, line)
	}
	return events.Continue
}
