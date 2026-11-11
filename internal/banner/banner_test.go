package banner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatSkillNoTier(t *testing.T) {
	got := Format(Skill, "Unarmed Combat", nil, 80)
	if !strings.Contains(got, "SKILL ADVANCEMENT") {
		t.Errorf("missing SKILL ADVANCEMENT: %q", got)
	}
	if !strings.Contains(got, "Unarmed Combat") {
		t.Errorf("missing name: %q", got)
	}
	if strings.Contains(got, "→") {
		t.Errorf("no tier should mean no arrow: %q", got)
	}
}

func TestFormatStatNoTier(t *testing.T) {
	got := Format(Stat, "Strength", nil, 80)
	if !strings.Contains(got, "STATISTIC INCREASED") {
		t.Errorf("missing STATISTIC INCREASED: %q", got)
	}
}

func TestFormatTierCrossingIncludesArrow(t *testing.T) {
	got := Format(Skill, "Unarmed Combat",
		&TierChange{From: "apprentice", To: "journeyman"}, 80)
	if !strings.Contains(got, "apprentice → journeyman") {
		t.Errorf("missing tier transition: %q", got)
	}
}

// goldenBanners are the banners pinned at 80 columns: the captured skill
// advancement, the longest skill name, a stat, and the longest tier change.
func goldenBanners(width int) string {
	return Format(Skill, "weapon-combat", &TierChange{From: "novice", To: "apprentice"}, width) + "\n" +
		Format(Skill, "unarmed-combat", nil, width) + "\n" +
		Format(Stat, "charisma", nil, width) + "\n" +
		Format(Skill, "unarmed-combat", &TierChange{From: "master", To: "grandmaster"}, width) + "\n"
}

// A desktop sees what master drew (testdata/banner_80.golden, recorded from
// master before plan 3b-3): a 64-glyph rule with the text centred against
// it, at 80 columns, at 64, and on a telnet screen wider than 80. A width
// of 0 (no reader width known) draws the same.
func TestFormatAtFullWidthIsMaster(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "banner_80.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{0, 64, 80, 120} {
		if got := goldenBanners(width); got != string(want) {
			t.Errorf("at %d the banners changed:\n got %q\nwant %q", width, got, string(want))
		}
	}
}

// The banners as the owner approved them at 42 and 30
// (narrow-screens-3b-mock/forms/banner_combat_*.txt): the rule spans the
// screen and each line is centred against it.
func TestFormatIsTheDrawing(t *testing.T) {
	rule := func(n int) string { return strings.Repeat("━", n) }
	for _, tc := range []struct {
		width int
		got   string
		want  []string
	}{
		{42, Format(Stat, "charisma", nil, 42), []string{rule(42), "           STATISTIC INCREASED", "                 charisma", rule(42)}},
		{30, Format(Stat, "charisma", nil, 30), []string{rule(30), "     STATISTIC INCREASED", "           charisma", rule(30)}},
		{42, Format(Skill, "weapon-combat", &TierChange{From: "novice", To: "apprentice"}, 42),
			[]string{rule(42), "            SKILL ADVANCEMENT", "              weapon-combat", "           novice → apprentice", rule(42)}},
		{30, Format(Skill, "weapon-combat", &TierChange{From: "novice", To: "apprentice"}, 30),
			[]string{rule(30), "      SKILL ADVANCEMENT", "        weapon-combat", "     novice → apprentice", rule(30)}},
	} {
		if want := strings.Join(tc.want, "\n"); tc.got != want {
			t.Errorf("at %d:\n got %q\nwant %q", tc.width, tc.got, want)
		}
	}
}

// At every width the rule is min(width, 64) glyphs, no line is wider, and
// each text line is centred against the rule (spec ruling 12: per-round
// output is laid out for the reader's width without measuring a form).
func TestFormatFitsEveryWidth(t *testing.T) {
	for _, width := range []int{30, 42, 60, 64, 80} {
		ruleWidth := min(width, 64)
		for _, b := range strings.Split(strings.TrimSuffix(goldenBanners(width), "\n"), "\n") {
			w := utf8.RuneCountInString(b)
			if strings.HasPrefix(b, "━") {
				if w != ruleWidth {
					t.Errorf("at %d a rule is %d wide, want %d", width, w, ruleWidth)
				}
				continue
			}
			text := strings.TrimLeft(b, " ")
			if pad, want := w-utf8.RuneCountInString(text), (ruleWidth-utf8.RuneCountInString(text))/2; pad != want || w > ruleWidth {
				t.Errorf("at %d %q is indented %d, want %d centred in %d", width, text, pad, want, ruleWidth)
			}
		}
	}
}
