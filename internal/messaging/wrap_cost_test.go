package messaging

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

// wrapBudget is far above what a linear pass over these inputs costs (a few
// milliseconds). The test guards against the quadratic scan returning: a scan
// to the end of the text at every less-than sign. The lastTagEnd shortcut in
// WrapAnsi is not separately measured.
const wrapBudget = 2 * time.Second

func TestWrapAnsi_LinearInLessThanSigns(t *testing.T) {
	const signs = 64000
	half := strings.Repeat("<", signs/2)

	cases := []struct {
		name string
		in   string
	}{
		{"only less-than signs", strings.Repeat("<", signs)},
		{"signs around tagged words", half + strings.Repeat(`<ansi fg="red">word</ansi> `, 2000) + half},
		{"signs before a lone tag at the end", strings.Repeat("<", signs) + `<ansi fg="red">x</ansi>`},
		{"open tags that never close", strings.Repeat("<ansi ", signs/5)},
		{"closers that never finish", strings.Repeat("</ansi ", signs/5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			got := WrapAnsi(tc.in, 80)
			elapsed := time.Since(start)
			t.Logf("%d bytes in %v", len(tc.in), elapsed)
			if got == "" {
				t.Fatal("WrapAnsi returned nothing for non-empty input")
			}
			if elapsed > wrapBudget {
				t.Errorf("WrapAnsi took %v on %d bytes, budget %v", elapsed, len(tc.in), wrapBudget)
			}
		})
	}
}

// A run of stray signs is one word of visible text and comes back as it went in.
func TestWrapAnsi_StraySignsAreVisibleText(t *testing.T) {
	in := strings.Repeat("<", 200) + " tail"
	want := strings.Repeat("<", 200) + "\ntail"
	if got := WrapAnsi(in, 80); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ansiTagPattern is the expression ansiTagLen replaced in the wrapper. It
// stays here as the oracle for what counts as a tag.
var ansiTagPattern = regexp.MustCompile(ansiOpenTagExpr + `|</ansi>`)

// ansiTagLen must agree with the expression at every less-than sign, on
// well-formed tags and on every malformed near miss these fragments build.
func TestAnsiTagLen_MatchesTheExpressionItReplaced(t *testing.T) {
	frags := []string{
		"<", ">", "<ansi", "</ansi", "</ansi>", "<ansi>", `<ansi fg="red">`, `<ansi fg=">">`,
		"<ANSI>", "</ANSI>", "</ansi >", "<ansix>", "<an", "</", " ", "\n", "a", `fg="x"`,
		"é", "\xff", "漢",
	}
	rng := rand.New(rand.NewSource(20261109))
	tagsSeen := 0
	for n := 0; n < 50000; n++ {
		var b strings.Builder
		for k := rng.Intn(10); k >= 0; k-- {
			b.WriteString(frags[rng.Intn(len(frags))])
		}
		s := b.String()
		for i := 0; i < len(s); i++ {
			if s[i] != '<' {
				continue
			}
			want := 0
			if loc := ansiTagPattern.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				want = loc[1]
				tagsSeen++
			}
			if got := ansiTagLen(s[i:]); got != want {
				t.Fatalf("ansiTagLen(%q) = %d, the expression says %d", s[i:], got, want)
			}
		}
	}
	if tagsSeen == 0 {
		t.Fatal("the corpus held no tags, so the comparison proved nothing")
	}
}

// columnsOf is the width a terminal gives one rendered line, by the library
// util.VisibleWidth uses and independent of the counter under test.
func columnsOf(line string) int {
	return runewidth.StringWidth(ansiTagPattern.ReplaceAllString(line, ""))
}

func TestWrapAnsi_WideCharactersCountTheirDisplayColumns(t *testing.T) {
	word := strings.Repeat("漢", 4) // four wide characters, 8 columns
	plain := strings.TrimSpace(strings.Repeat(word+" ", 40))

	cases := []struct {
		name string
		in   string
	}{
		{"plain", plain},
		{"tagged", `<ansi fg="saytext">` + plain + `</ansi>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(WrapAnsi(tc.in, 80), "\n")
			if len(lines) < 2 {
				t.Fatalf("a %d column text must wrap at 80, got one line", columnsOf(tc.in))
			}
			for i, line := range lines {
				if w := columnsOf(line); w > 80 {
					t.Errorf("line %d is %d display columns at width 80: %q", i+1, w, line)
				}
			}
		})
	}
}

func TestWrapAnsi_WideWordsBreakOnTheExactColumn(t *testing.T) {
	twenty := strings.Repeat("漢", 20) // 40 columns

	// 40 + 1 + 38 = 79 columns: fits.
	in := twenty + " " + strings.Repeat("漢", 19)
	if got := WrapAnsi(in, 80); got != in {
		t.Errorf("79 columns at width 80 must not wrap, got %q", got)
	}

	// 40 + 1 + 40 = 81 columns: breaks.
	want := twenty + "\n" + twenty
	if got := WrapAnsi(twenty+" "+twenty, 80); got != want {
		t.Errorf("81 columns at width 80 must wrap once, got %q", got)
	}
}

func TestWrapAnsi_CombiningMarksTakeNoColumn(t *testing.T) {
	// 40 columns + 1 + 39 = 80 columns, in 80 + 1 + 78 = 159 runes.
	in := strings.Repeat("é", 40) + " " + strings.Repeat("é", 39)
	if got := WrapAnsi(in, 80); got != in {
		t.Errorf("80 display columns at width 80 must not wrap, got %q", got)
	}
}
