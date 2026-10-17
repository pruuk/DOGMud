package combat

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"gopkg.in/yaml.v2"
)

// slamSplitTag strips every ansi tag so a rendered line can be compared with
// its template.
var slamSplitTag = regexp.MustCompile(`<[^>]*>`)

var slamSplitTokenRe = regexp.MustCompile(`\\\{[a-z]+\\\}`)

// slamSplitLoadFile parses one shipped dogmud combat-message file.
func slamSplitLoadFile(t *testing.T, sub items.ItemSubType) *items.WeaponAttackMessageGroup {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootForTest(t),
		"_datafiles", "world", "dogmud", "combat-messages", string(sub)+".yaml"))
	if err != nil {
		t.Fatalf("read %s.yaml: %v", sub, err)
	}
	var g items.WeaponAttackMessageGroup
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse %s.yaml: %v", sub, err)
	}
	return &g
}

// slamSplitLoadSpecies parses one shipped dogmud species file.
func slamSplitLoadSpecies(t *testing.T, file string) *species.Species {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootForTest(t),
		"_datafiles", "world", "dogmud", "species", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	sp := &species.Species{}
	if err := yaml.Unmarshal(raw, sp); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	return sp
}

// slamSplitActorMatchers turns every actor template of a file into a regexp
// with each token as a wildcard, tags stripped.
func slamSplitActorMatchers(g *items.WeaponAttackMessageGroup) map[string]*regexp.Regexp {
	out := map[string]*regexp.Regexp{}
	for _, opts := range g.Options {
		for _, tier := range []items.MessageOptions{
			opts.Together.ToAttacker.Beginner, opts.Together.ToAttacker.Expert, opts.Together.ToAttacker.Master,
		} {
			for _, m := range tier {
				plain := slamSplitTag.ReplaceAllString(string(m), ``)
				pattern := slamSplitTokenRe.ReplaceAllString(regexp.QuoteMeta(plain), `.*?`)
				out[plain] = regexp.MustCompile(`^` + pattern + `$`)
			}
		}
	}
	return out
}

func slamSplitMatchesAny(line string, ms map[string]*regexp.Regexp) bool {
	for _, re := range ms {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// A render test per type of the slam split (#541): a slime, a horse, a sand
// elemental and an automaton, built from their shipped species files, each
// swing through buildWeaponSetup and buildAttackMessages and get an attacker
// line from their own file, in that type's hit band. The four files and
// generic are all loaded, so a species on the wrong type would match the
// wrong file's line.
func TestSlamSplit_EachBodyRendersFromItsOwnFile(t *testing.T) {
	subs := []items.ItemSubType{items.Slam, items.Trample, items.Lash, items.Blast, items.Generic}
	pools := map[items.ItemSubType]*items.WeaponAttackMessageGroup{}
	matchers := map[items.ItemSubType]map[string]*regexp.Regexp{}
	for _, sub := range subs {
		pools[sub] = slamSplitLoadFile(t, sub)
		matchers[sub] = slamSplitActorMatchers(pools[sub])
	}
	t.Cleanup(items.SeedAttackMessagesForTest(pools))

	cases := []struct {
		file string
		want items.ItemSubType
	}{
		{"16-slime.yaml", items.Lash},
		{"26-horse.yaml", items.Trample},
		{"41-sand_elemental.yaml", items.Blast},
		{"45-automaton.yaml", items.Slam},
	}
	seeded := map[int]*species.Species{}
	for _, c := range cases {
		sp := slamSplitLoadSpecies(t, c.file)
		seeded[sp.SpeciesId] = sp
	}
	t.Cleanup(species.SeedSpeciesForTest(seeded))

	for _, c := range cases {
		sp := slamSplitLoadSpecies(t, c.file)
		t.Run(sp.Name, func(t *testing.T) {
			src, tgt := defenceFixture(1000)
			src.SpeciesId = sp.SpeciesId
			src.Name = "Rurik"
			tgt.Name = "Selka"
			tgt.HealthMax.Base = 100
			tgt.HealthMax.Recalculate()

			ws := buildWeaponSetup(src, tgt, items.Item{}, 0, 1)
			if ws.weaponSubType != c.want {
				t.Fatalf("%s swings as %q, want %q", sp.Name, ws.weaponSubType, c.want)
			}
			if ws.weaponName != sp.UnarmedName {
				t.Errorf("%s weapon name = %q, want its unarmedname %q", sp.Name, ws.weaponName, sp.UnarmedName)
			}

			// Several draws, so a lucky pick cannot carry the test.
			for range 20 {
				result := &AttackResult{}
				buildAttackMessages(result, src, tgt, ws, swingDamageParams{dmgMean: 20},
					10, 0, 0, 0, User, User, "", false, false)
				if len(result.MessagesToSource) == 0 {
					t.Fatalf("%s built no attacker line", sp.Name)
				}
				msg := result.MessagesToSource[0]
				line := strings.TrimSpace(slamSplitTag.ReplaceAllString(msg.Text, ``))
				if !slamSplitMatchesAny(line, matchers[c.want]) {
					t.Fatalf("%s line is not from %s.yaml: %q", sp.Name, c.want, line)
				}
				if slamSplitMatchesAny(line, matchers[items.Generic]) {
					t.Fatalf("%s line also reads as a generic line, so it proves nothing: %q", sp.Name, line)
				}
				if want := CategoryForWeaponSubtype(c.want); msg.Category != want {
					t.Errorf("%s line category = %v, want %v", sp.Name, msg.Category, want)
				}
			}
		})
	}
}

// The matcher must be able to fail: a slam line is not a lash line.
func TestSlamSplit_MatcherCanFail(t *testing.T) {
	slam := slamSplitActorMatchers(slamSplitLoadFile(t, items.Slam))
	if len(slam) < 40 {
		t.Fatalf("only %d slam actor templates decoded", len(slam))
	}
	if slamSplitMatchesAny(`You quiver and stretch your pseudopods toward Selka.`, slam) {
		t.Fatal("a lash line matched a slam template; the matcher accepts anything")
	}
}
