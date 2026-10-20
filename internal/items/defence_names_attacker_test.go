package items

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// defenceLinesMissingAttacker returns every defender (actee) and room
// (observer) line in group that does not name the attacker, as
// "<band>.<role>[<index>]: <line>". A deflected pool's defender lines count
// too; its attacker lines speak to the attacker and are exempt.
func defenceLinesMissingAttacker(group DefenseMessageGroup) []string {
	var out []string
	check := func(where string, lines []string) {
		for i, l := range lines {
			if !strings.Contains(l, string(TokenActor)) {
				out = append(out, where+"["+strconv.Itoa(i)+"]: "+l)
			}
		}
	}
	for band, opts := range group.Options {
		check(string(band)+".actee", messageStrings(opts.Together.ToDefender))
		check(string(band)+".observer", messageStrings(opts.Together.ToRoom))
	}
	if group.Deflected != nil {
		check("deflected.actee", group.Deflected.Actee)
	}
	return out
}

// The detector must be able to fail: the #631 line named no attacker.
func TestDefenceLinesMissingAttacker_CanFail(t *testing.T) {
	g := DefenseMessageGroup{Options: DefenseIntensity{Heavy: {Together: DefenseTogetherMessages{
		ToDefender: MessageOptions{`You hardly need to move, the attack is so far off!`},
		ToRoom:     MessageOptions{`{actee} hardly needs to move, {actor}''s attack is so far off.`},
	}}}}
	if got := defenceLinesMissingAttacker(g); len(got) != 1 {
		t.Fatalf("want exactly the nameless defender line flagged, got %v", got)
	}
}

// #631 (owner ruling 2026-10-19): every line that tells the defender or the
// room about a defended attack names the attacker, so a defender facing two
// foes and a room watching a crowd know whose attack was stopped. Read from
// disk so every shipped defence store, counters included, is held to it.
func TestDefenceLinesNameTheAttacker(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "defense-messages")
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil || len(files) < 10 {
		t.Fatalf("found %d defence stores in %s (err %v); the guard would check too little", len(files), dir, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		var g DefenseMessageGroup
		if err := yaml.Unmarshal(raw, &g); err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, miss := range defenceLinesMissingAttacker(g) {
			t.Errorf("%s: %s names no attacker", filepath.Base(f), miss)
		}
	}
}
