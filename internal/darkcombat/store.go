// Package darkcombat holds the combat lines a player reads when they cannot
// see at all: one line per swing outcome, from the attacker's or the
// defender's side, written from touch and sound.
//
// Go decides WHEN a dark line replaces a composed one and which outcome it
// narrates (internal/combat, hideIdentitiesInPersonalLines). Go holds no
// wording: every line lives in
// _datafiles/world/dogmud/narration/dark-combat/<viewpoint>.yaml.
package darkcombat

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/pkg/errors"
)

// Viewpoint is whose personal line a dark line replaces.
type Viewpoint string

const (
	Attacker Viewpoint = "attacker"
	Defender Viewpoint = "defender"
)

// Outcome is one swing's result as a blind reader feels it. The buckets and
// their order are the retired replaceDarknessMessages' (M4d PR 2).
type Outcome string

const (
	Fumble    Outcome = "fumble"    // the attacker fumbled
	Crit      Outcome = "crit"      // the attacker crit
	Deflected Outcome = "deflected" // the defence won but damage still landed
	Defended  Outcome = "defended"  // the defence won clean
	Hit       Outcome = "hit"
	Miss      Outcome = "miss"
)

var allViewpoints = []Viewpoint{Attacker, Defender}

var allOutcomes = []Outcome{Fumble, Crit, Deflected, Defended, Hit, Miss}

// Viewpoints and Outcomes return copies of the vocabularies in a fixed order,
// for tests and the narration snapshot harness.
func Viewpoints() []Viewpoint { return append([]Viewpoint(nil), allViewpoints...) }
func Outcomes() []Outcome     { return append([]Outcome(nil), allOutcomes...) }

// MinVariants is the fewest lines one pool may hold. A blind fight repeats
// the same outcome many times; four keeps it from reading as one sentence.
const MinVariants = 4

const maxLineColumns = 80

var (
	ansiTag = regexp.MustCompile(`<[^>]*>`)
	// spacedHyphen is a hyphen or double hyphen standing in for a dash.
	spacedHyphen = regexp.MustCompile(`[A-Za-z,.!?'"] --? [A-Za-z]`)
)

// ViewpointGroup is one viewpoint's file.
type ViewpointGroup struct {
	Viewpoint Viewpoint            `yaml:"viewpoint"`
	Outcomes  map[Outcome][]string `yaml:"outcomes"`
}

func (g *ViewpointGroup) Id() string       { return string(g.Viewpoint) }
func (g *ViewpointGroup) Filepath() string { return string(g.Viewpoint) + ".yaml" }

// Validate fails the boot on a malformed file. Every outcome is reachable
// from combat, so a missing one would be silence in a dark fight.
func (g *ViewpointGroup) Validate() error {
	if !slices.Contains(allViewpoints, g.Viewpoint) {
		return errors.Errorf("unknown viewpoint %q", g.Viewpoint)
	}
	for o := range g.Outcomes {
		if !slices.Contains(allOutcomes, o) {
			return errors.Errorf("viewpoint %q declares unknown outcome %q", g.Viewpoint, o)
		}
	}
	for _, o := range allOutcomes {
		pool, ok := g.Outcomes[o]
		if !ok {
			return errors.Errorf("viewpoint %q is missing outcome %q", g.Viewpoint, o)
		}
		if err := narration.ValidateVariants(narration.Variants{Actor: pool}, MinVariants, narration.RoleActor); err != nil {
			return errors.Wrapf(err, "viewpoint %q outcome %q", g.Viewpoint, o)
		}
		for i, text := range pool {
			if err := validateLine(text); err != nil {
				return errors.Wrapf(err, "viewpoint %q outcome %q variant %d", g.Viewpoint, o, i)
			}
		}
	}
	return nil
}

// validateLine enforces dogmud-player-copy on a line this store owns: at most
// 80 visible columns, no numbers, no dashes, no {tokens} (nothing fills them,
// and a dark line names no one).
func validateLine(text string) error {
	plain := ansiTag.ReplaceAllString(text, "")
	if n := utf8.RuneCountInString(plain); n > maxLineColumns {
		return errors.Errorf("is %d visible columns, over %d", n, maxLineColumns)
	}
	if strings.ContainsAny(plain, "0123456789") {
		return errors.New("shows a number")
	}
	if strings.ContainsAny(plain, "—–") || spacedHyphen.MatchString(plain) {
		return errors.New("uses a dash")
	}
	if strings.ContainsAny(plain, "{}") {
		return errors.New("carries a token; this store fills none")
	}
	return nil
}

var loaded map[string]*ViewpointGroup

// LoadFrom loads the store from an explicit directory and returns the error.
// Tests use it because a test binary never reads config.yaml.
func LoadFrom(dir string) error {
	got, err := fileloader.LoadAllFlatFiles[string, *ViewpointGroup](dir)
	if err != nil {
		return errors.Wrap(err, "loading dark-combat lines")
	}
	for _, v := range allViewpoints {
		if got[string(v)] == nil {
			return errors.Errorf("dark-combat lines: no file for viewpoint %q", v)
		}
	}
	loaded = got
	return nil
}

// LoadDarkCombatFiles loads the store at boot and panics on any failure,
// matching lightnotice.LoadLightNoticeFiles.
func LoadDarkCombatFiles() {
	dir := string(configs.GetFilePathsConfig().DataFiles) + `/narration/dark-combat`
	if err := LoadFrom(dir); err != nil {
		panic(err)
	}
}

// UnloadForTest empties the store. Test-only by name.
func UnloadForTest() { loaded = nil }

// Pool returns one viewpoint's lines for one outcome, or nil when the store is
// unloaded.
func Pool(v Viewpoint, o Outcome) []string {
	g := loaded[string(v)]
	if g == nil {
		return nil
	}
	return g.Outcomes[o]
}

// Line picks one dark line. ok is false while the store is unloaded, which is
// what keeps every test package that never loads it on its composed lines.
func Line(v Viewpoint, o Outcome, pick narration.Picker) (string, bool) {
	pool := Pool(v, o)
	if len(pool) == 0 {
		return "", false
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pick).Actor, true
}
