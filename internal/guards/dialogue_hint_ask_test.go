package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"gopkg.in/yaml.v2"
)

// askHintPattern is spec R30: a talk-menu hint never uses "ask" as a verb,
// because a player new to MUDs reads it as the `ask` command (which now only
// redirects to `talk`). The house phrase is "choose a line below".
var askHintPattern = regexp.MustCompile(`(?i)\byou could ask\b|\bask [a-z][a-z' -]* about\b`)

// askHintViolations names every hint in df (opener, opener variants, topics)
// that matches the pattern.
func askHintViolations(rel string, df dialogue.DialogueFile) []string {
	var out []string
	check := func(where, hint string) {
		if askHintPattern.MatchString(hint) {
			out = append(out, fmt.Sprintf("%s: %s: hint uses ask as a verb (R30): %q", rel, where, hint))
		}
	}
	if df.Opener != nil {
		check("opener", df.Opener.Hints)
		for i, v := range df.Opener.Variants {
			check(fmt.Sprintf("opener variant %d", i), v.Hints)
		}
	}
	for _, tp := range df.Topics {
		check("topic "+tp.Key, tp.Hints)
	}
	return out
}

// TestDialogueHints_NeverAskAsAVerb: no shipped talk-menu hint says "you could
// ask ..." or "ask <someone> about ...". Say "choose a line below" (R30).
func TestDialogueHints_NeverAskAsAVerb(t *testing.T) {
	root := filepath.Join("_datafiles", "world", "dogmud", "dialogue")
	paths, err := filepath.Glob(filepath.Join(root, "*", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no dialogue files (err %v): the guard cannot run", err)
	}
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var df dialogue.DialogueFile
		if err := yaml.UnmarshalStrict(data, &df); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, v := range askHintViolations(rel, df) {
			t.Error(v)
		}
	}
}

// TestDialogueHints_NeverAskAsAVerb_GuardCanFail proves the pattern sees the
// old wording in an opener, an opener variant and a topic, and passes the new.
func TestDialogueHints_NeverAskAsAVerb_GuardCanFail(t *testing.T) {
	bad := dialogue.DialogueFile{
		Opener: &dialogue.Opener{
			Hints:    "You could ask Velk about the barracks or the market.",
			Variants: []dialogue.OpenerVariant{{Hints: "You could also ask her about the road."}},
		},
		Topics: []dialogue.Topic{{Key: "road", Hints: "Ask the keeper about the door."}},
	}
	if got := askHintViolations("x.yaml", bad); len(got) != 3 {
		t.Fatalf("guard saw %d of 3 bad hints: %v", len(got), got)
	}
	good := dialogue.DialogueFile{
		Opener: &dialogue.Opener{Hints: "Choose a line below to hear about the barracks or the market."},
		Topics: []dialogue.Topic{{Key: "road", Hints: "Choose a line below to hear about the door."}},
	}
	if got := askHintViolations("x.yaml", good); len(got) != 0 {
		t.Fatalf("guard flagged the house phrase: %v", got)
	}
}
