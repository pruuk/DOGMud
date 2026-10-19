package mobcommands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// #631: the room read "Ordel Quist smoothly sweeps aside the fumbled hamstring
// slash." about a move every hamstring line narrates as a low lunge at the
// legs. The defence label must be words the move's own store already shows
// the reader, so the defence line and the move line describe the same thing.
func TestHamstringDefenceLabelUsesTheStoresOwnWords(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	f := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "narration", "special-moves", "hamstring.yaml")
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read %s: %v", f, err)
	}
	store := strings.ToLower(string(raw))
	for _, word := range strings.Fields(hamstringAttack) {
		if !strings.Contains(store, word) {
			t.Errorf("hamstringAttack %q: the word %q appears in no hamstring line in %s", hamstringAttack, word, f)
		}
	}
}
