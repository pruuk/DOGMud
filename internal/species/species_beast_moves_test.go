package species

import (
	"reflect"
	"strings"
	"testing"
)

// Spec 2026-10-16 #541 section 5, the Proposed column as reviewed, with the
// slam split's smoke elemental row. A species missing here has no beast
// moves.
var proposedBeastMoves = map[int][]string{
	2:  {"maul", "throttle", "pounce", "hamstring"}, // canine: wolves do all four
	3:  {"rake"},                                    // bear: a swipe; charge stays scripted
	6:  {"gore"},                                    // boar
	7:  {"gore"},                                    // deer
	9:  {"rake", "pounce"},                          // raptor: a stooping strike
	10: {"hamstring"},                               // rodent: ankle-biters
	11: {"rake", "pounce", "hamstring", "throttle"}, // feline: the throat crusher
	17: {"pounce"},                                  // arachnid: jumping spiders
	21: {"maul"},                                    // reptile: the crocodile's roll
	22: {"rake"},                                    // bat
	24: {"throttle", "pounce"},                      // mustelid: a neck bite
	32: {"drain"},                                   // wraith
	33: {"drain"},                                   // spectre
	34: {"drain"},                                   // vampire (hands allowed: drain is exempt)
	44: {"smother"},                                 // smoke elemental (Appendix A)
}

func TestBeastMovesMatchTheReviewedTable(t *testing.T) {
	all := dogmudSpecies(t)
	for id, sp := range all {
		want := proposedBeastMoves[id]
		got := sp.BeastMoves
		if len(want) == 0 && len(got) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("species %d (%s) beast_moves = %v, want %v", id, sp.Name, got, want)
		}
	}
	for id := range proposedBeastMoves {
		if all[id] == nil {
			t.Errorf("species %d in the table has no file", id)
		}
	}
}

// Every shipped list passes the load validator.
func TestShippedBeastMovesValidate(t *testing.T) {
	for id, sp := range dogmudSpecies(t) {
		if err := validateBeastMoves(sp); err != nil {
			t.Errorf("species %d: %v", id, err)
		}
	}
}

// The validator must be able to fail, once per rule.
func TestValidateBeastMoves_CanFail(t *testing.T) {
	cases := map[string]struct {
		sp   Species
		want string
	}{
		"unknown move":   {Species{SpeciesId: 1, Name: "x", BeastMoves: []string{"charge"}}, "unknown beast move"},
		"listed twice":   {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"legs"}, BeastMoves: []string{"rake", "rake"}}, "twice"},
		"hands":          {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"arms", "hands"}, BeastMoves: []string{"rake"}}, "without hands"},
		"pounce no legs": {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"mouth"}, BeastMoves: []string{"pounce"}}, "legs"},
		"gore no horns":  {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"legs"}, BeastMoves: []string{"gore"}}, "horns"},
		"drain no flag":  {Species{SpeciesId: 1, Name: "x", BeastMoves: []string{"drain"}}, "lifedrain"},
	}
	for name, c := range cases {
		err := validateBeastMoves(&c.sp)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one containing %q", name, err, c.want)
		}
	}
	vampire := Species{SpeciesId: 34, Name: "vampire", BodyParts: []string{"arms", "hands"}, LifeDrain: true, BeastMoves: []string{"drain"}}
	if err := validateBeastMoves(&vampire); err != nil {
		t.Errorf("a hands-bearing lifedrain species must drain: %v", err)
	}
}
