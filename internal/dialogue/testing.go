package dialogue

import "fmt"

// InstallForTest puts df in the load cache as if read from disk, so tests in
// other packages can give a mob a dialogue file. It returns the undo.
func InstallForTest(df *DialogueFile) func() {
	key := fmt.Sprintf("%d:%s", df.MobId, df.Zone)
	prev, had := dialogueCache[key]
	prevNil := nilSentinel[key]
	dialogueCache[key] = df
	delete(nilSentinel, key)
	return func() {
		if had {
			dialogueCache[key] = prev
		} else {
			delete(dialogueCache, key)
		}
		if prevNil {
			nilSentinel[key] = true
		}
	}
}
