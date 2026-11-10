package usercommands

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A queued message still carries its <ansi> tags (they become colour at
// delivery) and may carry escape codes and the wrap helper's \r.
var setScreenEscRE = regexp.MustCompile("\x1b\\[[0-9;]*m|</?ansi[^>]*>|\r")

// shippedFightPrompt is config.yaml's FightPrompt (2026-11-06, #591), 144
// columns raw. A test binary holds the Go default, so the screen test sets it.
const shippedFightPrompt = `{8}[{t} {255}HP:{hpbar} SP:{stbar} CP:{cvbar}{pos}{8}] {255}» {target}{8}[{targetpos}{8}|{targethealth}{8}]{tank}{tankpos}{tankbar}{239}{h}{8}:`

// renderedSetLines runs show, then returns what the player receives as lines,
// colour codes stripped.
func renderedSetLines(t *testing.T, u *users.UserRecord, show func()) []string {
	t.Helper()
	events.DrainQueuedMessagesForTest(u.UserId)
	show()
	var b strings.Builder
	for _, m := range events.DrainQueuedMessagesForTest(u.UserId) {
		b.WriteString(m)
	}
	text := setScreenEscRE.ReplaceAllString(b.String(), "")
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

// CategoryScreen never wraps, so the fprompt screen lays out its own lines to
// the reader: the header, the toggle rows and the closing sentence.
func TestSetFprompt_FitsANarrowReader(t *testing.T) {
	u := users.NewTestUser(8803, "narrow", "Narrow", 18803)
	u.LineWidth = 30

	lines := renderedSetLines(t, u, func() { _, _ = cmdSetFprompt(u, `fprompt`, `fprompt`, nil) })
	for i, line := range lines {
		if w := utf8.RuneCountInString(line); w > 30 {
			t.Errorf("line %d is %d columns, over 30: %q", i, w, line)
		}
	}
	joined := strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
	for _, want := range []string{
		"Toggle Settings (set fprompt tog <name> to change):",
		"Target health description",
		"Type help set-prompt for more info on customizing prompts.",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the screen lost %q:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

// #614: the set screens rode CategorySystem, whose normalizer stopped every
// label and value ("ScreenReader:.", "OFF.", "UTF-8.", "0%."), and each label
// and value sat on its own line. They are now laid-out blocks on
// CategoryScreen. This replaces #610's blank-line count for the set screen:
// rows no longer need a blank line between them.
func TestSetScreens_LaidOutRowsWithNoStops(t *testing.T) {
	u := users.NewTestUser(8802, "spacer", "Spacer", 18802)
	u.Character.Description = "A plain traveller."
	u.SetConfigOption(`fprompt`, shippedFightPrompt)

	screens := map[string]func(){
		"set":         func() { displaySetStatus(u) },
		"set fprompt": func() { _, _ = cmdSetFprompt(u, `fprompt`, `fprompt`, nil) },
	}
	// The one authored sentence on either screen.
	sentences := map[string]bool{
		"Type help set-prompt for more info on customizing prompts.": true,
	}

	for name, show := range screens {
		lines := renderedSetLines(t, u, show)
		if len(lines) < 5 {
			t.Fatalf("%s: rendered %d lines, want the whole screen: %q", name, len(lines), lines)
		}
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasSuffix(trimmed, ".") && !sentences[trimmed] && !strings.HasSuffix(trimmed, "traveller.") {
				t.Errorf("%s line %d gained a stop: %q", name, i, line)
			}
			if w := utf8.RuneCountInString(line); w > u.WrapWidth() {
				t.Errorf("%s line %d is %d columns, over %d: %q", name, i, w, u.WrapWidth(), line)
			}
			if strings.Count(line, "{") != strings.Count(line, "}") {
				t.Errorf("%s line %d splits a prompt {token} across lines: %q", name, i, line)
			}
			if i > 0 && trimmed == "" && strings.TrimSpace(lines[i-1]) == "" {
				t.Errorf("%s line %d is a second blank line in a row", name, i)
			}
		}
	}

	fprompt := renderedSetLines(t, u, func() { _, _ = cmdSetFprompt(u, `fprompt`, `fprompt`, nil) })
	for _, want := range []string{
		"Toggle Settings (set fprompt tog <name> to change):",
		"  targethealth   [ON]  Target health description",
		"Type help set-prompt for more info on customizing prompts.",
	} {
		if !containsLine(fprompt, want) {
			t.Errorf("at 80 columns the fprompt screen must keep the line %q:\n%s", want, strings.Join(fprompt, "\n"))
		}
	}

	set := strings.Join(renderedSetLines(t, u, func() { displaySetStatus(u) }), "\n")
	for _, want := range []string{"ScreenReader: OFF", "charset: UTF-8", "wimpy: 0%", "fprompt: {8}["} {
		if !strings.Contains(set, want) {
			t.Errorf("the set screen must show the row %q on one line:\n%s", want, set)
		}
	}
}
