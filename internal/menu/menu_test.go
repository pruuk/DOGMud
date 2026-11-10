package menu

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var tagRe = regexp.MustCompile(`<[^>]+>`)

func plain(s string) string { return tagRe.ReplaceAllString(s, "") }

// menuTestWidth is the reader these layout tests draw for.
const menuTestWidth = 80

func renderAt80(m Menu) string { return Render(m, menuTestWidth) }

func talkMenu() Menu {
	return Menu{
		Heading: `Kael sets down his whittling. "Ask what you like, I'll answer plain."`,
		Entries: []Entry{
			{Id: "feud", Marker: MarkerQuest, Label: "The Feud at the Well", Detail: "Do you want help settling the feud at the well?"},
			{Id: "pass", Marker: MarkerInfo, Label: "How do I reach the high pass?"},
			{Id: "carve", Label: "What do you carve?", Dim: true},
		},
		Leave: "Leave",
	}
}

func TestRender_TalkMenuLayout(t *testing.T) {
	got := plain(renderAt80(talkMenu()))
	want := strings.Join([]string{
		`Kael sets down his whittling. "Ask what you like, I'll answer plain."`,
		``,
		`  1. [!] The Feud at the Well`,
		`         Do you want help settling the feud at the well?`,
		`  2. [*] How do I reach the high pass?`,
		`  3.     What do you carve?`,
		`  4.     Leave`,
		``,
		`Type a number, or press Enter to leave.`,
	}, "\n")
	if got != want {
		t.Fatalf("render mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRender_DimEntryIsGreyAndOthersAreNot(t *testing.T) {
	out := renderAt80(talkMenu())
	if !strings.Contains(out, `<ansi fg="`+dimColor+`">What do you carve?</ansi>`) {
		t.Fatalf("dim entry not grey: %s", out)
	}
	if strings.Contains(out, `<ansi fg="`+dimColor+`">How do I reach`) {
		t.Fatalf("undimmed entry rendered grey: %s", out)
	}
}

func TestRender_DetailWrapsAtLabelIndentWithin80(t *testing.T) {
	m := Menu{Entries: []Entry{{Id: "a", Label: "A", Detail: strings.Repeat("word ", 30)}}}
	for _, line := range strings.Split(plain(renderAt80(m)), "\n") {
		if len(line) > 80 {
			t.Fatalf("line over 80 columns (%d): %q", len(line), line)
		}
	}
	lines := strings.Split(plain(renderAt80(m)), "\n")
	if !strings.HasPrefix(lines[1], "         word") || !strings.HasPrefix(lines[2], "         word") {
		t.Fatalf("detail not wrapped at the label indent: %q", lines[1:3])
	}
}

func TestRender_NumbersRightAlignPastNine(t *testing.T) {
	m := Menu{}
	for i := 0; i < 10; i++ {
		m.Entries = append(m.Entries, Entry{Id: string(rune('a' + i)), Label: "x"})
	}
	lines := strings.Split(plain(renderAt80(m)), "\n")
	if lines[0] != "   1.     x" || lines[9] != "  10.     x" {
		t.Fatalf("alignment wrong: %q / %q", lines[0], lines[9])
	}
}

func TestRender_TwoColumns(t *testing.T) {
	m := Menu{Columns: 2, Entries: []Entry{
		{Id: "a", Label: "alpha"}, {Id: "b", Label: "beta", Marker: MarkerChecked}, {Id: "c", Label: "gamma"},
	}}
	lines := strings.Split(plain(renderAt80(m)), "\n")
	if !strings.HasPrefix(lines[0], "  1.     alpha") || !strings.Contains(lines[0], "2. [x] beta") {
		t.Fatalf("row 1 wrong: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  3.     gamma") {
		t.Fatalf("row 2 wrong: %q", lines[1])
	}
}

var col2Re = regexp.MustCompile(`^\s*\d+\. `)

// assertCol2 checks every two-cell row puts column 2's number block at visible
// column 2+39, with a separating space before it, and no row exceeds 80.
func assertCol2(t *testing.T, m Menu) {
	t.Helper()
	for _, line := range strings.Split(plain(renderAt80(m)), "\n") {
		r := []rune(line)
		if len(r) > 80 {
			t.Fatalf("line over 80 columns: %q", line)
		}
		if len(r) <= 41 || !strings.Contains(string(r[41:]), ". ") {
			continue
		}
		if r[40] != ' ' {
			t.Fatalf("no separator before column 2: %q", line)
		}
		if !col2Re.MatchString(string(r[41:])) {
			t.Fatalf("column 2 number not at column 41: %q", line)
		}
	}
}

func TestRender_TwoColumnsAlignAndSeparate(t *testing.T) {
	full := strings.Repeat("w", 31)
	m := Menu{Columns: 2, Entries: []Entry{
		{Id: "a", Label: full}, {Id: "b", Label: "beta"},
		{Id: "c", Label: strings.Repeat("w", 45)}, {Id: "d", Label: "delta"},
		{Id: "e", Label: "café"}, {Id: "f", Label: "zeta"},
	}}
	assertCol2(t, m)
	lines := strings.Split(plain(renderAt80(m)), "\n")
	if !strings.HasPrefix(lines[1], "  3.     "+full+" ") {
		t.Fatalf("long label not truncated to 31 with a gap: %q", lines[1])
	}
}

func TestRender_TwoColumnsTenPlusEntries(t *testing.T) {
	m := Menu{Columns: 2}
	for i := 0; i < 12; i++ {
		m.Entries = append(m.Entries, Entry{Id: string(rune('a' + i)), Label: strings.Repeat("n", 20+i)})
	}
	assertCol2(t, m)
}

func TestRender_TwoColumnsCountRunesNotBytes(t *testing.T) {
	m := Menu{Columns: 2, Entries: []Entry{
		{Id: "a", Label: "café"}, {Id: "b", Label: "b"},
		{Id: "c", Label: strings.Repeat("é", 40)}, {Id: "d", Label: "d"},
	}}
	assertCol2(t, m)
	lines := strings.Split(plain(renderAt80(m)), "\n")
	if !strings.HasPrefix(lines[1], "  3.     "+strings.Repeat("é", 31)+" ") {
		t.Fatalf("accented label not cut to 31 runes: %q", lines[1])
	}
}

func TestRender_WrapCountsRunesNotBytes(t *testing.T) {
	m := Menu{Entries: []Entry{{Id: "a", Label: "A", Detail: strings.Repeat("éé ", 40)}}}
	lines := strings.Split(plain(renderAt80(m)), "\n")
	if n := len([]rune(lines[1])); n < 78 || n > 80 {
		t.Fatalf("first detail line is %d runes, want 78..80: %q", n, lines[1])
	}
}

func TestRender_LongMarkerDoesNotPanic(t *testing.T) {
	out := plain(renderAt80(Menu{Entries: []Entry{{Id: "a", Marker: "[long]", Label: "x"}}}))
	if !strings.Contains(out, "[long]x") {
		t.Fatalf("long marker: %q", out)
	}
}

func TestRender_FooterVariants(t *testing.T) {
	noLeave := Menu{Entries: []Entry{{Id: "a", Label: "A"}}}
	if !strings.HasSuffix(plain(renderAt80(noLeave)), "Type a number.") {
		t.Fatalf("no-leave footer: %q", plain(renderAt80(noLeave)))
	}
	withDefault := Menu{Entries: []Entry{{Id: "a", Label: "A"}, {Id: "b", Label: "B"}}, Default: "b"}
	if !strings.HasSuffix(plain(renderAt80(withDefault)), "Type a number, or press Enter for 2.") {
		t.Fatalf("default footer: %q", plain(renderAt80(withDefault)))
	}
	custom := Menu{Entries: []Entry{{Id: "a", Label: "A"}}, Footer: "Pick one."}
	if !strings.HasSuffix(plain(renderAt80(custom)), "Pick one.") {
		t.Fatalf("custom footer: %q", plain(renderAt80(custom)))
	}
}

func TestRender_EmptyMenuStillShowsHeadingAndFooter(t *testing.T) {
	got := plain(renderAt80(Menu{Heading: "Nothing here."}))
	if got != "Nothing here.\n\nType a number." {
		t.Fatalf("empty menu: %q", got)
	}
}

func TestRender_BodyWrapsBetweenHeadingAndEntries(t *testing.T) {
	m := Confirm("Cull the Wash", "Accept", "Leave")
	m.Body = []string{
		"Garve has asked you to thin out the bandits in the wash again: a bandit scout first, then a bandit squatter.",
		"Reward: 6 gold, and training in weapon combat.",
	}
	got := plain(renderAt80(m))
	want := strings.Join([]string{
		`Cull the Wash`,
		``,
		`Garve has asked you to thin out the bandits in the wash again: a bandit scout`,
		`first, then a bandit squatter.`,
		`Reward: 6 gold, and training in weapon combat.`,
		``,
		`  1.     Accept`,
		`  2.     Leave`,
		``,
		`Type a number, or press Enter for 2.`,
	}, "\n")
	if got != want {
		t.Fatalf("render mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range strings.Split(got, "\n") {
		if n := utf8.RuneCountInString(line); n > menuTestWidth {
			t.Fatalf("line over %d columns (%d): %q", menuTestWidth, n, line)
		}
	}
}

// A menu's prose follows its reader: the Body and each Detail wrap to the
// width given, so a narrow reader gets more lines and none over the width.
func TestRender_ProseWrapsToTheReadersWidth(t *testing.T) {
	long := "Garve has asked you to thin out the bandits in the wash again, a scout first and then a squatter."
	m := Menu{
		Heading: "Offer",
		Body:    []string{long},
		Entries: []Entry{{Id: "a", Label: "Accept", Detail: long}},
	}
	for _, width := range []int{30, 42, 80} {
		for _, line := range strings.Split(plain(Render(m, width)), "\n") {
			if n := utf8.RuneCountInString(line); n > width {
				t.Errorf("width %d: line over the width (%d): %q", width, n, line)
			}
		}
	}
	narrow := strings.Count(plain(Render(m, 30)), "\n")
	wide := strings.Count(plain(Render(m, 80)), "\n")
	if narrow <= wide {
		t.Errorf("width 30 made %d line breaks, width 80 made %d: the width is not reaching the wrap", narrow, wide)
	}
}

// The computed footers are 36 and 39 columns, so they wrap at the narrow end
// of the web range and stay one line at 80.
func TestRender_FooterWrapsToTheReadersWidth(t *testing.T) {
	entries := []Entry{{Id: "a", Label: "Accept"}}
	for _, tc := range []struct {
		name   string
		m      Menu
		footer string
	}{
		{"leave", Menu{Entries: entries, Leave: "Leave"}, "Type a number, or press Enter to leave."},
		{"default", Menu{Entries: entries, Default: "a"}, "Type a number, or press Enter for 1."},
		{"custom", Menu{Entries: entries, Footer: "Choose carefully, because the choice you make here is final."}, "Choose carefully, because the choice you make here is final."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, width := range []int{30, 37, 80} {
				for _, line := range strings.Split(plain(Render(tc.m, width)), "\n") {
					if n := utf8.RuneCountInString(line); n > width {
						t.Errorf("width %d: line over the width (%d): %q", width, n, line)
					}
				}
			}
			if !strings.HasSuffix(plain(Render(tc.m, 80)), "\n"+tc.footer) {
				t.Errorf("at 80 the footer must stay one line: %q", plain(Render(tc.m, 80)))
			}
			narrow := strings.Fields(plain(Render(tc.m, 30)))
			if !strings.Contains(strings.Join(narrow, " "), tc.footer) {
				t.Errorf("the wrap lost words of the footer: %q", narrow)
			}
		})
	}
}

func TestRender_NoBodyIsUnchanged(t *testing.T) {
	m := talkMenu()
	before := renderAt80(m)
	m.Body = nil
	if renderAt80(m) != before {
		t.Fatal("a nil Body changed the render")
	}
}
