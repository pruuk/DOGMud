// Package lightnotice tells a player, in world terms, when the light they can
// see by crosses a band: dark, shapes, faces or dazzled.
//
// Go decides WHETHER a notice fires and names its cause and transition. Go
// holds no wording: every line lives in
// _datafiles/world/dogmud/narration/light-notices/<cause>.yaml.
package lightnotice

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/pkg/errors"
)

// Cause names what changed the light, as the player is told it.
type Cause string

const (
	CauseMovement Cause = "movement" // the player is in a different room
	CauseCarried  Cause = "carried"  // a carried light arrived or left
	CauseLamp     Cause = "lamp"     // the room's own lamp
	CauseWeather  Cause = "weather"  // weather filtering the sky
	CauseSky      Cause = "sky"      // the sky itself: dusk, dawn, moons
	CauseEyes     Cause = "eyes"     // no light term explains it; the observer's sight changed
	// CauseDarkness is a carried darkness arriving, lapsing or changing
	// strength (lighting plan 5d, ruling D4). Without it such a change moves
	// no light term and falls to CauseEyes, and carried's lines ("The
	// carried light is gone") are wrong for it.
	CauseDarkness Cause = "darkness"
)

// Transition names the band change a line narrates.
type Transition string

const (
	DarkerFaces   Transition = "darker_faces"   // dazzled down to faces
	DarkerShapes  Transition = "darker_shapes"  // down to shapes
	DarkerDark    Transition = "darker_dark"    // down to nothing
	LighterShapes Transition = "lighter_shapes" // dark up to shapes
	LighterFaces  Transition = "lighter_faces"  // up to faces
	IntoDazzle    Transition = "dazzled"        // up into dazzle
)

var allCauses = []Cause{CauseMovement, CauseCarried, CauseLamp, CauseWeather, CauseSky, CauseEyes, CauseDarkness}

var allTransitions = []Transition{DarkerFaces, DarkerShapes, DarkerDark, LighterShapes, LighterFaces, IntoDazzle}

// Causes and Transitions return copies of the full vocabularies, in a fixed
// order, for tests and the narration snapshot harness.
func Causes() []Cause           { return append([]Cause(nil), allCauses...) }
func Transitions() []Transition { return append([]Transition(nil), allTransitions...) }

// MinVariants is the fewest lines any one pool may hold, so a player who sees
// the same crossing twice need not read the same sentence.
const MinVariants = 2

// maxLineColumns is dogmud-player-copy's hard wrap. A notice is one line.
const maxLineColumns = 80

// Pools is one transition's lines: either one pool for any setting, or an
// outdoor and an indoor pool when the wording needs the difference.
type Pools struct {
	Any     []string `yaml:"any,omitempty"`
	Outdoor []string `yaml:"outdoor,omitempty"`
	Indoor  []string `yaml:"indoor,omitempty"`
}

// CauseGroup is one cause's file.
type CauseGroup struct {
	Cause       Cause                 `yaml:"cause"`
	Transitions map[Transition]*Pools `yaml:"transitions"`
}

func (g *CauseGroup) Id() string       { return string(g.Cause) }
func (g *CauseGroup) Filepath() string { return string(g.Cause) + ".yaml" }

// Validate fails the boot on any malformed file. Every cause must author every
// transition, because the trigger rules can reach all of them: a missing pool
// would be silence in play rather than a boot failure.
func (g *CauseGroup) Validate() error {
	if !slices.Contains(allCauses, g.Cause) {
		return errors.Errorf("unknown cause %q", g.Cause)
	}
	for tr := range g.Transitions {
		if !slices.Contains(allTransitions, tr) {
			return errors.Errorf("cause %q declares unknown transition %q", g.Cause, tr)
		}
	}
	for _, tr := range allTransitions {
		p, ok := g.Transitions[tr]
		if !ok {
			return errors.Errorf("cause %q is missing transition %q", g.Cause, tr)
		}
		if p == nil {
			return errors.Errorf("cause %q transition %q is empty", g.Cause, tr)
		}
		if err := p.validate(); err != nil {
			return errors.Wrapf(err, "cause %q transition %q", g.Cause, tr)
		}
	}
	return nil
}

func (p *Pools) validate() error {
	split := len(p.Outdoor) > 0 || len(p.Indoor) > 0
	switch {
	case len(p.Any) > 0 && split:
		return errors.New("declares both any and outdoor/indoor; author one or the other")
	case split && (len(p.Outdoor) == 0 || len(p.Indoor) == 0):
		return errors.New("an outdoor/indoor split must author both settings")
	case !split && len(p.Any) == 0:
		return errors.New("holds no lines")
	}
	named := []struct {
		name string
		pool []string
	}{{"any", p.Any}, {"outdoor", p.Outdoor}, {"indoor", p.Indoor}}
	for _, n := range named {
		if len(n.pool) == 0 {
			continue
		}
		if err := narration.ValidateVariants(narration.Variants{Actor: n.pool}, MinVariants, narration.RoleActor); err != nil {
			return errors.Wrap(err, n.name)
		}
		for i, text := range n.pool {
			if err := validateLine(text); err != nil {
				return errors.Wrapf(err, "%s variant %d", n.name, i)
			}
		}
	}
	return nil
}

// validateLine enforces dogmud-player-copy on a line this store alone owns: one
// line of at most 80 columns, no numbers, no dashes, and no {tokens} (this
// store fills none, so a token would render its own braces).
func validateLine(text string) error {
	if n := utf8.RuneCountInString(text); n > maxLineColumns {
		return errors.Errorf("is %d columns, over %d", n, maxLineColumns)
	}
	if strings.ContainsAny(text, "0123456789") {
		return errors.New("shows a number")
	}
	if strings.ContainsAny(text, "—–") {
		return errors.New("uses an em or en dash")
	}
	if strings.ContainsAny(text, "{}") {
		return errors.New("carries a token; this store fills none")
	}
	return nil
}

// pool returns the lines for a setting.
func (p *Pools) pool(indoor bool) []string {
	if len(p.Any) > 0 {
		return p.Any
	}
	if indoor {
		return p.Indoor
	}
	return p.Outdoor
}

var loaded map[string]*CauseGroup

// LoadFrom loads the store from an explicit directory and returns the error.
// It exists for tests: a test binary never reads config.yaml, so the
// configured path would resolve to _datafiles/world/default.
func LoadFrom(dir string) error {
	got, err := fileloader.LoadAllFlatFiles[string, *CauseGroup](dir)
	if err != nil {
		return errors.Wrap(err, "loading light notices")
	}
	for _, c := range allCauses {
		if got[string(c)] == nil {
			return errors.Errorf("light notices: no file for cause %q", c)
		}
	}
	loaded = got
	return nil
}

// LoadLightNoticeFiles loads the store at boot and panics on any failure,
// matching movenarration.LoadMoveNarrationFiles.
func LoadLightNoticeFiles() {
	dir := string(configs.GetFilePathsConfig().DataFiles) + `/narration/light-notices`
	if err := LoadFrom(dir); err != nil {
		panic(err)
	}
}

// Pool returns the lines one notice draws from, or nil when the store is
// unloaded or the entry is absent.
func Pool(c Cause, tr Transition, indoor bool) []string {
	g := loaded[string(c)]
	if g == nil {
		return nil
	}
	p := g.Transitions[tr]
	if p == nil {
		return nil
	}
	return p.pool(indoor)
}

// line renders one notice through the narration core. ok is false when there
// is nothing to say, which is always the case while the store is unloaded:
// that is what keeps every test package that never loads it silent.
func line(c Cause, tr Transition, indoor bool, pick narration.Picker) (string, bool) {
	pool := Pool(c, tr, indoor)
	if len(pool) == 0 {
		return "", false
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pick).Actor, true
}

// setStoreForTest installs a store directly. Test-only by name.
func setStoreForTest(m map[string]*CauseGroup) { loaded = m }
