package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// M6 slice 3 (ledger row 35): a parry counter is sent for any parrying weapon,
// a mace or a staff as much as a blade, so the lines may not name a cut, a
// point or a thrust.
var bladeOnlyWord = regexp.MustCompile(`(?i)\b(cut|cuts|point|points|thrust|thrusts)\b`)

func bladeWordsIn(text string) []string {
	var hits []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if m := bladeOnlyWord.FindString(line); m != "" {
			hits = append(hits, m+" in: "+strings.TrimSpace(line))
		}
	}
	return hits
}

func TestParryCountersNameNoBladeOnlyWord(t *testing.T) {
	if len(bladeWordsIn("# comment cut\n- 'You cut back.'\n")) != 1 || len(bladeWordsIn("- 'You strike back.'\n# thrust\n")) != 0 {
		t.Fatal("bladeWordsIn cannot tell a blade line from a clean one; the guard would prove nothing")
	}
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(here), "_datafiles", "world", "dogmud", "defense-messages", "counter-parry.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "actee:") {
		t.Fatal("counter-parry.yaml has no actee lines; the guard would check nothing")
	}
	for _, h := range bladeWordsIn(string(raw)) {
		t.Errorf("counter-parry.yaml names a blade-only word: %s", h)
	}
}
