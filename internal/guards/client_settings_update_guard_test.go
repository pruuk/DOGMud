package guards

import (
	"fmt"
	"testing"
)

// TestClientSettingsChangeOnlyThroughUpdate: production code changes a
// connection's settings only through connections.UpdateClientSettings, which
// applies the change under the connections lock. A GetClientSettings, change,
// OverwriteClientSettings sequence is not atomic: a connection goroutine (NAWS,
// MTTS, MSP) and MainWorker (Client.Display, set charset) could each lose the
// other's write (#910). OverwriteClientSettings stays for test setup, and its
// own definition is the only production mention allowed.
func TestClientSettingsChangeOnlyThroughUpdate(t *testing.T) {
	calls, files := walkGoCalls(t, map[string]bool{
		"OverwriteClientSettings": true,
		"UpdateClientSettings":    true,
	})
	updates := 0
	var problems []string
	for _, c := range calls {
		switch c.name {
		case "UpdateClientSettings":
			updates++
		case "OverwriteClientSettings":
			problems = append(problems, fmt.Sprintf("%s:%d calls OverwriteClientSettings; use connections.UpdateClientSettings", c.file, c.line))
		}
	}
	if files < 100 || updates < 10 {
		t.Fatalf("scanned %d files and found %d UpdateClientSettings calls; the walk is not reaching the tree, so a clean result proves nothing", files, updates)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
