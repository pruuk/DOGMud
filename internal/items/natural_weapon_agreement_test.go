package items

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// naturalWeaponPools are the combat-message pools a natural weapon renders
// through: the natural-attack subtypes, plus every species natural_attack
// value found in either world.
var naturalWeaponPools = []ItemSubType{Claws, Bite, Sting, Gore, Slam}

// afterItemName finds each {itemname} and the text that follows it.
var afterItemName = regexp.MustCompile(`\{itemname\}(.*)`)

var ansiTag = regexp.MustCompile(`</?ansi[^>]*>`)

// weaponNotSubject lists the words that may follow {itemname} without
// making the weapon the subject of a verb: prepositions, conjunctions and
// the like ("with your {itemname} but misses", "{itemname} poised to
// strike", "you snap your {itemname} shut"). Participles ending in -ing or
// -ed are allowed too.
var weaponNotSubject = map[string]bool{
	"at": true, "into": true, "for": true, "ready": true, "poised": true,
	"low": true, "in": true, "down": true, "deep": true, "with": true,
	"up": true, "through": true, "overhead": true, "and": true,
	"toward": true, "towards": true, "to": true, "but": true, "as": true,
	"from": true, "on": true, "off": true, "across": true, "against": true,
	"over": true, "home": true, "shut": true,
}

// subjectVerbAfterItemName returns the word that makes {itemname} the
// subject of a verb on line, or "".
func subjectVerbAfterItemName(line string) string {
	for _, m := range afterItemName.FindAllStringSubmatch(line, -1) {
		rest := strings.TrimSpace(ansiTag.ReplaceAllString(m[1], ``))
		if rest == `` || !isLetter(rest[0]) {
			continue // punctuation or end of line: the weapon is an object
		}
		for _, word := range strings.Fields(rest) {
			w := strings.TrimRight(word, `.,!?'";:`)
			lower := strings.ToLower(w)
			if strings.HasSuffix(lower, "ly") && len(lower) > 3 {
				continue // an adverb: look at the word it modifies
			}
			if weaponNotSubject[lower] || strings.HasSuffix(lower, "ing") || strings.HasSuffix(lower, "ed") {
				break
			}
			return w
		}
	}
	return ``
}

func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

// The detector must be able to fail: each of these put the weapon before a
// verb and printed in play.
func TestSubjectVerbAfterItemName_CatchesShippedDefects(t *testing.T) {
	for _, line := range []string{
		`Your <ansi fg="item">{itemname}</ansi> bites into you!`,
		`Your <ansi fg="item">{itemname}</ansi> barely scrape it.`,
		`Their <ansi fg="item">{itemname}</ansi> <ansi fg="crit">CRITICALLY SMASHES</ansi> you!`,
		`Your {itemname} crashes home!`,
	} {
		if subjectVerbAfterItemName(line) == `` {
			t.Errorf("missed the verb after the weapon in %q", line)
		}
	}
	for _, line := range []string{
		`You rake it with your <ansi fg="item">{itemname}</ansi>!`,
		`It glares at you, <ansi fg="item">{itemname}</ansi> ready to snap.`,
		`It swipes with its <ansi fg="item">{itemname}</ansi> but misses.`,
		`It circles, <ansi fg="item">{itemname}</ansi> menacingly at you.`,
	} {
		if v := subjectVerbAfterItemName(line); v != `` {
			t.Errorf("flagged %q in %q, which has no verb after the weapon", v, line)
		}
	}
}

// naturalWeaponPoolFiles lists every natural-weapon pool file in both worlds.
func naturalWeaponPoolFiles(t *testing.T) []string {
	t.Helper()
	pools := map[string]bool{}
	for _, p := range naturalWeaponPools {
		pools[string(p)] = true
	}
	species, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "*", "species", "*.yaml"))
	if err != nil || len(species) == 0 {
		t.Fatalf("no species files found: %v", err)
	}
	for _, f := range species {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		var sp struct {
			NaturalAttack string `yaml:"natural_attack"`
		}
		if yaml.Unmarshal(raw, &sp) == nil && sp.NaturalAttack != `` {
			pools[sp.NaturalAttack] = true
		}
	}
	var files []string
	for _, world := range []string{"default", "dogmud"} {
		for pool := range pools {
			f := filepath.Join("..", "..", "_datafiles", "world", world, "combat-messages", pool+".yaml")
			if _, err := os.Stat(f); err == nil {
				files = append(files, f)
			}
		}
	}
	return files
}

// #455: a mob's natural weapon name is the species' unarmed name, singular
// for some ("maw", "beak") and plural for most ("claws", "pseudopods",
// "stone fists"), so no verb can agree with it: "A figure's claws bites
// into you!", "Your maw barely scrape". Every natural-weapon pool, in both
// worlds, has the attacker do the verb: "with their {itemname}".
func TestNaturalWeaponPoolsNeverPutAVerbAfterTheWeapon(t *testing.T) {
	files := naturalWeaponPoolFiles(t)
	if len(files) < 6 {
		t.Fatalf("found only %d natural-weapon pool files: %v", len(files), files)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if v := subjectVerbAfterItemName(line); v != `` {
				t.Errorf("%s:%d: the weapon is the subject of %q: %s", f, i+1, v, strings.TrimSpace(line))
			}
		}
	}
}
