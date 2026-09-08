package items

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

func validDefenseMessageGroup() *DefenseMessageGroup {
	options := DefenseIntensity{}
	for _, intensity := range []Intensity{Weak, Normal, Heavy} {
		prefix := string(intensity)
		options[intensity] = DefenseOptions{Together: DefenseTogetherMessages{
			ToDefender: MessageOptions{ItemMessage(prefix + "-def-0"), ItemMessage(prefix + "-def-1"), ItemMessage(prefix + "-def-2"), ItemMessage(prefix + "-def-3"), ItemMessage(prefix + "-def-4")},
			ToAttacker: MessageOptions{ItemMessage(prefix + "-atk-0"), ItemMessage(prefix + "-atk-1"), ItemMessage(prefix + "-atk-2"), ItemMessage(prefix + "-atk-3"), ItemMessage(prefix + "-atk-4")},
			ToRoom:     MessageOptions{ItemMessage(prefix + "-room-0"), ItemMessage(prefix + "-room-1"), ItemMessage(prefix + "-room-2"), ItemMessage(prefix + "-room-3"), ItemMessage(prefix + "-room-4")},
		}}
	}
	return &DefenseMessageGroup{OptionId: DefenseQuell, Options: options}
}

func TestDefenseMessageValidAcceptsFiveCoordinatedVariantsPerBand(t *testing.T) {
	if err := validDefenseMessageGroup().Validate(); err != nil {
		t.Fatalf("valid five-variant group rejected: %v", err)
	}
}

func TestDefenseMessageValidRejectsInvalidAudienceShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DefenseMessageGroup)
		want   string
	}{
		{"missing_band", func(g *DefenseMessageGroup) { delete(g.Options, Normal) }, "missing option"},
		{"fewer_than_five", func(g *DefenseMessageGroup) {
			o := g.Options[Weak]
			o.Together.ToDefender = o.Together.ToDefender[:4]
			o.Together.ToAttacker = o.Together.ToAttacker[:4]
			o.Together.ToRoom = o.Together.ToRoom[:4]
			g.Options[Weak] = o
		}, "at least 5"},
		{"empty_defender", func(g *DefenseMessageGroup) {
			o := g.Options[Normal]
			o.Together.ToDefender = nil
			g.Options[Normal] = o
		}, "todefender"},
		{"empty_attacker", func(g *DefenseMessageGroup) {
			o := g.Options[Normal]
			o.Together.ToAttacker = nil
			g.Options[Normal] = o
		}, "toattacker"},
		{"empty_room", func(g *DefenseMessageGroup) { o := g.Options[Normal]; o.Together.ToRoom = nil; g.Options[Normal] = o }, "toroom"},
		{"unequal_lengths", func(g *DefenseMessageGroup) {
			o := g.Options[Heavy]
			o.Together.ToRoom = append(o.Together.ToRoom, "heavy-room-5")
			g.Options[Heavy] = o
		}, "equal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			group := validDefenseMessageGroup()
			tc.mutate(group)
			err := group.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestDefenseMessageRenderCoordinatesAudienceIndexAndBands(t *testing.T) {
	restore := SeedDefenseMessagesForTest(map[DefenseType]*DefenseMessageGroup{DefenseQuell: validDefenseMessageGroup()})
	defer restore()

	tests := []struct {
		name          string
		crit          bool
		margin        float64
		wantIntensity string
	}{
		{"narrow_noncrit_is_weak", false, 0.49, "weak"},
		{"clean_noncrit_is_normal", false, 0.5, "normal"},
		{"large_noncrit_never_heavy", false, 99, "normal"},
		{"crit_is_always_heavy", true, 0.01, "heavy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			triad := RenderDefenseMessage(DefenseQuell, tc.crit, tc.margin, map[TokenName]string{}, 3)
			want := tc.wantIntensity + "-def-3"
			if string(triad.ToDefender) != want {
				t.Fatalf("defender = %q, want %q", triad.ToDefender, want)
			}
			if string(triad.ToAttacker) != tc.wantIntensity+"-atk-3" || string(triad.ToRoom) != tc.wantIntensity+"-room-3" {
				t.Fatalf("audiences did not share index 3: %+v", triad)
			}
		})
	}
}

func TestDefenseMessageRenderReplacesTokensAfterCoordinatedSelection(t *testing.T) {
	group := validDefenseMessageGroup()
	o := group.Options[Weak]
	o.Together.ToDefender[2] = "{defender}|{attacker}|{attack}"
	o.Together.ToAttacker[2] = "{attacker}|{defender}|{attack}"
	o.Together.ToRoom[2] = "{attack}|{attacker}|{defender}"
	group.Options[Weak] = o
	restore := SeedDefenseMessagesForTest(map[DefenseType]*DefenseMessageGroup{DefenseQuell: group})
	defer restore()

	triad := RenderDefenseMessage(DefenseQuell, false, 0.1, map[TokenName]string{
		TokenDefender: "Selka", TokenAttacker: "Rurik", TokenAttack: "Mind Fog",
	}, 2)
	if triad.ToDefender != "Selka|Rurik|Mind Fog" || triad.ToAttacker != "Rurik|Selka|Mind Fog" || triad.ToRoom != "Mind Fog|Rurik|Selka" {
		t.Fatalf("token replacement mismatch: %+v", triad)
	}
}

// ---------------------------------------------------------------------
// RenderTriad: the coordinated-render step extracted so both defence paths
// (RenderDefenseMessage's band-selecting wrapper AND the melee path in
// internal/combat, which does its own zScore banding via GetDefenseMessage)
// share ONE place that turns an already-selected band into a coherent triad.
// ---------------------------------------------------------------------

// TestRenderTriadUsesSameIndexAcrossAllThreeRoles is the regression test for
// the melee-narration coherence bug: the authored pools pair up BY INDEX
// (variant N of todefender/toattacker/toroom describe the SAME event), so
// RenderTriad must pick exactly one index and use it for all three roles.
// Walking a SequencePicker across more than one full lap of the pool proves
// this holds for every index, not just index 0.
func TestRenderTriadUsesSameIndexAcrossAllThreeRoles(t *testing.T) {
	group := validDefenseMessageGroup()
	options := group.Options[Weak]

	n := len(options.Together.ToDefender)
	picker := narration.SequencePicker()
	for i := 0; i < n*2; i++ {
		triad := options.RenderTriad(map[TokenName]string{}, picker)
		wantIdx := i % n
		wantDef := ItemMessage(fmt.Sprintf("weak-def-%d", wantIdx))
		wantAtk := ItemMessage(fmt.Sprintf("weak-atk-%d", wantIdx))
		wantRoom := ItemMessage(fmt.Sprintf("weak-room-%d", wantIdx))
		if triad.ToDefender != wantDef || triad.ToAttacker != wantAtk || triad.ToRoom != wantRoom {
			t.Fatalf("iteration %d: got defender=%q attacker=%q room=%q, want all three at index %d",
				i, triad.ToDefender, triad.ToAttacker, triad.ToRoom, wantIdx)
		}
	}
}

// TestRenderTriadEmptyOnMismatchedPoolLengths pins the same invariant guard
// RenderDefenseMessage already enforced: unequal-length audience pools cannot
// be coordinated by index, so RenderTriad must refuse to render rather than
// silently pairing mismatched variants.
func TestRenderTriadEmptyOnMismatchedPoolLengths(t *testing.T) {
	group := validDefenseMessageGroup()
	options := group.Options[Weak]
	options.Together.ToRoom = append(options.Together.ToRoom, "weak-room-extra")

	triad := options.RenderTriad(map[TokenName]string{}, nil)
	if triad != (DefenseMessageTriad{}) {
		t.Fatalf("expected empty triad for mismatched pool lengths, got %+v", triad)
	}
}

// TestRenderTriadEmptyWhenToDefenderEmpty mirrors RenderDefenseMessage's other
// guard: an empty ToDefender pool (e.g. a band with no authored data) must not
// render anything, even if ToAttacker/ToRoom happen to be non-empty.
func TestRenderTriadEmptyWhenToDefenderEmpty(t *testing.T) {
	options := DefenseOptions{}
	triad := options.RenderTriad(map[TokenName]string{}, nil)
	if triad != (DefenseMessageTriad{}) {
		t.Fatalf("expected empty triad for empty ToDefender pool, got %+v", triad)
	}
}
