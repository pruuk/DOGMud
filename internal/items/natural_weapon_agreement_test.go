package items

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// naturalWeaponSubjectVerb finds {itemname} used as the subject of a
// singular verb: the next word, through one colour tag and any capitalised
// adverb, ends in "s" ("bites", "CRITICALLY EVISCERATES").
var naturalWeaponSubjectVerb = regexp.MustCompile(`\{itemname\}</ansi> (?:<ansi fg="[^"]*">)?(?:[A-Z]+ )*[A-Za-z]+[sS]\b`)

// #455: a mob's natural weapon name is the species' unarmed name, mostly
// plural ("claws", "fangs", "jaws"), so "A figure's claws bites into you!"
// The claws and bite pools never make {itemname} the subject of a singular
// verb; the attacker does the verb, "with their {itemname}".
func TestNaturalWeaponPoolsNeverPutASingularVerbAfterTheWeapon(t *testing.T) {
	for _, name := range []string{"claws.yaml", "bite.yaml"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "combat-messages", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if m := naturalWeaponSubjectVerb.FindString(line); m != "" {
				t.Errorf("%s:%d: %q makes the weapon the subject of a singular verb", name, i+1, m)
			}
		}
	}
}
