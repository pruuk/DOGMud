package guards

import (
	"flag"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// motdAnsiTagRE mirrors usercommands/motd.go's own regexp of the same
// name: strips <ansi …> / </ansi> tags for visible-width math when padding
// MOTD lines. Duplicated rather than imported because internal/usercommands
// is not importable here without pulling in its full command-table init;
// this is a two-line mirror, not an independent design.
var motdAnsiTagRE = regexp.MustCompile(`<ansi[^>]*>|</ansi>`)

// motdVisibleWidthForTest mirrors usercommands/motd.go's motdVisibleWidth.
func motdVisibleWidthForTest(s string) int {
	return len(motdAnsiTagRE.ReplaceAllString(s, ""))
}

// updateLiveRender re-records testdata/wrap_live_render.golden.
var updateLiveRender = flag.Bool("update-live-render", false, "rewrite testdata/wrap_live_render.golden")

// wrapLiveRenderGoldenName is the golden in this package's testdata folder
// (internal/guards/testdata), resolved through testdataPath.
const wrapLiveRenderGoldenName = "wrap_live_render.golden"

// dogmudDataFilesRoot is the LIVE world's data directory, relative to the
// repo root. NOT world/default: that tree is vestigial, does not boot, and
// its templates are not what any player is served. See
// internal/templates/process_test.go's dataFilesRoot comment for the same
// lesson learned the hard way on 2026-09-22.
const dogmudDataFilesRoot = "_datafiles/world/dogmud"

// TestLiveRenderedOutputSurvivesThePipeline pins the wrap allowlist
// (internal/messaging/pipeline.go's shouldWrap) against REAL production
// output, not a stand-in.
//
// WHY THIS TEST EXISTS SEPARATELY FROM THE IN-PACKAGE TEST.
// internal/messaging/wrap_preformatted_golden_test.go already pins
// shouldWrap's consequence against five hand-built SHAPES (a column table,
// a box banner, a side-by-side minimap, ASCII art, a progress bar). It has
// to use shapes: internal/messaging sits below internal/templates and
// internal/usercommands in the import graph, so it cannot call
// templates.Process or build the real MOTD box. That is exactly the "a
// golden covers the store, not the path production calls" trap that bit
// this project's M4a stage three separate times — a stand-in can be correct
// in isolation and still miss a real payload shape nobody ran through it.
// This test closes that gap: package guards sits above the whole of
// internal/ and imports freely across it, so it can render the actual character/status template,
// the actual templates.DynamicList packer behind the admin item and mob
// listings, and a box that mirrors usercommands/motd.go line for line.
//
// WHY THE GOLDEN RECORDS PIPELINE OUTPUT, NOT TEMPLATE OUTPUT.
// Byte-identity against the raw template render is the wrong assertion and
// fails immediately: RenderForRecipient (pipeline.go) always alters
// CategorySystem and CategoryBroadcast text even when the wrap stage never
// fires. Stage 5 (applyCategoryColor) wraps every non-default category in
// <ansi fg="...">...</ansi>, and stage 2 (normalize) runs on both — Category
// System and CategoryBroadcast are NOT in normalize.go's skipStages list —
// so appendEndPunct adds a trailing period to the last line. Recording the
// PIPELINE's output, run through RenderForRecipient exactly as production
// callers do, is what actually reflects what a player receives.
//
// WHY A NEWLINE COUNT ALONE IS NOT SUFFICIENT.
// WrapAnsi rejoins words with a single space regardless of how many spaces
// separated them in the source. A real DynamicList table once went from 494
// bytes to 436 at width 55 with its NEWLINE COUNT UNCHANGED — the wrap
// stage had silently eaten the column padding that lined up the rows,
// without folding a single line. A line-count-only assertion would have
// passed while the table was ruined. Only a full checked-in golden of the
// pipeline's byte output catches that, plus any future stage that starts
// touching this text.
func TestLiveRenderedOutputSurvivesThePipeline(t *testing.T) {
	wrapLiveRenderGoldenPath := testdataPath(wrapLiveRenderGoldenName)
	prevDataFiles := configs.GetFilePathsConfig().DataFiles.String()
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dogmudDataFilesRoot}); err != nil {
		t.Fatalf("AddOverlayOverrides: %v", err)
	}
	defer func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prevDataFiles})
	}()

	// readFile() (internal/templates/templates.go) returns (nil, nil) on an
	// empty fileSystems slice, which makes every lookup fall through to the
	// .md variant with empty content. Registering the live world's dir as a
	// ReadFileFS is what makes Process() find the real .template files.
	// There is no unregister: templates.fileSystems is package-private, so
	// this registration outlives the test for the rest of this binary's
	// run, same as internal/templates/process_test.go's TestMain does for
	// its own package.
	templates.RegisterFS(os.DirFS(dogmudDataFilesRoot).(fs.ReadFileFS))

	const lineWidth = 55

	var sb strings.Builder

	// ── Fixture 1: character/status, the real per-player status sheet ──────
	// templates.Process("character/status", u, u.UserId) is the exact call
	// internal/usercommands/status.go makes; CategorySystem is the exact
	// category it sends under.
	u := users.NewUserRecord(1, 0)
	u.Character.Name = "Fixturewick"

	// characters.New() (invoked by NewUserRecord via characters.New()) rolls
	// RANDOM stats via RollCharacterStats(), and status.template's title line
	// derives its class word from whichever stat rolls highest
	// (skills.GetStatArchetype). Left alone, that makes this fixture flip
	// between "Novice Paladin", "Novice Generalist" and others on every run,
	// which a byte-exact golden cannot tolerate. Pin every stat's Base to a
	// distinct, clearly-separated value and recalculate, per this project's
	// rule that a stat write must go through .Base + RecalculateStats(),
	// never straight at .Value.
	u.Character.Stats.Strength.Base = 90
	u.Character.Stats.Dexterity.Base = 70
	u.Character.Stats.Perception.Base = 60
	u.Character.Stats.Vitality.Base = 50
	u.Character.Stats.Willpower.Base = 40
	u.Character.Stats.Charisma.Base = 30
	u.Character.RecalculateStats()

	statusRendered, err := templates.Process("character/status", u, u.UserId)
	if err != nil {
		t.Fatalf("templates.Process(character/status): %v", err)
	}
	// Sanity check: a single-line render would make this whole golden
	// pin nothing meaningful. Fail loudly instead of silently.
	if !strings.Contains(statusRendered, "\n") {
		t.Fatalf("character/status render has no newline, fixture is degenerate:\n%s", statusRendered)
	}

	sb.WriteString("=== status (CategorySystem) ===\n")
	sb.WriteString(messaging.RenderForRecipient(messaging.RenderInput{
		Category:  messaging.CategorySystem,
		Text:      statusRendered,
		Channel:   messaging.ChannelAudio,
		LineWidth: lineWidth,
	}))
	sb.WriteString("\n")

	// ── Fixture 2: templates.DynamicList, the real column packer behind ────
	// the admin item and mob listings (internal/usercommands/admin.item.go,
	// admin.mob.go). Six entries, mirroring how admin.item.go derives its
	// own numWidth/colWidth from the entry set.
	names := []string{
		"Iron Sword",
		"Healing Draught",
		"Leather Boots",
		"Silver Ring",
		"Oak Shield",
		"Bronze Dagger",
	}
	itmNames := make([]templates.NameDescription, 0, len(names))
	longestName := 0
	for i, name := range names {
		itmNames = append(itmNames, templates.NameDescription{Id: i + 1, Name: name})
		if len(name) > longestName {
			longestName = len(name)
		}
	}
	numWidth := len(strconv.Itoa(len(itmNames)))
	colWidth := 1 + numWidth + 2 + longestName + 1
	// sw (screen width) is normally user.ClientSettings().Display.GetScreenWidth();
	// this fixture has no live connection, so it is pinned to lineWidth to
	// match the rest of this test's narrow-width intent.
	listRendered := templates.DynamicList(itmNames, colWidth, lineWidth, numWidth, longestName)

	sb.WriteString("=== item list (CategorySystem) ===\n")
	sb.WriteString(messaging.RenderForRecipient(messaging.RenderInput{
		Category:  messaging.CategorySystem,
		Text:      listRendered,
		Channel:   messaging.ChannelAudio,
		LineWidth: lineWidth,
	}))
	sb.WriteString("\n")

	// ── Fixture 3: a MOTD box, mirroring internal/usercommands/motd.go's ───
	// real construction (frame arithmetic, border glyphs, per-line padding).
	// Calling Motd() directly is impractical here: it needs a live
	// connection and an active user session (it reads
	// configs.GetServerConfig().Motd and sends through user.SendText on a
	// *rooms.Room), neither of which this test constructs. So this fixture
	// hand-builds the same box motd.go builds, reading straight off
	// motd.go's own comments and arithmetic rather than inventing a new
	// shape:
	//
	//   lw := user.GetLineWidth()          -> lineWidth (55) here
	//   innerWidth := lw - 3                -> chars between the corners
	//   padWidth := lw - 4                  -> ` ║ ` (3) + `║` (1) = 4 frame chars
	//   wrapWidth := padWidth - 1           -> lw - 5, leaves a gutter space
	motdText := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
		"newbie zone hazards, and ask a mod if anything looks broken."
	motdWrapped := strings.Split(messaging.WrapAnsi(motdText, lineWidth-5), "\n")

	innerWidth := lineWidth - 3
	padWidth := lineWidth - 4
	horiz := strings.Repeat("═", innerWidth)
	title := `.:  M E S S A G E   O F   T H E   D A Y`
	titleInner := " " + title
	if len(titleInner) > padWidth {
		titleInner = titleInner[:padWidth]
	}
	titlePadded := titleInner + strings.Repeat(" ", padWidth-len(titleInner))
	titleStyled := strings.Replace(titlePadded, title, `<ansi fg="cyan-bold">`+title+`</ansi>`, 1)

	var motdOutput string
	motdOutput += `<ansi fg="yellow"> ╔` + horiz + `╗` + "\n"
	motdOutput += ` ║ ` + titleStyled + `║` + "\n"
	motdOutput += ` ╠` + horiz + `╣` + "\n"
	for _, line := range motdWrapped {
		padLen := padWidth - motdVisibleWidthForTest(line)
		if padLen < 0 {
			padLen = 0
		}
		motdOutput += ` ║ ` + line + strings.Repeat(" ", padLen) + `║` + "\n"
	}
	motdOutput += ` ╚` + horiz + `╝</ansi>` + "\n"

	sb.WriteString("=== motd (CategoryBroadcast) ===\n")
	sb.WriteString(messaging.RenderForRecipient(messaging.RenderInput{
		Category:  messaging.CategoryBroadcast,
		Text:      motdOutput,
		Channel:   messaging.ChannelAudio,
		LineWidth: lineWidth,
	}))
	sb.WriteString("\n")

	// Normalise CRLF before writing or comparing. The live world's .template
	// files are ordinary checked-out text and this repo runs with
	// core.autocrlf=true, so on a Windows checkout a template's literal
	// blank/newline bytes arrive as \r\n rather than \n. That is payload
	// content here (templates.Process reads the file raw, with no line-
	// ending translation), not just checkout metadata, so it would
	// otherwise make this golden differ by host platform. Normalising both
	// the freshly rendered text and (defensively, on read below) the
	// checked-in golden keeps the comparison platform-independent, the same
	// belt-and-braces reasoning m2_routing_guard_test.go documents for its
	// own golden.
	got := strings.ReplaceAll(sb.String(), "\r\n", "\n")

	if *updateLiveRender {
		if err := os.WriteFile(wrapLiveRenderGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Skipf("recorded %d bytes to %s", len(got), wrapLiveRenderGoldenPath)
	}

	raw, err := os.ReadFile(wrapLiveRenderGoldenPath)
	if err != nil {
		t.Fatalf("read %s: %v (record it with -update-live-render)", wrapLiveRenderGoldenPath, err)
	}
	// Normalise CRLF before comparing. This repo has core.autocrlf=true; a
	// Windows checkout rewrites line endings, and .gitattributes pins
	// *.golden to LF on checkout, but that only applies on checkout — a
	// golden reaching this test any other way could still arrive CRLF.
	want := strings.ReplaceAll(string(raw), "\r\n", "\n")

	if got != want {
		pos, ctxWant, ctxGot := firstDiffContext(want, got)
		t.Fatalf(
			"live render pipeline output does not match %s (first difference at byte %d)\n\n"+
				"want context:\n%s\n\ngot context:\n%s\n\n"+
				"If CategorySystem or CategoryBroadcast was added to shouldWrap's "+
				"allowlist (internal/messaging/pipeline.go), REVERT that: those two "+
				"categories are deliberately excluded because they carry pre-formatted "+
				"tables, box banners and ASCII art that wrapping mangles.\n\n"+
				"Otherwise, if this change is deliberate, re-record with:\n"+
				"  go test ./internal/guards/ -run TestLiveRenderedOutputSurvivesThePipeline -update-live-render -v",
			wrapLiveRenderGoldenPath, pos, ctxWant, ctxGot,
		)
	}
}

// firstDiffContext returns the byte offset of the first differing position
// between want and got, plus a small surrounding window of each. Dumping two
// full ~3.5KB rendered sheets into a test failure log is unreadable; this
// narrows a failure to the one line that actually moved.
func firstDiffContext(want, got string) (pos int, wantCtx, gotCtx string) {
	n := len(want)
	if len(got) < n {
		n = len(got)
	}
	for pos = 0; pos < n; pos++ {
		if want[pos] != got[pos] {
			break
		}
	}
	const window = 60
	ctx := func(s string, at int) string {
		start := at - window
		if start < 0 {
			start = 0
		}
		end := at + window
		if end > len(s) {
			end = len(s)
		}
		return s[start:end]
	}
	return pos, ctx(want, pos), ctx(got, pos)
}
