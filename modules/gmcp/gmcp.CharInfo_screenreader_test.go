package gmcp

import (
	"encoding/json"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// false must reach the client too, so another device turns its mode off.
func TestCharInfo_ScreenReaderAlwaysSent(t *testing.T) {
	b, err := json.Marshal(GMCPCharModule_Payload_Info{})
	require.NoError(t, err)
	require.Contains(t, string(b), `"screenreader":false`)

	b, err = json.Marshal(GMCPCharModule_Payload_Info{ScreenReader: true})
	require.NoError(t, err)
	require.Contains(t, string(b), `"screenreader":true`)
}

// set screenreader re-pushes Char.Info; other settings do not.
func TestCharInfo_PushedWhenScreenReaderChanges(t *testing.T) {
	events.DrainQueuedByTypeForTest(`GMCPCharUpdate`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPCharUpdate`) })
	g := &GMCPCharModule{}

	g.userSettingChangedHandler(events.UserSettingChanged{UserId: 5, Name: `tips`})
	require.Empty(t, events.DrainQueuedByTypeForTest(`GMCPCharUpdate`))

	g.userSettingChangedHandler(events.UserSettingChanged{UserId: 5, Name: `ScreenReader`})
	queued := events.DrainQueuedByTypeForTest(`GMCPCharUpdate`)
	require.Len(t, queued, 1)
	require.Equal(t, GMCPCharUpdate{UserId: 5, Identifier: `Char.Info`}, queued[0])

	g.userSettingChangedHandler(events.UserSettingChanged{UserId: 0, Name: `screenreader`})
	require.Empty(t, events.DrainQueuedByTypeForTest(`GMCPCharUpdate`))
}
