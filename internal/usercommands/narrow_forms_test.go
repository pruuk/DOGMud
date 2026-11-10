package usercommands

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// The narrow forms of the screens whose data types are private to this
// package (plan 3b): character/conditions, character/quests and
// character/title, and the companion list built here in Go. Each is rendered
// at 30, 42 and 80 columns with data as wide as it gets in play, and its
// 80-column output is pinned to what master drew. internal/guards holds the
// other character screens the same way (character_screens_test.go).

var updateNarrowForms = flag.Bool("update-narrow-forms", false,
	"rewrite internal/usercommands/testdata/narrow_forms_80/*.golden from the current output")

// narrowWidths are the widths every screen is rendered at: the narrowest
// screen, an upright phone, and a desktop.
var narrowWidths = []int{30, 42, 80}

// conditionsWide is the conditions list at its widest: the longest shipped
// name with a stack count, long descriptions, every duration word.
func conditionsWide() []conditionEntry {
	return []conditionEntry{
		{Name: "Chrysalis Regeneration (99)", Description: "Your body knits itself back together, stronger in the places it broke.", RoundsLeft: 50},
		{Name: "Minor Potion Healing", Description: "A small healing effect is working on you.", RoundsLeft: 20},
		{Name: "Death's Shadow", Description: "A dark shadow clings to you, dulling your senses and weakening your body.", RoundsLeft: 2},
		{Name: "Stoneskin", Description: "Skin like rock.", Permanent: true},
	}
}

// questRow is a quest list row as activeQuestRecord builds it.
func questRow(id int, name string, repeatable, following bool, desc, progress, guide string, done float64) questRecord {
	return questRecord{Id: id, Name: name, Repeatable: repeatable, Following: following,
		Description: desc, Progress: progress, Guide: guide}.withProgress(done)
}

// The quest bar and the percentage beside it come from one number: only
// withProgress writes Completion, BarFull, BarEmpty and done (no other
// code in this package's non-test files sets them), and the bar it draws at
// any width agrees with the percentage.
//
// The package is type-checked from source (imports stand in as empty
// packages, and the errors that causes are ignored), so a literal is known to
// be a questRecord by its type, not its spelling: an elided element of a
// []questRecord, a pointer literal, an alias and a package-level var are all
// caught, as is a positional literal. An assignment or ++ to one of those
// field names is flagged whatever its receiver, since no other type in the
// package has fields so named.
func TestQuestProgressHasOneSource(t *testing.T) {
	for _, finding := range questProgressWriters(t, ".") {
		t.Error(finding)
	}

	for _, done := range []float64{0, 1.0 / 8, 1.0 / 7, 0.25, 1.0 / 3, 0.5, 0.999, 1} {
		r := questRecord{}.withProgress(done)
		require.Equal(t, fmt.Sprintf(`%d%%`, int(math.Floor(done*100))), r.Completion)
		require.Equal(t, r.BarFull, r.BarFullAt(questBarCells), "done %v", done)
		require.Equal(t, r.BarEmpty, r.BarEmptyAt(questBarCells), "done %v", done)
		require.Equal(t, int(math.Floor(20*done)), utf8.RuneCountInString(r.BarFullAt(20)), "done %v", done)
	}
	require.Empty(t, questRecord{Name: "set aside"}.BarFullAt(20), "a row with no bar has none at any width")
}

// emptyImporter answers every import with an empty, complete package, so
// the package under test type-checks from its own source alone.
type emptyImporter struct{}

func (emptyImporter) Import(path string) (*types.Package, error) {
	pkg := types.NewPackage(path, filepath.Base(path))
	pkg.MarkComplete()
	return pkg, nil
}

// questProgressWriters lists every place in dir's non-test Go files, outside
// questRecord.withProgress, that sets a questRecord's progress fields.
func questProgressWriters(t *testing.T, dir string) []string {
	t.Helper()
	fields := map[string]bool{"Completion": true, "BarFull": true, "BarEmpty": true, "done": true}
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	fset := token.NewFileSet()
	files := []*ast.File{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		files = append(files, f)
	}
	require.Greater(t, len(files), 100, "the scan must reach the package's files")

	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: emptyImporter{}, Error: func(error) {}}
	pkg, _ := conf.Check("usercommands", fset, files, info)
	obj := pkg.Scope().Lookup("questRecord")
	require.NotNil(t, obj, "questRecord must type-check")
	isRecord := func(e ast.Expr) bool {
		tv, ok := info.Types[e]
		if !ok || tv.Type == nil {
			return false
		}
		typ := tv.Type
		if p, ok := typ.(*types.Pointer); ok {
			typ = p.Elem()
		}
		return types.Identical(typ, obj.Type())
	}

	var found []string
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "withProgress" && fn.Recv != nil {
				return false
			}
			flag := func(pos token.Pos, what string) {
				found = append(found, fmt.Sprintf("%s: %s; use withProgress", fset.Position(pos), what))
			}
			switch x := n.(type) {
			case *ast.CompositeLit:
				if !isRecord(x) {
					return true
				}
				for _, el := range x.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						flag(el.Pos(), "a positional questRecord literal sets every field")
						break
					}
					if key, ok := kv.Key.(*ast.Ident); ok && fields[key.Name] {
						flag(kv.Pos(), "a questRecord literal sets "+key.Name)
					}
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && fields[sel.Sel.Name] {
						flag(sel.Pos(), "an assignment sets ."+sel.Sel.Name)
					}
				}
			case *ast.IncDecStmt:
				if sel, ok := x.X.(*ast.SelectorExpr); ok && fields[sel.Sel.Name] {
					flag(sel.Pos(), "an increment sets ."+sel.Sel.Name)
				}
			}
			return true
		})
	}
	return found
}

// questsWide is a quest list at its widest: the longest shipped quest name,
// repeatable and followed, a long step description, progress and guide
// lines, a finished repeatable, a quest set aside, and three-digit counts.
func questsWide() questsScreen {
	rows := []questRecord{
		questRow(1, "The City Watch's Missing Person", true, true,
			"Orrin has set you against the Unfolded. Climb out of the grove through the Veil Tear, cross the Folded Distance and the Unraveling, and defeat the Unfolded in the Thinnest Place.",
			"Unfolded defeated in the Thinnest Place: 2 of 5", "Usually found around here: Drillmaster Vorn the Unyielding.", 1.0/3),
		questRow(2, "First Blood", false, false,
			"Crier Toke hands you a cudgel when you take this on and sends you to Drillmaster Vorn in the Drill Yard.",
			"", "Take the portal.", 1.0/8),
	}
	rows = append(rows, questRecord{Id: 3, Name: "The Peddler's Overdue Freight", Repeatable: true,
		Description: repeatReadyLine}.withProgress(1))
	rows = append(rows, questRecord{Id: 4, Name: "The Innkeeper's Complaint",
		Description: "Set aside. Talk to Innkeeper Bramblewick of the Sleeping Stag to take it up again."})
	return questsScreen{QuestsTotal: 112, QuestsFound: len(rows), Records: rows}
}

// questsAsCaptured is the quest list as the 2026-11-10 capture showed it,
// which fits 80 columns, for the 80-column golden.
func questsAsCaptured() questsScreen {
	return questsScreen{QuestsTotal: 7, QuestsFound: 2, Records: []questRecord{
		questRow(1, "Find Your Footing", false, true,
			"Crier Toke wants you to learn the town before the town has to learn you. Start at the Chrysalis School Hall. If you lose your way, the map command shows where you have been.",
			"", "Take the portal.", 1.0/7),
		questRow(2, "The Unfolded", false, false,
			"Orrin has set you against the Unfolded. Climb out of the grove through the Veil Tear, cross the Folded Distance and the Unraveling, and defeat the Unfolded in the Thinnest Place.",
			"Unfolded defeated: 0 of 1", "Take the portal.", 1.0/3),
	}}
}

// titleWide is the title screen at its widest words.
func titleWide() titleScreen {
	bar := func(done float64) string {
		full, empty := util.ProgressBar(done, 20)
		return `<ansi fg="green">` + full + `</ansi><ansi fg="black-bold">` + empty + `</ansi>`
	}
	return titleScreen{FullTitle: "Ascendant Grandmaster Juggernaut", MutTier: "Ascendant",
		MutLoad: "overwhelming", SkillTier: "Grandmaster", SkillPct: "accomplished", Archetype: "Juggernaut",
		TopStats: []statBar{{"Perception", 300, bar(1)}, {"Strength", 290, bar(0.96)}, {"Vitality", 120, bar(0.4)}}}
}

// narrowTemplateScreens are the template screens held here: data for the
// width checks, and data whose base fits 80 for the golden.
var narrowTemplateScreens = map[string]struct{ wide, golden func() any }{
	"character/conditions": {func() any { return conditionsWide() }, func() any { return conditionsWide() }},
	"character/quests":     {func() any { return questsWide() }, func() any { return questsAsCaptured() }},
	"character/title":      {func() any { return titleWide() }, func() any { return titleWide() }},
}

func narrowGoldenPath(name string) string {
	return filepath.Join("testdata", "narrow_forms_80", strings.ReplaceAll(name, "/", "_")+".golden")
}

// checkGolden compares got, line ends aside, with the named golden, or
// records it with -update-narrow-forms.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, "\r\n", "\n")
	path := narrowGoldenPath(name)
	if *updateNarrowForms {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "record the golden from master with -update-narrow-forms")
	require.LessOrEqual(t, templates.WidestLine(string(want)), 80, "the golden no longer fits 80, so it pins a form")
	require.Equal(t, string(want), got, "%s changed at 80 columns", name)
}

func TestNarrowTemplateScreensFitEveryWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	for name, screen := range narrowTemplateScreens {
		for _, width := range narrowWidths {
			out, err := templates.ProcessAtWidth(name, screen.wide(), width)
			require.NoError(t, err, "%s at %d", name, width)
			require.NotEmpty(t, strings.TrimSpace(out), "%s at %d rendered nothing", name, width)
			require.LessOrEqual(t, templates.WidestLine(out), width, "%s at %d columns:\n%s", name, width, out)
		}
	}
}

func TestNarrowTemplateScreensKeepTheir80ColumnOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	for name, screen := range narrowTemplateScreens {
		out, err := templates.ProcessAtWidth(name, screen.golden(), 80)
		require.NoError(t, err)
		checkGolden(t, name, out)
	}
}

// companionScreen runs `companion` for user 1 with a summoned wolf whose
// name and mutation list run long and a bonded hound that is away, at a
// reader width, and returns what was sent.
func companionScreen(t *testing.T, width int) string {
	t.Helper()
	return companionCommand(t, width, "", longCompanionMutations)
}

// longCompanionMutations are the two longest shipped mutation names, at
// levels that add "(Ln)", and two short ones; capturedCompanionMutations are
// the two the 2026-11-10 capture showed.
var (
	longCompanionMutations     = map[string]int{"rending-claws": 1, "tail": 1, "sustaining-presence": 4, "commanding-presence": 3}
	capturedCompanionMutations = map[string]int{"rending-claws": 1, "tail": 1}
)

// companionCommand runs `companion <rest>` as companionScreen does, the wolf
// holding muts.
func companionCommand(t *testing.T, width int, rest string, muts map[string]int) string {
	t.Helper()
	user, room := getTestUserAndRoom(t)
	prevWidth, prevComps := user.LineWidth, user.Character.Companions
	t.Cleanup(func() { user.LineWidth, user.Character.Companions = prevWidth, prevComps })
	user.LineWidth = width

	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"rending-claws":       {MutationId: "rending-claws", Name: "Rending Claws"},
		"tail":                {MutationId: "tail", Name: "Tail"},
		"sustaining-presence": {MutationId: "sustaining-presence", Name: "Sustaining Presence"},
		"commanding-presence": {MutationId: "commanding-presence", Name: "Commanding Presence"},
	}))
	wolf := &mobs.Mob{MobId: 1, InstanceId: 301, Character: characters.Character{
		Name: "Steppe Spirit Wolf", Health: 40, Stamina: 30, Conviction: 10,
		Conditions: conditions.New(), Cooldowns: map[string]int{},
		Mutations: muts,
	}}
	wolf.Character.HealthMax.Value = 40
	wolf.Character.StaminaMax.Value = 60
	wolf.Character.ConvictionMax.Value = 50
	mobs.SetInstanceForTest(301, wolf)
	t.Cleanup(func() { mobs.SetInstanceForTest(301, nil) })

	user.Character.Companions = []characters.CompanionInfo{
		{MobId: 1, InstanceId: 301, SourceType: characters.CompanionSummoned, Name: "Steppe Spirit Wolf", AutoAssist: true},
		{MobId: 1, InstanceId: 999, SourceType: characters.CompanionBonded, Name: "Fenwickerly the Ashen Hound", AutoAssist: false},
	}

	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Companion(rest, user, room, 0)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
}

// The detail view of one companion, present and away, fits too. Its
// mutations line is never wrapped (CategoryScreen), so the long list runs
// past 80 columns there and gets a form even at 80, as a template wider than
// its reader does. With the captured list it fits 80 and is pinned.
func TestCompanionDetailFitsEveryWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	for _, name := range []string{"steppe", "fenwickerly"} {
		for _, width := range narrowWidths {
			out := companionCommand(t, width, name, longCompanionMutations)
			require.Contains(t, out, "Auto-Assist", "%s at %d", name, width)
			require.LessOrEqual(t, templates.WidestLine(out), width, "companion %s at %d columns:\n%s", name, width, out)
			if width < 80 {
				requireFullWidthRules(t, out, width)
			}
		}
	}
	// The long mutations line does not fit 80, so the wolf's detail view
	// is the 42 form there too.
	requireFullWidthRules(t, companionCommand(t, 80, "steppe", longCompanionMutations), 42)
}

// A mutation entry (its name, its level and comma) is never split while it
// fits on a line by itself, in the list or the detail view, with the two
// longest shipped names at a level.
func TestCompanionMutationEntriesStayWhole(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	entries := []string{"Commanding Presence (L3),", "Rending Claws,", "Sustaining Presence (L4),", "Tail"}
	for _, width := range []int{30, 42} {
		for _, rest := range []string{"", "steppe"} {
			out := companionCommand(t, width, rest, longCompanionMutations)
			for _, entry := range entries {
				if rest == "steppe" {
					entry = strings.TrimSuffix(entry, ",")
				}
				require.Contains(t, out, entry, "companion %q at %d splits %q:\n%s", rest, width, entry, out)
			}
		}
		// The first line is measured from its own indent: the 27-letter
		// name fits after 2 spaces at 30.
		require.Contains(t, companionScreen(t, width), "Fenwickerly the Ashen Hound", "at %d", width)
	}
}

// requireFullWidthRules holds a companion form to the ruling that every
// rule spans the form's width: each line with a "━" in it is exactly
// formWidth wide, and there is at least one.
func requireFullWidthRules(t *testing.T, out string, formWidth int) {
	t.Helper()
	rules := 0
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if !strings.Contains(line, "━") {
			continue
		}
		rules++
		require.Equal(t, formWidth, templates.WidestLine(line), "rule line %q in:\n%s", line, out)
	}
	require.Positive(t, rules, "no rule line in:\n%s", out)
}

func TestCompanionDetailKeepsIts80ColumnOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	checkGolden(t, "companion_detail", companionCommand(t, 80, "steppe", capturedCompanionMutations)+
		companionCommand(t, 80, "fenwickerly", capturedCompanionMutations))
}

func TestCompanionListFitsEveryWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	for _, width := range narrowWidths {
		out := companionScreen(t, width)
		require.Contains(t, out, "Steppe Spirit Wolf", "at %d", width)
		require.LessOrEqual(t, templates.WidestLine(out), width, "companion at %d columns:\n%s", width, out)
		if width < 80 {
			requireFullWidthRules(t, out, width)
		}
	}
}

func TestCompanionListKeepsIts80ColumnOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	checkGolden(t, "companion", companionScreen(t, 80))
}
