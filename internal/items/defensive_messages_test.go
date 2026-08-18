package items

import (
	"strings"
	"testing"
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
