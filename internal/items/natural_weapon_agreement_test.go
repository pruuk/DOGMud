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
var naturalWeaponPools = []ItemSubType{Claws, Bite, Sting, Gore, Slam, Siphon, Trample, Lash, Blast, Maul, Pounce}

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
	// A direction word, like "down" and "up" ("drive your {itemname} forward").
	"forward": true,
	// Modal and place words that take no verb ending ("before your {itemname}
	// can close", "clamp your {itemname} around the throat").
	"can": true, "around": true, "tight": true,
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
		`You drive your {itemname} forward and crash in!`,
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

// bodyPartBeforeBanner finds a natural-weapon word written out in the pool
// (not {itemname}) as the subject of a FUMBLE banner: "A figure's claws
// FUMBLE pathetically as they stumble!" (#631). The pool's word is wrong for
// some species that render through it (a bat's "wings and fangs" through
// claws), and the #455 rule holds either way: the attacker fumbles, the
// weapon does not.
var bodyPartBeforeBanner = regexp.MustCompile(`\b(claws|jaws|fangs|teeth|talons|horns?|tail|stinger|beak|maw|fists?)\s+(<ansi[^>]*>)?FUMBLES?\b`)

func TestBodyPartBeforeBanner_CanFail(t *testing.T) {
	if !bodyPartBeforeBanner.MatchString(`{actor}''s claws <ansi fg="red-bold">FUMBLE</ansi> pathetically as they stumble!`) {
		t.Error("missed the shipped #631 line")
	}
	if bodyPartBeforeBanner.MatchString(`You <ansi fg="red-bold">FUMBLE</ansi> your <ansi fg="item">{itemname}</ansi> awkwardly!`) {
		t.Error("flagged a line where the attacker fumbles")
	}
}

func TestNaturalWeaponPoolsNeverMakeABodyPartFumble(t *testing.T) {
	for _, f := range naturalWeaponPoolFiles(t) {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if bodyPartBeforeBanner.MatchString(line) {
				t.Errorf("%s:%d: a body part fumbles, not the attacker: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// #631: claws.yaml narrates every species whose natural attack is claws,
// among them birds, bats, skeletons and the smoke-wrought Elemental Prince,
// none of which can pounce. Pounce is a beast move with its own pool; the
// claws pool promises a strike instead.
func TestClawsPoolPromisesNoPounce(t *testing.T) {
	f := filepath.Join("..", "..", "_datafiles", "world", "dogmud", "combat-messages", "claws.yaml")
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read %s: %v", f, err)
	}
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(strings.ToLower(line), "pounc") {
			t.Errorf("%s:%d: promises a pounce: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}

// An actor is "they/their" everywhere in player copy, never "it/its", and a
// natural weapon is the actor's: "{actor} rakes their {itemname}", not "its".
// Task 6 of M6 slice 2 ruled this; the verb-agreement rewrite once put "its"
// into about 55 lines.
var (
	itsBeforeWeapon = regexp.MustCompile(`\bits\s+(<ansi[^>]*>)?\{itemname\}`)
	itsAfterActor   = regexp.MustCompile(`\{actor\}[^.!?]*\bits\s+([A-Za-z]+)`)
)

// objectNounsAfterIts are the things an actor line may legitimately call
// "its": a thing the actor holds or aims, not the actor itself.
var objectNounsAfterIts = map[string]bool{
	"crystal": true, "energy": true, "mark": true, "length": true,
	"tip": true, "target": true,
}

// actorItsOffence returns a description of an actor-possessive "its" on line,
// or "".
func actorItsOffence(line string) string {
	if itsBeforeWeapon.MatchString(line) {
		return `its {itemname}`
	}
	for _, m := range itsAfterActor.FindAllStringSubmatch(line, -1) {
		if !objectNounsAfterIts[strings.ToLower(m[1])] {
			return `its ` + m[1] + ` after {actor}`
		}
	}
	return ``
}

func TestActorItsDetector_CanFail(t *testing.T) {
	for _, line := range []string{
		`<ansi fg="a">{actor}</ansi> rakes its <ansi fg="item">{itemname}</ansi> through the air`,
		`{actor} cuts through empty air with its {itemname}!`,
		`{actor} loses its angle!`,
	} {
		if actorItsOffence(line) == `` {
			t.Errorf("missed an actor 'its' in %q", line)
		}
	}
	for _, line := range []string{
		`{actor}''s wand FUMBLES as its energy fizzles!`,
		`{actor} raises their {itemname}, its crystal pulsing.`,
		`The shot finds its mark!`,
	} {
		if v := actorItsOffence(line); v != `` {
			t.Errorf("flagged %q in %q", v, line)
		}
	}
}

func TestCombatCopyNeverCallsAnActorIts(t *testing.T) {
	var files []string
	for _, pat := range []string{
		filepath.Join("..", "..", "_datafiles", "world", "*", "combat-messages", "*.yaml"),
		filepath.Join("..", "..", "_datafiles", "world", "*", "narration", "special-moves", "*.yaml"),
	} {
		got, err := filepath.Glob(pat)
		if err != nil || len(got) == 0 {
			t.Fatalf("no files for %s (err %v); the guard would check nothing", pat, err)
		}
		files = append(files, got...)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			if v := actorItsOffence(line); v != `` {
				t.Errorf("%s:%d: an actor is \"their\", not %q: %s", f, i+1, v, strings.TrimSpace(line))
			}
		}
	}
}
