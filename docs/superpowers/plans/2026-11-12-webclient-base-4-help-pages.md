# Web Client Base, Plan 4: Help Pages to 30 Columns Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every player help page reads correctly at 30, 42 and 80 columns, laid out to the reader's own screen width, with no guessing at run time about where a paragraph, a row or an example starts.

**Architecture:** Help files state their layout with four line markers (`::rows`, `::list`, `::keep`, closed by `::`); everything unmarked is a paragraph. A new pass, `templates.LayoutHelp`, runs after the template renders and lays each block out to the reader's width with the game's existing packer (`messaging.PackUnits`) and wrapper (`messaging.WrapAnsi`). `templates.ProcessHelp` renders at the reader's width without the screen-reader cleanup and runs that pass; every player help render goes through it. A one-time converter adds the markers to the 485 help files, and anything it cannot convert safely is converted by hand. A source guard refuses any line whose layout would otherwise be a guess; a render guard holds every page to 30, 42 and 80 and proves the layout pass never loses or reorders a word.

**Tech Stack:** Go 1.25 `text/template`, `testify`, guards in `internal/guards`, a Python 3 one-time converter in `tools/`.

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, section 2 (the "Aligned help pages" row, the "Hard line breaks in authored text" and "Rendering without a reader" bullets) and ruling 3. Mock (2026-11-12): https://claude.ai/artifact/7ZoZFjo34G591jbBjU4duY, source `_archive/notes/reports/help-pages-p4-mock/` in the main checkout (gitignored). Part of #783.

**Review gate.** This plan opens in VS Code for the owner's review. Nothing is built until he approves it.

---

## Owner rulings this plan follows (2026-11-12)

1. Help renders at the reader's width but **without** the screen-reader cleanup. `StripCharsForScreenReaders` turns `[`, `]` and `+` into spaces; in help that erases optional-argument brackets (`assist [name]`) and formula plus signs.
2. The help files that can never be rejoined (tables, art, grids, the species box) are fixed in plan 4.
3. Module help files join the width guard.
4. The web help pages (`modules/webhelp`) stay at 80 columns.
5. **Help files state their layout; the game never guesses.** A one-time conversion marks every block that is not a plain paragraph. A guard refuses ambiguous unmarked lines. A test proves no page loses or reorders a word. A report shows every converted page at 30.
6. Label rows use the **same-line form**: the label, then its text on the same line, wrapping two columns in; a label too long to share its line takes a line of its own, its text below, two in.

Earlier rulings in force: the spec's "one form everywhere" for aligned help (desktop values no longer line up in a column); units (commands, names) never split while they fit a line (2026-11-10); admin command screens are out of scope (spec ruling 25); every form shows all the information.

## Facts verified against source

Read 2026-11-12 from master `70e04d1fc` by grep, by reading, and by a probe test (species and spell pages rendered from Go in a scratch worktree, since removed).

| # | Fact | Where |
|---|---|---|
| 1 | `help <topic>` renders through `GetHelpContents`, which calls `templates.Process(..., 0)`: user 0, so `FullWidth` (80) and no screen-reader form | `internal/usercommands/help.go:225`, `:230`, `:233`; `internal/templates/forms.go:71-78` |
| 2 | The bare `help` index, `help/bounty` and `help/rename` render with the user's id, so screen-reader players already get the cleanup on those three | `help.go:80`; `admin.bounty.go:83`; `renameself.go:23` |
| 3 | Help output goes out on `CategoryScreen`, which the pipeline does not wrap | `help.go:99` |
| 4 | Admin help (`admincommands/help/command.<name>`) renders through `adminHelpFor` with the user's id; the width guard exempts `admincommands/` (ruling 25) | `help.go:117-135`; `internal/guards/template_width_guard_test.go:86`; spec line 93 |
| 5 | The web help page calls `usercommands.GetHelpContents(searchTerm)` and parses the result to HTML | `modules/webhelp/webhelp.go:125-129` |
| 6 | Other callers of `GetHelpContents`: four tests | `admin_earnmutation_test.go:73`, `help_species_test.go:29`, `help_spell_test.go:34`, `usercommands_test.go:4415`, `:4421` |
| 7 | Screen-reader cleanup replaces `┌─┐└┘╔═╗╚╝│─•]╒═╕█░╲╱+` and every `[` not after ESC with spaces, and `.:` with two spaces | `internal/util/util.go:1098-1137`, applied at `internal/templates/templates.go:289-290` |
| 8 | Of 467 live help files, 70 contain `[` and 18 contain `+` | grep of `_datafiles/world/dogmud/templates/help/*.template` |
| 9 | Help files: 467 in `_datafiles/world/dogmud/templates/help`, 18 in `modules/*/files/datafiles/templates/help` (aicompanion 8, auctions, cleanup, follow, gmcp 5, leaderboards, time). The 113 in `world/default` are not read: `DataFiles` is `_datafiles/world/dogmud` | `find`; `_datafiles/config.yaml:240` |
| 10 | Four player help files hold template logic: `help` (the index), `emote`, `species` (`range`) and `spell` (`with`, `if`) | grep `{{ range`, `{{ with`, `{{ if` |
| 11 | `templates.ProcessForWidth(fname, data, screenReader, width)` renders without the cleanup when `screenReader` is false, picking a form by width without measuring | `internal/templates/forms.go:105-107` |
| 12 | `templates.ProcessAtWidth` exists for guards only (in `testing_support.go`) | `internal/templates/testing_support.go:45-47` |
| 13 | `templates.FullWidth = users.MaxAutoWrapWidth`; `widthFuncs` caps `screenwidth` at it | `forms.go:25`, `:84-87` |
| 14 | `UserRecord.WrapWidth()` is the reader's width (line width on telnet; screen width capped at 80 on the web; floor `MinWrapWidth`) | `internal/users/wrapwidth.go:46-55` |
| 15 | `messaging.PackUnits(units, first, rest, width, names...)` lays units out with first and rest indents, breaking only between units; a unit too long for a line at `rest` is wrapped by `WrapAnsi` or broken at the edge | `internal/messaging/pack.go:35-37`, `:54-131` |
| 16 | `messaging.WrapAnsi` breaks only at the byte `' '`; a line that fits is copied byte for byte; at an embedded newline it closes every open tag and reopens them on the next line | `internal/messaging/wrap.go:52-59`, `:193-201`, `:259` |
| 17 | `util.VisibleWidth` is `runewidth.StringWidth` after stripping tags, so a control character such as `\x1f` (the keep marker `wrapSpans` uses with `WrapAnsi`) measures 0 there; U+00A0 measures 1 in both and is not a break for `WrapAnsi` | `internal/util/util.go:231-233`; `templatesfunctions.go:501-502` |
| 18 | `visibleRunes`, `rebuild` and `ansiTag` exist in package `templates` and are reusable | `templatesfunctions.go:613-647`, `:685` |
| 19 | No help file contains U+00A0 | grep |
| 20 | `charm.template` ends every line with CR CR LF in the working tree (51 lines, 102 CRs); the other help files are CRLF from `core.autocrlf` (LF in the repo; `.gitattributes` pins only Go, module and golden files) | `od -c`; `.gitattributes` |
| 21 | Every species help entry prints `Unarmed: jaws ()`: the template prints `.Damage.DiceRoll`, empty for species that set `basedamage` | `templates/help/species.template` (Unarmed line); probe render 2026-11-12 |
| 22 | The template width guard holds `help/` as a class ("plan 4") and reads module template folders | `template_width_guard_test.go:36-50`, `:83` |
| 23 | Guard tests run from the repo root (`TestMain` chdirs) and render with `templateRenderSetup` | `internal/guards/testmain_test.go:70-85`; `template_width_guard_test.go:378-392` |
| 24 | `species.LoadForTest(t)` and `spells.LoadSpellFiles()` load the real data relative to the repo root; `conditions.LoadDataFiles()` panics under test config (condition 0's `TriggersCount`) | `internal/species/testing_support.go:30`; probe 2026-11-12 |
| 25 | Over-80 player help pages today (survey 2026-11-11, widest rendered source line): alchemy 91, auction 81, blacksmithing 86, character 101, checkclient 97, client 97, companion-stay 83, craft 82, discord 110, dogmud 82, guild 83, jewelcrafting 87, killstats 103, leaderboard 84, mana 84, mire-renewal-draught 86, mudletui 107, mutations 84, picklock-example 106, pvp 112, quit 101, serpents-edge 128, set-prompt 87, species 133, time 100 | `C:\Users\CALABE~1\AppData\Local\Temp\claude\1028a153ef5b922d\0e21d696-e31a-43ba-903e-3a7900b0d2df\scratchpad\p4probe\out\widths.tsv` (may be swept; the converter's report re-measures) |
| 26 | Survey classes (2026-11-11 heuristic, to be re-derived by the converter): box drawing in keyring, online, picklock, picklock-example, species; ASCII art in adjectives, map-examples, set, submission; three or more aligned columns in alchemy, blacksmithing, colors, help, jewelcrafting, keyring, map, map-examples, mutations, online, picklock, picklock-example, pvp, set, set-prompt, species | same folder, `classify.json` |

## Decisions taken without asking

Each lists why, and what changes if it is wrong.

**Worth your eye first:**

1. **The markup** (approved by the owner 2026-11-12). A help file says how each block is laid out, so the game reads it instead of guessing from spacing. A block that is not a plain paragraph gets a marker line above it and `::` below it; the text between stays exactly as authors write it today, and the marker lines never reach the player. `::rows` is a label and its text (usage lists, `Name: value`, `See also:`); `::list` is items starting `- `, `* ` or `1. `; `::keep` is lines never joined or rewrapped (`quaff poultice` over `quaff draught`). Unmarked text is paragraphs, rejoined and rewrapped when they do not fit. A line ending in `:` ends its paragraph, so a lead-in such as `Species are affected by Size:` may sit directly above a block. Example, `look`'s usage list:

   ```
   Usage:
   ::rows
     look              Look at the room you are in.
     look [thing]      Look closely at a colored word in the room.
   ::
   ```

   Rejected: template functions such as `{{ row "look" "Look at the room" }}`, far noisier across 485 files and harder for the next help author.
2. **The bare `help` index, `help bounty` and `help rename` lose the screen-reader cleanup too** (fact 2), so every player help page follows ruling 1, not only `help <topic>`. If wrong: those three keep `Process` with the user id and stay at 80.
3. **Paragraphs and lists keep their authored line breaks wherever every line fits the screen; label rows always take the same-line form** (the spec's "one form everywhere"). So a desktop sees today's prose and lists, and a usage list changes from an aligned column to `label  text`. If wrong: rows also keep their authored alignment when they fit, which leaves desktops unchanged and is a one-line change in `layoutHelpEntries`.
4. **The command grids in the `help` index and `emote` become flowing lists**, packed to the screen at every width, since a grid of 17-column cells cannot fit 30. If wrong: a `.w30` form for those two templates keeps the grid at 80.
5. **The species page drops the empty `()` after the unarmed attack** instead of showing the damage (fact 21): the player-copy rule forbids raw damage numbers.

**The rest:**

6. Separator in a row: two spaces, or one when the label ends in `:` (`Name: Blood Boil`), as the mock drew.
7. A rule heading (`━━━ Title ━━━`) too wide for the screen drops rule marks, three, then two, then one, then none, until it fits.
8. A `::keep` line must be at most 30 columns wide, so it never runs past any screen. The only exception is a template in the guard's `wide` class; this plan adds `help/map-examples` (ASCII maps, which ruling 3 lets scroll sideways) and reports any other addition in the PR.
9. Width is `min(reader width, FullWidth)`: a telnet reader with `set linewidth 120` sees help laid out at 80, as other layouts are (fact 13).
10. A `cmd - what it does` line becomes the row `cmd  what it does`: the converter drops the lone `-`. The word check ignores lone `-`.
11. A `See also:` line becomes a one-row `::rows` block, so its list wraps two in.
12. The converter (`tools/help_markup_convert.py`) is committed for the record and for module authors. It writes only to an output folder; its output is copied into place (global rule H3).
13. The layout pass joins the words of a unit with U+00A0 while packing, not `\x1f` (fact 17), and the guard refuses U+00A0 in help files.
14. The spec's help bullets get a dated amendment line pointing at ruling 5; their text is not rewritten.
15. Admin help stays on `Process` and is not converted (ruling 25); the `admincommands/` guard class stays.

## The markup, for authors

```
.: Help for look                       <- a paragraph of one line
                                       
The look command shows you what is     <- a paragraph: its lines join and
around you.                               rewrap when a line does not fit
                                       
Usage:                                 <- ends in ":", so it ends its paragraph
::rows
  look              Look at the room.  <- a row: label, two or more spaces, text
  look [thing]      Look closely at a
                    colored word.      <- deeper indent: the row continues
::
::keep
  quaff poultice                       <- kept exactly, never joined (<= 30 wide)
  quaff draught
::
::list
  - Heavy armor (reduces mobility)     <- an item: "- ", "* " or "1. "
  - Being prone                           continuation lines go deeper
::
```

Rules the guard enforces (Task 3): outside a block, a run of lines keeps one indent (a new indent may start only after a line ending in `:`), has no run of two or more spaces after its first visible character, starts no line with a list marker, has no line that is only a command or skill name, and puts any `━`/`─` line alone as a heading. Inside `::rows` every entry has its gap; inside `::list` every entry has its marker; continuation lines are deeper than the entry. Markers open at column 0, close with `::`, and never nest.

## File structure

| File | Change |
|---|---|
| `internal/templates/helplayout.go` | Create: `LayoutHelp`, `ProcessHelp` and their helpers |
| `internal/templates/helplayout_test.go` | Create: layout unit tests |
| `internal/usercommands/help.go` | Modify: `GetHelpContents(input, width)`, index via `ProcessHelp` |
| `internal/usercommands/admin.bounty.go`, `renameself.go` | Modify: `ProcessHelp` with the reader's width |
| `internal/usercommands/help_reader_test.go` | Create: width and screen-reader behaviour |
| `internal/usercommands/{admin_earnmutation,help_species,help_spell,usercommands}_test.go` | Modify: new `GetHelpContents` signature |
| `modules/webhelp/webhelp.go` | Modify: pass `templates.FullWidth` |
| `tools/help_markup_convert.py` | Create: the one-time converter |
| `internal/guards/help_markup_guard_test.go` | Create: the source guard |
| `internal/guards/help_render_guard_test.go` | Create: widths, word preservation, the report dump |
| `internal/guards/template_width_guard_test.go` | Modify: retire the `help/` class, add `help/map-examples` to `wide` |
| 485 help templates | Modify: markers (converter), hand conversions |
| `internal/templates/context.md`, `internal/usercommands/context.md`, `modules/webhelp/context.md`, `internal/guards/context.md` | Modify: document the new surface |
| `.claude/skills/dogmud-player-copy/SKILL.md` | Modify: the markup, for help authors |
| `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md` | Modify: dated amendment line |
| `docs/PATCH_NOTES.md`, `docs/README.md` | Modify: patch note; index this plan |

---

### Task 0: Worktree

- [ ] **Step 1: Create the worktree and branch**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/webclient-base-4-help "C:/tmp/dogmud-p4" origin/master
```

Record `C:/tmp/dogmud-p4` in memory (`project-touch-arc-base-first`) so the tmp sweep keeps it. Every later step runs in `C:/tmp/dogmud-p4`.

- [ ] **Step 2: Baseline**

Run: `go build ./... && go test ./internal/templates/ ./internal/usercommands/ ./internal/guards/ -count=1`
Expected: PASS. If not, stop and report: the baseline is broken before any change.

---

### Task 1: The layout pass

**Files:**
- Create: `internal/templates/helplayout.go`
- Test: `internal/templates/helplayout_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package templates

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestLayoutHelp(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"a paragraph that fits keeps its breaks",
			"Hello world.\nSecond line.", 30,
			"Hello world.\nSecond line."},
		{"a paragraph that does not fit rewraps",
			"The quick brown fox jumps over\nthe lazy dog.", 20,
			"The quick brown fox\njumps over the lazy\ndog."},
		{"an indented paragraph keeps its indent",
			"  aaa bbb ccc ddd", 10,
			"  aaa bbb\n  ccc ddd"},
		{"a line ending in a colon ends its paragraph",
			"Usage:\n  look at the very long thing now", 20,
			"Usage:\n  look at the very\n  long thing now"},
		{"a row takes the same-line form and wraps two in",
			"::rows\n  look  Look at the room you are in.\n::", 30,
			"  look  Look at the room you\n    are in."},
		{"a row is laid out the same way at 80",
			"::rows\n  look          Look at the room.\n::", 80,
			"  look  Look at the room."},
		{"a label ending in a colon takes one space",
			"::rows\nName:        Blood Boil\n::", 30,
			"Name: Blood Boil"},
		{"a label too long to share its line takes its own",
			"::rows\n  bank withdraw [amount/all]  Withdraw gold from the bank\n::", 30,
			"  bank withdraw [amount/all]\n    Withdraw gold from the\n    bank"},
		{"a row's deeper lines continue it",
			"::rows\n  look  Look at\n        the room.\n::", 80,
			"  look  Look at the room."},
		{"a list item hangs under its text when it does not fit",
			"::list\n  - Heavy armor (reduces mobility)\n::", 20,
			"  - Heavy armor\n    (reduces\n    mobility)"},
		{"a list that fits keeps its lines",
			"::list\n  - Heavy armor\n  - Being prone\n::", 30,
			"  - Heavy armor\n  - Being prone"},
		{"kept lines are never joined",
			"::keep\n  quaff poultice\n  quaff draught\n::", 30,
			"  quaff poultice\n  quaff draught"},
		{"a rule heading drops marks until it fits",
			`<ansi fg="yellow">━━━ What Kind of Game Is This? ━━━</ansi>`, 30,
			`<ansi fg="yellow">━ What Kind of Game Is This? ━</ansi>`},
		{"a command that fits a line is never split",
			`See <ansi fg="command">help item-names</ansi> now`, 16,
			"See\n" + `<ansi fg="command">help item-names</ansi>` + "\nnow"},
		{"blank lines inside and between blocks are kept",
			"One.\n\n::rows\n  a  b\n\n  c  d\n::\n\nTwo.", 30,
			"One.\n\n  a  b\n\n  c  d\n\nTwo."},
		{"carriage returns are dropped",
			"a\r\nb", 30,
			"a\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, LayoutHelp(c.in, c.width))
		})
	}
}

// A colour span cut by a line break is closed at the end of the line and
// reopened on the next, so each line stands alone (WrapAnsi's rule).
func TestLayoutHelpBalancesTagsAtEachBreak(t *testing.T) {
	out := LayoutHelp(`<ansi fg="white">aaa bbb ccc</ansi>`, 7)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 2)
	for _, l := range lines {
		assert.Equal(t, strings.Count(l, "<ansi"), strings.Count(l, "</ansi>"), l)
		assert.LessOrEqual(t, util.VisibleWidth(l), 7, l)
	}
}

// The spell page's labels carry their padding inside the colour span.
func TestLayoutHelpRowLabelPaddedInsideItsSpan(t *testing.T) {
	in := "::rows\n" + `<ansi fg="yellow">SpellID:     </ansi> <ansi fg="white-bold">blood-boil</ansi>` + "\n::"
	out := LayoutHelp(in, 30)
	assert.Equal(t, "SpellID: blood-boil", ansiTag.ReplaceAllString(out, ""))
}

// Directive lines never reach the reader, even an unclosed block.
func TestLayoutHelpDropsMarkers(t *testing.T) {
	out := LayoutHelp("::keep\n  x\n", 30)
	assert.NotContains(t, out, "::")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/templates/ -run 'TestLayoutHelp' -count=1`
Expected: FAIL to compile, `undefined: LayoutHelp`.

- [ ] **Step 3: Write the layout pass**

```go
package templates

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Help markup (web client base plan 4, owner ruling 2026-11-12: help files
// state their layout and the game never guesses). Four marker lines at
// column 0: "::rows", "::list" and "::keep" open a block and "::" closes it.
// Everything outside a block is paragraphs. internal/guards'
// help_markup_guard_test.go holds every help file to these rules.
const (
	helpMarkRows = "::rows"
	helpMarkList = "::list"
	helpMarkKeep = "::keep"
	helpMarkEnd  = "::"
)

// helpJoin holds the words of one unit together while it is packed: one
// column to util.VisibleWidth and to WrapAnsi, and never a break. \x1f will
// not do here: VisibleWidth measures it as 0.
const helpJoin = "\u00a0"

var (
	helpListMarker = regexp.MustCompile(`^(- |\* |\d+\. )`)
	helpGap        = regexp.MustCompile(`\S( {2,})\S`)
	helpHeading    = regexp.MustCompile(`^([━─]+) (.+) ([━─]+)$`)
	helpCloseTag   = regexp.MustCompile(`^</ansi>`)
)

// ProcessHelp renders a help template for a reader whose screen is width
// columns wide, without the screen-reader cleanup (owner ruling 2026-11-12:
// that cleanup blanks "[", "]" and "+", which help uses for optional
// arguments and formulas), and lays it out to that width.
func ProcessHelp(fname string, data any, width int) (string, error) {
	out, err := ProcessForWidth(fname, data, false, width)
	if err != nil {
		return out, err
	}
	return LayoutHelp(out, width), nil
}

type helpBlock struct {
	kind  string // "para", "rows", "list", "keep" or "blank"
	lines []string
}

// LayoutHelp lays a rendered help page out to width columns (at most
// FullWidth). A paragraph or list whose lines all fit keeps its authored
// breaks; one that does not is rejoined and packed. A row always takes the
// same-line form: its label, then its text wrapping two columns in. A kept
// line is never touched. Marker lines are dropped.
func LayoutHelp(text string, width int) string {
	width = min(width, FullWidth)
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	out := []string{}
	for _, b := range helpBlocks(lines) {
		switch b.kind {
		case "blank", "keep":
			out = append(out, b.lines...)
		case "para":
			out = append(out, layoutHelpPara(b.lines, width)...)
		case "list":
			out = append(out, layoutHelpEntries(b.lines, width, false)...)
		case "rows":
			out = append(out, layoutHelpEntries(b.lines, width, true)...)
		}
	}
	return strings.Join(out, "\n")
}

// helpBlocks splits lines into blocks: at blank lines, at marker lines, and
// after a paragraph line that ends in ":".
func helpBlocks(lines []string) []helpBlock {
	blocks := []helpBlock{}
	open := ""      // the open marker's kind, "" outside a block
	boundary := true // the next line starts a new block
	for _, line := range lines {
		if open == "" && (line == helpMarkRows || line == helpMarkList || line == helpMarkKeep) {
			open, boundary = strings.TrimPrefix(line, "::"), true
			continue
		}
		if open != "" && line == helpMarkEnd {
			open, boundary = "", true
			continue
		}
		if strings.TrimSpace(ansiTag.ReplaceAllString(line, "")) == "" {
			blocks = append(blocks, helpBlock{"blank", []string{line}})
			boundary = true
			continue
		}
		kind := open
		if kind == "" {
			kind = "para"
		}
		if boundary || blocks[len(blocks)-1].kind != kind {
			blocks = append(blocks, helpBlock{kind, nil})
		}
		blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, line)
		boundary = kind == "para" && strings.HasSuffix(strings.TrimRight(helpVisible(line), " "), ":")
	}
	return blocks
}

func helpVisible(s string) string { return ansiTag.ReplaceAllString(s, "") }

func helpIndent(s string) int {
	v := helpVisible(s)
	return len(v) - len(strings.TrimLeft(v, " "))
}

func helpFits(lines []string, width int) bool {
	for _, l := range lines {
		if util.VisibleWidth(strings.TrimRight(l, " ")) > width {
			return false
		}
	}
	return true
}

func layoutHelpPara(lines []string, width int) []string {
	if helpFits(lines, width) {
		return lines
	}
	if len(lines) == 1 {
		if h, ok := shrinkHelpHeading(lines[0], width); ok {
			return []string{h}
		}
	}
	indent := helpIndent(lines[0])
	return packHelp(strings.Join(lines, " "), nil, indent, indent, width)
}

// shrinkHelpHeading drops rule marks from a "━━━ Title ━━━" heading until it
// fits, keeping its colour tags.
func shrinkHelpHeading(line string, width int) (string, bool) {
	runes := visibleRunes(line)
	if len(runes) == 0 {
		return "", false
	}
	v := string(func() []rune {
		r := make([]rune, len(runes))
		for i, x := range runes {
			r[i] = x.r
		}
		return r
	}())
	m := helpHeading.FindStringSubmatch(v)
	if m == nil {
		return "", false
	}
	mark := []rune(m[1])[0]
	before, after := line[:runes[0].start], line[runes[len(runes)-1].end:]
	for n := len([]rune(m[1])); n >= 0; n-- {
		cand := m[2]
		if n > 0 {
			cand = strings.Repeat(string(mark), n) + " " + m[2] + " " + strings.Repeat(string(mark), n)
		}
		if util.VisibleWidth(cand) <= width {
			return before + cand + after, true
		}
	}
	return "", false
}

// layoutHelpEntries lays out a ::rows or ::list block. An entry starts at the
// block's least indent; deeper lines continue it.
func layoutHelpEntries(lines []string, width int, rows bool) []string {
	if !rows && helpFits(lines, width) {
		return lines
	}
	base := helpIndent(lines[0])
	for _, l := range lines {
		base = min(base, helpIndent(l))
	}
	entries := [][]string{}
	for _, l := range lines {
		if helpIndent(l) == base || len(entries) == 0 {
			entries = append(entries, []string{l})
			continue
		}
		entries[len(entries)-1] = append(entries[len(entries)-1], l)
	}
	out := []string{}
	for _, e := range entries {
		head, rest := e[0], strings.Join(e[1:], " ")
		v := []rune(helpVisible(head))
		body := string(v[base:])
		var cut int // visible runes of head that make the label or marker
		if rows {
			m := helpGap.FindStringSubmatchIndex(body)
			if m == nil {
				out = append(out, packHelp(strings.Join(e, " "), nil, base, base+2, width)...)
				continue
			}
			cut = base + len([]rune(body[:m[2]]))
		} else {
			m := helpListMarker.FindString(body)
			if m == "" {
				out = append(out, packHelp(strings.Join(e, " "), nil, base, base+2, width)...)
				continue
			}
			cut = base + len([]rune(m)) - 1 // the marker without its space
		}
		label, text := cutHelpVisible(head, cut)
		text = strings.TrimSpace(text + " " + rest)
		label = trimHelpVisible(label)
		if !rows {
			hang := base + len([]rune(helpListMarker.FindString(body)))
			out = append(out, packHelp(text, []string{label}, base, hang, width)...)
			continue
		}
		var lead []string
		if util.VisibleWidth(label) <= width-base {
			lead = []string{protectHelpSpaces(label)}
		} else {
			lead = helpUnits(label, width-base-2)
		}
		if !strings.HasSuffix(helpVisible(label), ":") {
			lead[len(lead)-1] += helpJoin
		}
		out = append(out, packHelp(text, lead, base, base+2, width)...)
	}
	return out
}

// cutHelpVisible splits s after its first n visible runes. Closing tags right
// after the cut (past any spaces) go with the left part, so a label's own
// colour closes with it.
func cutHelpVisible(s string, n int) (string, string) {
	runes := visibleRunes(s)
	if n <= 0 {
		return "", s
	}
	if n >= len(runes) {
		return s, ""
	}
	at := runes[n-1].end
	for j := at; j < len(s); {
		if s[j] == ' ' {
			j++
			continue
		}
		if m := helpCloseTag.FindString(s[j:]); m != "" {
			j += len(m)
			at = j
			continue
		}
		break
	}
	return s[:at], s[at:]
}

// trimHelpVisible drops visible spaces at both ends of s, keeping its tags.
func trimHelpVisible(s string) string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	first, last := -1, -1
	for i, v := range runes {
		what[i] = s[v.start:v.end]
		if v.r != ' ' {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	for i := range runes {
		if i < first || i > last {
			what[i] = ""
		}
	}
	return rebuild(s, runes, what)
}

// protectHelpSpaces joins every visible space of s, so s packs as one unit.
func protectHelpSpaces(s string) string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	for i, v := range runes {
		what[i] = s[v.start:v.end]
		if v.r == ' ' {
			what[i] = helpJoin
		}
	}
	return rebuild(s, runes, what)
}

// helpUnits splits s into the units PackUnits places: at spaces, except
// inside a colour span no wider than limit, which stays one unit (a command
// or a name never splits while it fits a line, owner 2026-11-10). Tags with
// no visible text go with the next unit.
func helpUnits(s string, limit int) []string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	for i, v := range runes {
		what[i] = s[v.start:v.end]
	}
	opens := []int{}
	for _, t := range ansiTag.FindAllStringIndex(s, -1) {
		if !strings.HasPrefix(s[t[0]:t[1]], "</") {
			opens = append(opens, t[1])
			continue
		}
		if len(opens) == 0 {
			continue
		}
		open := opens[len(opens)-1]
		opens = opens[:len(opens)-1]
		w, inside := 0, []int{}
		for i, v := range runes {
			if v.start >= open && v.end <= t[0] {
				w += util.VisibleWidth(string(v.r))
				inside = append(inside, i)
			}
		}
		if w == 0 || w > limit {
			continue
		}
		for _, i := range inside {
			if runes[i].r == ' ' {
				what[i] = helpJoin
			}
		}
	}
	units, pending := []string{}, ""
	for _, u := range strings.Split(rebuild(s, runes, what), " ") {
		if util.VisibleWidth(u) == 0 {
			pending += u
			continue
		}
		units = append(units, pending+u)
		pending = ""
	}
	if pending != "" {
		if len(units) == 0 {
			return []string{pending}
		}
		units[len(units)-1] += pending
	}
	return units
}

// packHelp packs lead and the units of text into lines: the first indented
// by first, the rest by rest. WrapAnsi then closes and reopens colour at each
// line break, so every line stands alone.
func packHelp(text string, lead []string, first, rest, width int) []string {
	units := append(append([]string{}, lead...), helpUnits(text, width-rest)...)
	lines := messaging.PackUnits(units, first, rest, width)
	joined := messaging.WrapAnsi(strings.Join(lines, "\n"), width)
	out := strings.Split(strings.ReplaceAll(joined, helpJoin, " "), "\n")
	for i := range out {
		out[i] = strings.TrimRight(out[i], " ")
	}
	return out
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/templates/ -run 'TestLayoutHelp' -count=1 -v`
Expected: PASS. Where a case fails, read the diff before changing code: the expected strings above were worked by hand, column by column, against `PackUnits`' rules (a unit goes on the line in hand when `used + 1 + width <= limit`). Fix the code when the expected text matches the rulings; fix the expectation only when it miscounted, and say which in the commit message.

- [ ] **Step 5: Run the package**

Run: `go test ./internal/templates/ -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/templates/helplayout.go internal/templates/helplayout_test.go
git commit -F - <<'EOF'
feat(templates): help layout pass for the help markup (plan 4)

LayoutHelp lays a rendered help page out to the reader's width from the
markers the page states (::rows, ::list, ::keep); ProcessHelp renders
without the screen-reader cleanup and runs it.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Help renders at the reader's width

**Files:**
- Modify: `internal/usercommands/help.go:80`, `:87`, `:177`, `:225`, `:230`, `:233`
- Modify: `internal/usercommands/admin.bounty.go:83`, `internal/usercommands/renameself.go:23`
- Modify: `modules/webhelp/webhelp.go:125`
- Modify: `internal/usercommands/admin_earnmutation_test.go:73`, `help_species_test.go:29`, `help_spell_test.go:34`, `usercommands_test.go:4415`, `:4421`
- Test: `internal/usercommands/help_reader_test.go`

- [ ] **Step 1: Write the failing test**

The setup is the one `help_species_test.go` and `help_spell_test.go` use: `seedAllRegistries()` and `useDogmudTemplates(t)`, both existing helpers in this package's tests.

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Help is laid out to the width it is asked for, and keeps the characters
// the screen-reader cleanup would blank (owner ruling 2026-11-12).
func TestGetHelpContentsUsesTheReadersWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	for _, width := range []int{30, 42, 80} {
		out, err := GetHelpContents("look", width)
		require.NoError(t, err)
		assert.LessOrEqual(t, templates.WidestLine(out), width, "width %d:\n%s", width, out)
		assert.Contains(t, out, "[thing]", "brackets survive at %d", width)
		assert.NotContains(t, out, "::", "markers never reach the reader")
	}
}

func TestGetHelpContentsDifferBetweenWidths(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	narrow, err := GetHelpContents("look", 30)
	require.NoError(t, err)
	wide, err := GetHelpContents("look", 80)
	require.NoError(t, err)
	assert.Greater(t, strings.Count(narrow, "\n"), strings.Count(wide, "\n"))
}
```

`look` must already carry its markers for the first test's `::` and width assertions to be meaningful; until Task 4 lands it, the width assertion at 30 fails, which is the red this step wants.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/usercommands/ -run 'TestGetHelpContents(UsesTheReadersWidth|DifferBetweenWidths)' -count=1`
Expected: FAIL to compile: `too many arguments in call to GetHelpContents`.

- [ ] **Step 3: Change the render path**

In `help.go`:

```go
// line 80, the bare index
		helpTxt, err = templates.ProcessHelp("help/help", helpCommandList, user.WrapWidth())
// line 87
		helpTxt, err = GetHelpContents(rest, user.WrapWidth())
```

```go
// GetHelpContents renders the help page for input, laid out for a screen
// width columns wide (the reader's WrapWidth in game; templates.FullWidth on
// the web help pages, owner ruling 2026-11-12).
func GetHelpContents(input string, width int) (string, error) {
```

and the three renders at `:225`, `:230`, `:233`:

```go
	result, err := templates.ProcessHelp("help/"+helpName, helpVars, width)
	...
			return templates.ProcessHelp("help/spell", sData, width)
	...
			return templates.ProcessHelp("help/spell", sData, width)
```

`adminHelpFor` (`:117-135`) is unchanged (ruling 25).

`admin.bounty.go:83`: `templates.ProcessHelp("help/bounty", nil, user.WrapWidth())`.
`renameself.go:23`: `templates.ProcessHelp("help/rename", nil, user.WrapWidth())`.
`webhelp.go:125`: `usercommands.GetHelpContents(searchTerm, templates.FullWidth)`.

Update the four test files' calls to pass `80` (`GetHelpContents("mutations", 80)` and so on).

- [ ] **Step 4: Build and run the package**

Run: `go build ./... && go test ./internal/usercommands/ ./modules/webhelp/ -count=1`
Expected: build passes; the new width test still fails at 30 (no markers yet), everything else passes. Note the failure and continue: Task 4 turns it green.

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/help.go internal/usercommands/admin.bounty.go internal/usercommands/renameself.go modules/webhelp/webhelp.go internal/usercommands/help_reader_test.go internal/usercommands/admin_earnmutation_test.go internal/usercommands/help_species_test.go internal/usercommands/help_spell_test.go internal/usercommands/usercommands_test.go
git commit -F - <<'EOF'
feat(help): render help at the reader's width, without screen-reader cleanup

Every player help render goes through templates.ProcessHelp at the
reader's WrapWidth; the web help pages stay at 80. Admin help is
unchanged (spec ruling 25). help_reader_test is red at 30 until the
help files carry their markers.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: The source guard

**Files:**
- Create: `internal/guards/help_markup_guard_test.go`

- [ ] **Step 1: Write the guard**

```go
package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Help files state their layout and the game never guesses (web client base
// plan 4, owner ruling 2026-11-12). Every line whose layout would otherwise
// be a guess must sit in a marked block. See internal/templates/helplayout.go
// for what the markers mean.

var (
	helpTag        = regexp.MustCompile(`</?ansi[^>]*>`)
	helpAction     = regexp.MustCompile(`^\s*(\{\{[^}]*\}\}\s*)+$`)
	helpGapRun     = regexp.MustCompile(`\S {2,}\S`)
	helpListStart  = regexp.MustCompile(`^(- |\* |\d+\. )`)
	helpLoneUnit   = regexp.MustCompile(`^\s*<ansi fg="(command|skill)">[^<]*</ansi>\s*$`)
	helpRuleLine   = regexp.MustCompile(`[━─]`)
	helpHeadingVis = regexp.MustCompile(`^[━─]+ .+ [━─]+$`)
)

// helpWide are help templates whose kept lines may run past 30 (ruling 3:
// maps may scroll sideways). Keep in step with templateClassNames["wide"].
var helpWide = map[string]bool{"help/map-examples": true}

func helpTemplateFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, folder := range templateFolders(t) {
		matches, err := filepath.Glob(filepath.Join(folder, "help", "*.template"))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			files["help/"+strings.TrimSuffix(filepath.Base(m), ".template")] = m
		}
	}
	if len(files) < 480 {
		t.Fatalf("found %d help templates; the folders are not being reached", len(files))
	}
	return files
}

func TestHelpFilesStateTheirLayout(t *testing.T) {
	for name, path := range helpTemplateFiles(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if strings.Contains(text, "\r") {
			t.Errorf("%s: a carriage return is left after CRLF is removed", name)
		}
		if strings.Contains(text, "\u00a0") {
			t.Errorf("%s: a no-break space (the layout pass's own joiner)", name)
		}
		for _, p := range helpMarkupProblems(name, strings.Split(text, "\n")) {
			t.Errorf("%s: %s", name, p)
		}
	}
}

func helpMarkupProblems(name string, lines []string) []string {
	problems := []string{}
	add := func(n int, msg string) { problems = append(problems, sprintf("line %d: %s", n+1, msg)) }
	open, base := "", -1
	prevVis, prevIndent := "", -1 // the previous unmarked line in this run, "" after a blank
	for i, line := range lines {
		vis := helpTag.ReplaceAllString(line, "")
		trimmed := strings.TrimRight(vis, " ")
		switch {
		case line == "::rows" || line == "::list" || line == "::keep":
			if open != "" {
				add(i, "a block opens inside "+open)
			}
			open, base, prevVis = line, -1, ""
			continue
		case line == "::":
			if open == "" {
				add(i, `"::" closes no block`)
			}
			open, prevVis = "", ""
			continue
		case strings.HasPrefix(strings.TrimSpace(vis), "::"):
			add(i, "an unknown marker or a marker not at column 0")
			continue
		}
		if helpAction.MatchString(line) {
			continue // a template action alone prints no line
		}
		if strings.TrimSpace(vis) == "" {
			prevVis, prevIndent = "", -1
			continue
		}
		indent := len(vis) - len(strings.TrimLeft(vis, " "))
		body := strings.TrimLeft(vis, " ")
		switch open {
		case "::rows", "::list":
			if base < 0 {
				base = indent
			}
			if indent < base {
				add(i, "a line left of its block's first entry")
			}
			if indent == base {
				if open == "::rows" && !helpGapRun.MatchString(body) {
					add(i, "a row with no gap of two or more spaces between label and text")
				}
				if open == "::list" && !helpListStart.MatchString(body) {
					add(i, `a list entry without "- ", "* " or "1. "`)
				}
			}
			continue
		case "::keep":
			if !helpWide[name] && len([]rune(trimmed)) > 30 {
				add(i, "a kept line wider than 30 columns")
			}
			continue
		}
		// Unmarked: paragraphs only.
		if helpRuleLine.MatchString(vis) {
			if indent != 0 || !helpHeadingVis.MatchString(trimmed) {
				add(i, "a rule line that is not a heading")
			}
			if prevVis != "" || (i+1 < len(lines) && strings.TrimSpace(helpTag.ReplaceAllString(lines[i+1], "")) != "") {
				add(i, "a heading not alone between blank lines")
			}
		}
		if helpGapRun.MatchString(body) {
			add(i, "two or more spaces inside a line: mark it ::rows or ::keep")
		}
		if helpListStart.MatchString(body) {
			add(i, "a list marker: mark it ::list")
		}
		if helpLoneUnit.MatchString(line) {
			add(i, "a line that is only a command: mark it ::keep")
		}
		if prevVis != "" && indent != prevIndent && !strings.HasSuffix(strings.TrimRight(prevVis, " "), ":") {
			add(i, "the indent changes inside a paragraph: mark the block, or end the line above with \":\"")
		}
		prevVis, prevIndent = vis, indent
	}
	if open != "" {
		add(len(lines)-1, open+" is never closed")
	}
	return problems
}
```

Use `fmt.Sprintf` for `sprintf` (add `"fmt"` to the imports); it is written short here for width.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/guards/ -run TestHelpFilesStateTheirLayout -count=1 2>&1 | tail -5`
Expected: FAIL with problems in most help files (nothing is converted yet). Do not commit yet: the guard lands with the converted files in Task 5, so every commit on the branch is green.

- [ ] **Step 3: Prove each rule can fail**

Add a table test beside the guard that feeds `helpMarkupProblems` one bad snippet per rule and asserts a problem, and the plan's "markup, for authors" example (as lines) and asserts none:

```go
func TestHelpMarkupRulesCanFail(t *testing.T) {
	bad := map[string][]string{
		"gap":          {"  look      Look at the room."},
		"list":         {"- an item"},
		"lone command": {`  <ansi fg="command">quaff poultice</ansi>`},
		"indent":       {"A paragraph line", "  indented after it"},
		"heading":      {"Text", "━━━ Title ━━━"},
		"open":         {"::rows", "  a  b"},
		"close":        {"::"},
		"nest":         {"::rows", "::keep", "::", "::"},
		"row gap":      {"::rows", "  a b", "::"},
		"list marker":  {"::list", "  item", "::"},
		"keep width":   {"::keep", strings.Repeat("x", 31), "::"},
	}
	for name, lines := range bad {
		assert.NotEmpty(t, helpMarkupProblems("help/x", lines), name)
	}
	good := []string{
		".: Help for look", "", "The look command shows you what is", "around you.", "", "Usage:",
		"::rows", "  look              Look at the room.", "  look [thing]      Look closely at a",
		"                    colored word.", "::", "::keep", "  quaff poultice", "  quaff draught", "::",
		"::list", "  - Heavy armor (reduces mobility)", "  - Being prone", "::",
	}
	assert.Empty(t, helpMarkupProblems("help/x", good))
}
```

Run: `go test ./internal/guards/ -run TestHelpMarkupRulesCanFail -count=1`
Expected: PASS.

---

### Task 4: Convert the help files

**Files:**
- Create: `tools/help_markup_convert.py`
- Modify: help templates (copied into place from the converter's output)

- [ ] **Step 1: Write the converter**

```python
"""One-time conversion of DOGMud help files to the help markup (plan 4, 2026-11-12).

Reads every player help template (the live world and every module), writes the converted
copy under OUT at the same relative path, and prints a report. It never writes into the
repo: land its output with cp (global rule H3). A file whose words would change is written
unchanged and listed. Spots it cannot convert safely are left as they are and listed under
FLAGGED; Task 5 converts them by hand. The guard (help_markup_guard_test.go) is the judge.

usage: python -I tools/help_markup_convert.py <repo> <out>
"""
import glob, os, re, sys

TAG = re.compile(r"</?ansi[^>]*>")
GAP = re.compile(r"(?<=\S) {2,}(?=\S)")
LIST = re.compile(r"^(- |\* |\d+\. )")
COLONLABEL = re.compile(r"^([A-Z][^:]{0,28}:) (?=\S)")
DASHROW = re.compile(r'^(\s*<ansi fg="(?:command|skill)">[^<]*</ansi>) - (?=\S)')
FIRSTSPAN = re.compile(r"^<ansi[^>]*>([^<]*)</ansi>")
LONEUNIT = re.compile(r'^\s*<ansi fg="(command|skill)">[^<]*</ansi>\s*$')
ART = re.compile(r"[┌┐└┘╔╗╚╝│║╒╕╲╱█░]")
RULE = re.compile(r"[━─]")
LOGIC = re.compile(r"\{\{-?\s*(range|with|if)\b")
MARKERS = ("::rows", "::list", "::keep", "::")


def vis(s):
    return TAG.sub("", s)


def indent(s):
    v = vis(s)
    return len(v) - len(v.lstrip(" "))


def classify(line):
    v = vis(line)
    if not v.strip():
        return "blank"
    ind = indent(line)
    body = v[ind:]
    if ART.search(v):
        return "art"
    if ind == 0 and (v.startswith(".:") or RULE.search(v)):
        return "para"
    gaps = GAP.findall(body.rstrip())
    if len(gaps) >= 2:
        return "table"
    if gaps:
        if ind > 0:
            return "row"
        m = FIRSTSPAN.match(line)
        if m and GAP.search(body).start() <= len(m.group(1).rstrip()):
            return "row"
    if ind == 0:
        return "seealso" if body.lower().startswith("see also:") else "para"
    if LIST.match(body):
        return "item"
    if LONEUNIT.match(line):
        return "example"
    if DASHROW.match(line):
        return "dashrow"
    if COLONLABEL.match(body):
        return "colonrow"
    return "iprose"


def widen_after_colon(line):
    """'Label: text' becomes 'Label:  text' (the gap a row needs). The space goes right after
    the colon's visible character, inside its colour span if it has one."""
    v = vis(line)
    ind = indent(line)
    m = COLONLABEL.match(v[ind:])
    target = ind + len(m.group(1))  # visible index just after the colon
    seen = 0
    i = 0
    while i < len(line):
        t = TAG.match(line, i)
        if t:
            i = t.end()
            continue
        seen += 1
        i += 1
        if seen == target:
            return line[:i] + " " + line[i:]
    return line


def convert(lines, name, flags):
    kinds = [classify(l) for l in lines]
    out, i, n = [], 0, len(lines)

    def flag(j, why):
        flags.append("%s:%d %s" % (name, j + 1, why))

    def entry_block(i, starts, marker, transform, seealso=False):
        base = indent(lines[i])
        block = []
        while i < n:
            if kinds[i] in starts and indent(lines[i]) == base:
                block.append(transform(lines[i], kinds[i]))
                i += 1
                while i < n and kinds[i] != "blank":
                    k, ind = kinds[i], indent(lines[i])
                    if k in starts and ind == base:
                        break
                    if ind > base and k != "item":
                        block.append(lines[i])  # deeper: the entry continues
                    elif marker == "::rows" and ind == base and k == "iprose":
                        block.append("  " + lines[i])  # same-indent continuation, made deeper
                    elif seealso and ind == 0 and k == "para":
                        block.append(" " * (base + 2) + lines[i])  # a See also list's next line
                    else:
                        if marker == "::list" and ind == base and k == "iprose":
                            flag(i, "a same-indent line after a list item")
                        break
                    i += 1
                # a blank line followed by another entry at the same indent stays in the block
                if i + 1 < n and kinds[i] == "blank" and kinds[i + 1] in starts and indent(lines[i + 1]) == base:
                    block.append(lines[i])
                    i += 1
                    continue
                break
            else:
                break
        out.append(marker)
        out.extend(block)
        out.append("::")
        return i

    def row_t(line, kind):
        if kind == "colonrow":
            return widen_after_colon(line)
        if kind == "dashrow":
            return DASHROW.sub(lambda m: m.group(1) + "  ", line, count=1)
        if kind == "seealso":
            return seealso_t(line, kind)
        return line

    def seealso_t(line, kind):
        return re.sub(r"(?i)(see also:)(</ansi>)? ", lambda m: m.group(1) + (m.group(2) or "") + "  ", line, count=1)

    while i < n:
        k = kinds[i]
        if k == "blank":
            out.append(lines[i])
            i += 1
        elif k in ("row", "colonrow", "dashrow"):
            i = entry_block(i, {"row", "colonrow", "dashrow"}, "::rows", row_t)
        elif k == "seealso":
            i = entry_block(i, {"seealso"}, "::rows", seealso_t, seealso=True)
        elif k == "item":
            i = entry_block(i, {"item"}, "::list", lambda l, _: l)
        elif k == "example":
            j = i
            while j < n and kinds[j] == "example" and indent(lines[j]) == indent(lines[i]):
                j += 1
            if all(len(vis(l).rstrip()) <= 30 for l in lines[i:j]):
                out.append("::keep")
                out.extend(lines[i:j])
                out.append("::")
            else:
                flag(i, "example lines wider than 30")
                out.extend(lines[i:j])
            i = j
        elif k in ("table", "art"):
            j = i
            while j < n and kinds[j] != "blank":
                j += 1
            flag(i, "%s: convert by hand" % k)
            out.extend(lines[i:j])
            i = j
        else:  # para, iprose: a paragraph run
            j = i
            ind = indent(lines[i])
            while j < n and kinds[j] in ("para", "iprose"):
                if j > i and indent(lines[j]) != ind:
                    if not vis(lines[j - 1]).rstrip().endswith(":"):
                        flag(j, "the indent changes inside a paragraph")
                    break
                if RULE.search(vis(lines[j])) and j > i:
                    flag(j, "a heading inside a paragraph")
                j += 1
            out.extend(lines[i:j])
            i = j
    return out


def words(text):
    keep = [l for l in text.split("\n") if l not in MARKERS]
    return [w for w in vis("\n".join(keep)).split() if w != "-"]


def main():
    repo, outroot = sys.argv[1], sys.argv[2]
    paths = sorted(glob.glob(os.path.join(repo, "_datafiles/world/dogmud/templates/help/*.template")) +
                   glob.glob(os.path.join(repo, "modules/*/files/datafiles/templates/help/*.template")))
    flags, logic, changed, unchanged, wordfail = [], [], 0, 0, []
    for p in paths:
        rel = os.path.relpath(p, repo)
        raw = open(p, encoding="utf-8", newline="").read()
        text = raw.replace("\r\n", "\n").replace("\r", "")
        name = rel.replace("\\", "/")
        if LOGIC.search(text):
            logic.append(name)
            new = text
        else:
            new = "\n".join(convert(text.split("\n"), name, flags))
            if words(new) != words(text):
                wordfail.append(name)
                new = text
        dst = os.path.join(outroot, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        with open(dst, "w", encoding="utf-8", newline="\n") as f:
            f.write(new)
        if new != raw.replace("\r\n", "\n"):
            changed += 1
        else:
            unchanged += 1
    print("files %d, changed %d, unchanged %d" % (len(paths), changed, unchanged))
    print("LOGIC (convert by hand): %d" % len(logic))
    for x in logic:
        print("  " + x)
    print("WORDS CHANGED (left as they were): %d" % len(wordfail))
    for x in wordfail:
        print("  " + x)
    print("FLAGGED: %d" % len(flags))
    for x in flags:
        print("  " + x)


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Run it into the scratchpad**

```bash
python -I tools/help_markup_convert.py . "<scratchpad>/p4-converted" | tee "<scratchpad>/p4-convert-report.txt"
```

`<scratchpad>` is the session scratchpad directory the system prompt names, written as its full path.
Expected: `files 485`, a `LOGIC` list of exactly `help`, `emote`, `species`, `spell` (fact 10; any other name means the regex missed or a file changed since: stop and check), `WORDS CHANGED: 0` (any entry is a converter bug: fix the converter, not the file), and a `FLAGGED` list.

- [ ] **Step 3: Read a sample before landing**

Open five converted files beside their originals (`git diff --no-index`): `look`, `bank`, `defense`, `about`, `alchemy`. Each must show only marker lines added, `See also:` and colon labels widened, and `cmd - ` turned into `cmd  `. Anything else is a converter bug: fix and rerun Step 2.

- [ ] **Step 4: Land the output**

```bash
cp -r "<scratchpad>/p4-converted/_datafiles" .
cp -r "<scratchpad>/p4-converted/modules" .
git status --short | grep -c template
```

Expected: the count equals the report's `changed` plus any file whose only change is line endings. `charm.template` is among them: its stray carriage returns are gone (fact 20).

- [ ] **Step 5: Run the guard and the width test**

Run: `go test ./internal/guards/ -run TestHelpFilesStateTheirLayout -count=1 2>&1 | grep -c "line "`
Expected: a count close to the FLAGGED list plus the four logic files. Save the full output to `<scratchpad>/p4-guard-1.txt` for Task 5.

- [ ] **Step 6: Commit the converter and the converted files**

```bash
git add tools/help_markup_convert.py _datafiles/world/dogmud/templates/help modules/aicompanion/files/datafiles/templates/help modules/auctions/files/datafiles/templates/help modules/cleanup/files/datafiles/templates/help modules/follow/files/datafiles/templates/help modules/gmcp/files/datafiles/templates/help modules/leaderboards/files/datafiles/templates/help modules/time/files/datafiles/templates/help
git commit -F - <<'EOF'
feat(help): mark every help file's layout (one-time conversion)

tools/help_markup_convert.py adds ::rows, ::list and ::keep markers;
no file's words change (the converter checks). Flagged spots and the
four template-logic files are converted by hand next.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Hand conversions until the source guard is green

**Files:** the help templates in `<scratchpad>/p4-guard-1.txt`, and:
- `_datafiles/world/dogmud/templates/help/help.template` (the index)
- `_datafiles/world/dogmud/templates/help/emote.template`
- `_datafiles/world/dogmud/templates/help/species.template`
- `_datafiles/world/dogmud/templates/help/spell.template`
- `internal/guards/help_markup_guard_test.go` (committed here)

Edit with the Edit tool only. Every change keeps every word, except the species `()` (decision 5) and lone `-` separators (decision 10).

- [ ] **Step 1: The spell page**

Wrap the eight label lines and the `{{ with }}`/`{{ if }}` lines around them in `::rows` / `::`. The template actions sit inside the block and print no line.

- [ ] **Step 2: The species page**

Rewrite the detail section to the form the mock drew (https://claude.ai/artifact/7ZoZFjo34G591jbBjU4duY, Species): the size lines as `::rows` with `Small:`, `Medium:`, `Large:` labels; each species between two rules drawn with `{{ padRight screenwidth "" "─" }}` (check `padRight`'s signature at `templatesfunctions.go:759` and that `screenwidth` is the render width, `forms.go:84`); its fields as one `::rows` block with labels `Species:`, `Description:`, `Size:`, `Unarmed:`, `First Aid:`, `Gear Slots:` (the slots joined by `, ` with `{{ join ... }}`, `templatesfunctions.go:720`), `Stat Gains:`, `Conditions:` (joined by `, `); `Unarmed:` prints the name only. The `splitstring` call goes: the layout pass wraps the description. Keep `help_species_test.go` passing; extend it to assert the Unarmed line has no `()`.

- [ ] **Step 3: The help index and emote**

Replace each four-column command grid with one indented paragraph line per category built by the `range`: each command a coloured unit (`<ansi fg="{{ $cmdInfo.Type }}">{{ if $cmdInfo.Missing }}*{{ end }}{{ $cmdInfo.Command }}</ansi>`) separated by single spaces, no `padRight`, no `printf "\n"` (decision 4). The category heading line stays above it. The emote shortcut grid becomes the same kind of line. The emote example lines (`[HP:10/10 MP:10/10]: emote scratches his head.`, 50 columns) go in `::keep` only if shortened to 30 or less; otherwise make each a paragraph of its own. Show the before and after of both pages in the PR.

- [ ] **Step 4: Each flagged spot**

Work through `<scratchpad>/p4-guard-1.txt` file by file. The fixes the guard asks for:
- A table (three or more aligned columns): one row per table row, label first, the other columns joined in the text in the order they appear (as the mock's alchemy recipes: `Healing Salve  Skill 0: 2x healers-root, 1x bottle`).
- Box drawing (keyring, online, picklock, picklock-example): the box goes; its content becomes rows or paragraphs. Horizontal rules may stay as headings (`━━━ Title ━━━`).
- ASCII art: `::keep` if every line is 30 or less; `map-examples` is in `helpWide`; anything else wider than 30 is reported to the owner, not exempted.
- "The indent changes inside a paragraph": a hanging paragraph becomes a row (its first words are the label) or one indent.

After each file: `go test ./internal/guards/ -run TestHelpFilesStateTheirLayout -count=1 2>&1 | grep "<file name>"` must print nothing.

- [ ] **Step 5: The whole guard**

Run: `go test ./internal/guards/ -run 'TestHelpFilesStateTheirLayout|TestHelpMarkupRulesCanFail' -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

Stage each edited template by name (list them from `git status --short`), plus the guard:

```bash
git add internal/guards/help_markup_guard_test.go <each edited template>
git commit -F - <<'EOF'
feat(help): hand-convert tables, boxes, grids and logic pages; source guard

The source guard refuses any help line whose layout would be a guess.
Species pages drop the empty () after the unarmed attack.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: The render guard

**Files:**
- Create: `internal/guards/help_render_guard_test.go`
- Modify: `internal/guards/template_width_guard_test.go:68` (`wide`), `:83` (remove `"help/": "help"` and the comment's "plan 4 does help/")

- [ ] **Step 1: Write the render guard**

```go
package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"io/fs"
)

// helpRenderSetup renders help as the server does: the live world's
// templates and every module's, the real species and spells.
func helpRenderSetup(t *testing.T) {
	t.Helper()
	templateRenderSetup(t)
	fss := []fs.ReadFileFS{os.DirFS(dogmudDataFilesRoot).(fs.ReadFileFS)}
	mods, _ := filepath.Glob(filepath.Join("modules", "*", "files", "datafiles"))
	for _, m := range mods {
		fss = append(fss, os.DirFS(m).(fs.ReadFileFS))
	}
	templates.SetFSForTest(t, fss...)
	species.LoadForTest(t)
	spells.LoadSpellFiles()
}

// helpTopics is every help file's topic, plus the topics whose page needs
// data its name alone does not give.
func helpTopics(t *testing.T) []string {
	topics := []string{"spell blood-boil", "species all"}
	for name := range helpTemplateFiles(t) {
		topics = append(topics, strings.TrimPrefix(name, "help/"))
	}
	sort.Strings(topics)
	return topics
}

func TestEveryHelpPageFitsEveryWidth(t *testing.T) {
	helpRenderSetup(t)
	for _, topic := range helpTopics(t) {
		for _, width := range templateWidths {
			out, err := usercommands.GetHelpContents(topic, width)
			if err != nil {
				t.Errorf("help %s at %d: %v", topic, width, err)
				continue
			}
			if helpWide["help/"+topic] {
				continue
			}
			if got := templates.WidestLine(out); got > width {
				t.Errorf("help %s at %d has a line %d wide:\n%s", topic, width, got, out)
			}
		}
	}
}

// The layout pass moves line breaks and drops marker lines; it never loses,
// adds or reorders a word (owner ruling 2026-11-12).
func TestHelpLayoutKeepsEveryWord(t *testing.T) {
	helpRenderSetup(t)
	for name := range helpTemplateFiles(t) {
		raw, err := templates.ProcessForWidth(name, nil, false, 30)
		if err != nil {
			continue // a page that needs data; its words are held through GetHelpContents above
		}
		for _, width := range templateWidths {
			laid := templates.LayoutHelp(raw, width)
			if a, b := helpWords(raw), helpWords(laid); strings.Join(a, " ") != strings.Join(b, " ") {
				t.Errorf("%s at %d: words differ after layout", name, width)
			}
		}
	}
}

func helpWords(s string) []string {
	keep := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		switch l {
		case "::rows", "::list", "::keep", "::":
			continue
		}
		keep = append(keep, l)
	}
	return strings.Fields(helpTag.ReplaceAllString(strings.Join(keep, "\n"), ""))
}

// HELP_DUMP=<folder> writes every page at 30 and 80 for the owner's report
// (Task 7). It checks nothing.
func TestDumpHelpPages(t *testing.T) {
	dir := os.Getenv("HELP_DUMP")
	if dir == "" {
		t.Skip("set HELP_DUMP to a folder to write the report pages")
	}
	helpRenderSetup(t)
	for _, topic := range helpTopics(t) {
		for _, width := range []int{30, 80} {
			out, _ := usercommands.GetHelpContents(topic, width)
			name := strings.ReplaceAll(topic, " ", "_") + "." + map[int]string{30: "w30", 80: "w80"}[width] + ".txt"
			if err := os.WriteFile(filepath.Join(dir, name), []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
```

Group the imports as gofmt wants (`io/fs` with the standard library).

- [ ] **Step 2: Retire the help class**

In `template_width_guard_test.go`: delete the `"help/": "help"` line (`:83`), add `"help/map-examples"` to `templateClassNames["wide"]` (`:68`), and change the class comment (`:29-33`) to say plan 4 is done for `help/` and the help pages are held by `help_render_guard_test.go`. Then run `TestEveryTemplateHasOneWidthClass`: every help template now needs a class or a fit entry. Add `"help"` back as a class named `"help, held by help_render_guard_test.go"` if the class map needs every template named; read `templateClasses` (`:159`) to see which applies.

- [ ] **Step 3: Run the guards**

Run: `go test ./internal/guards/ -count=1`
Expected: PASS. A page over its width is fixed in its file (Task 5's rules), never by widening the guard. A word difference is a layout bug: fix `helplayout.go` with a failing unit test first.

- [ ] **Step 4: Run the usercommands width test from Task 2**

Run: `go test ./internal/usercommands/ -count=1`
Expected: PASS, now that `look` carries its markers.

- [ ] **Step 5: Commit**

```bash
git add internal/guards/help_render_guard_test.go internal/guards/template_width_guard_test.go
git commit -F - <<'EOF'
test(guards): every help page fits 30, 42 and 80, and keeps every word

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: The report for the owner

- [ ] **Step 1: Dump every page**

```bash
mkdir -p "<scratchpad>/p4-dump"
HELP_DUMP="<scratchpad>/p4-dump" go test ./internal/guards/ -run TestDumpHelpPages -count=1
ls "<scratchpad>/p4-dump" | wc -l
```

Expected: twice the topic count (about 974).

- [ ] **Step 2: Build the page**

Copy `_archive/notes/reports/help-pages-p4-mock/` (main checkout) to `_archive/notes/reports/help-pages-p4-report/` and change `gen.py`'s input: read each `<topic>.w30.txt` (tagged text) and draw it at 30 with `to_html(parse(...))`, beside its `.w80.txt`, alphabetically, with a contents list by first letter. Put the report's counts at the top: files converted by the tool, by hand, and the pages whose desktop form changed (`.w80` differs from master's render at 80, which you get by running the same dump on master in a detached worktree; remove that worktree afterwards).

- [ ] **Step 3: Publish and stop**

Publish with the Artifact tool (`icon: "book"`). Give the owner the link and the counts, and wait for his read before Task 8. A page he marks wrong is fixed in its file, with the guard run again.

---

### Task 8: Documents

- [ ] **Step 1: `internal/templates/context.md`**: add `helplayout.go` to the file list (`ProcessHelp`, `LayoutHelp`, the markers and their rules, U+00A0 as the joiner and why). Verify each symbol named exists (`Select-String -Path internal\templates\*.go -Pattern '^(func|type|const|var)\s'`).
- [ ] **Step 2: `internal/usercommands/context.md`**: `GetHelpContents(input, width)`; help renders through `ProcessHelp` at `WrapWidth()`; admin help unchanged.
- [ ] **Step 3: `modules/webhelp/context.md`**: the web help renders at `templates.FullWidth`.
- [ ] **Step 4: `internal/guards/context.md`**: the two new guards and what each holds.
- [ ] **Step 5: `.claude/skills/dogmud-player-copy/SKILL.md`**: a short "Help file layout" section with the markup example from this plan and the guard's rules.
- [ ] **Step 6: The spec**: under the "Hard line breaks in authored text" bullet, add one line: `Amended 2026-11-12 (plan 4, owner ruling): help files state their layout with ::rows, ::list and ::keep markers instead of the game guessing paragraph bounds; see the plan.` Do not edit the bullet's text.
- [ ] **Step 7: `docs/PATCH_NOTES.md`**: a dated entry, player-facing, no numbers, no dashes:

```markdown
## 2026-11-12: Help fits your screen

- Every help page now fits your window, down to the narrowest phone.
  Long lines wrap neatly, and a command is never cut in two.
- Usage lists show each command with what it does on the same line.
- Help pages keep their brackets and plus signs for screen readers, so
  optional parts of a command still read as optional.
- Species help no longer shows empty brackets after a creature's attack.
```

- [ ] **Step 8: `docs/README.md`**: a row for this plan, and one for `tools/help_markup_convert.py` if `docs/README.md` lists tools (grep it).
- [ ] **Step 9: Run the context audit**: `python tools/context_md_audit.py` reports no phantom symbols in the four files.
- [ ] **Step 10: Commit** the documents by name.

---

### Task 9: Gates, owner test, PR

- [ ] **Step 1:** Follow the `dogmud-shipping` skill's pre-push gate in its order (build, vet, the full test suite, guards, boot check in a detached worktree).
- [ ] **Step 2:** Boot a test server on a private port (`dogmud-playtesting` skill; kill only its PID) and check `help look`, `help species`, `help spell blood-boil`, `help alchemy` and the bare `help` on telnet at 80 and in the web client at a phone width. Screenshots go in the repo root.
- [ ] **Step 3:** Hand the owner a punchlist of ordinary player steps only (no admin commands): set the browser to a phone width, open the four pages above, compare with the report. Wait for his test before the PR merges.
- [ ] **Step 4:** Open the PR with `gh pr create --repo pruuk/DOGMud`. The body lists the rulings, the decisions above, the converter's counts, the hand-converted files, the report link and the deploy note (none: no migration, no config). No closing keywords for #783.
