package usercommands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The craft list and the bare `craft` view (plan 3b-3): the rows as drawn at
// 80, 42 and 30 (craft_list in narrow-screens-3b-mock, craft_ready in
// narrow-screens-3b3-mock; both 80 forms are the new 80-column layout, ruling
// 5), every shipped recipe fitting at every width with its longest reason, and
// both commands fitting when run.

// capturedRow is a recipe row as the captures showed it.
func capturedRow(indicator, name, reason, station, speed string, ingredients ...string) craftRow {
	return craftRow{indicator: indicator, name: name, ingredients: ingredients, reason: reason, station: station, speed: speed}
}

// craftListCaptured is the craft list the 2026-11-10 capture showed: every
// recipe locked, by skill.
var craftListCaptured = []struct {
	heading string
	rows    []craftRow
}{
	{"Alchemy (apprentice)", []craftRow{
		capturedRow("X", "Healing Salve", "need alchemy bench", "alchemy bench", "quick", "2x Healer's Root", "1x bottle"),
		capturedRow("X", "Stamina Tonic", "need alchemy bench", "alchemy bench", "quick", "2x Bitter Thistle", "1x bottle"),
	}},
	{"Blacksmithing (novice)", []craftRow{
		capturedRow("X", "Basalt-Iron Blade", "apprentice skill required", "forge", "moderate", "2x Basalt-Iron Ore", "1x Iron Ingot"),
		capturedRow("X", "Iron Dagger", "need forge", "forge", "quick", "1x Iron Ingot", "1x Leather Strip"),
	}},
	{"Cooking (apprentice)", []craftRow{
		capturedRow("X", "Grilled Meat", "need cooking fire", "cooking fire", "quick", "1x Raw Meat", "1x Salt Pouch"),
		capturedRow("X", "Trail Rations", "need cooking fire", "cooking fire", "quick", "1x Raw Meat", "1x Salt Pouch"),
	}},
	{"Enchanting (apprentice)", []craftRow{
		capturedRow("X", "Honed-Edge (weapon)", "need enchanting circle", "enchanting circle", "quick", "1x Binding Paste", "1x Healer's Root"),
	}},
	{"Jewelcrafting (apprentice)", []craftRow{
		capturedRow("X", "Copper Ring", "need jeweler bench", "jeweler bench", "quick", "1x Copper Wire", "1x Gem Dust"),
	}},
	{"Tailoring (apprentice)", []craftRow{
		capturedRow("X", "Cloth Pants", "need loom", "loom", "quick", "2x Cloth Strip", "1x Thread Spool"),
		capturedRow("X", "Linen Tunic", "need loom", "loom", "quick", "2x Cloth Strip", "1x Thread Spool"),
	}},
}

// craftReadyCaptured is the bare `craft` view the 2026-11-11 capture showed.
var craftReadyCaptured = []craftRow{
	capturedRow("V", "Elixir of Renewal", "", "alchemy bench", "lengthy", "2x Healer's Root", "1x Bitter Thistle", "1x Dustwalk Herb", "1x Chrysalis Core", "1x bottle"),
	capturedRow("V", "Healing Salve", "", "alchemy bench", "quick", "2x Healer's Root", "1x bottle"),
	capturedRow("V", "Phial of Second Birth", "", "alchemy bench", "prolonged", "1x Reduction Base", "1x bottle", "1x Unmaking Distillate", "1x First-Bloom Nectar", "6x Blood-moss", "4x Salt Pouch"),
	capturedRow("V", "Stamina Tonic", "", "alchemy bench", "quick", "2x Bitter Thistle", "1x bottle"),
	capturedRow("V", "Vitalis Bandolier", "", "alchemy bench", "prolonged", "1x Reinforced Harness", "1x Preservation Runes", "1x Chrysalis Filter-Membrane", "1x Still-Glass Rosette", "6x Pine Pitch", "6x Salt Pouch", "4x Beeswax"),
}

// craftListRowsText is the captured list's skill headings and rows at
// formWidth, colour tags removed.
func craftListRowsText(formWidth int) string {
	indent := 4
	if formWidth < 42 {
		indent = 3
	}
	lines := []string{}
	for _, group := range craftListCaptured {
		lines = append(lines, "  "+group.heading)
		for _, row := range group.rows {
			lines = append(lines, craftRowLines(row, indent, formWidth)...)
		}
	}
	return strings.Join(plainLines(strings.Join(lines, "\n")), "\n")
}

func craftReadyRowsText(formWidth int) string {
	lines := []string{}
	for _, row := range craftReadyCaptured {
		lines = append(lines, craftRowLines(row, 2, formWidth)...)
	}
	return strings.Join(plainLines(strings.Join(lines, "\n")), "\n")
}

// The rows as drawn. At 80 and 42 they are the drawings; at 30 the lines
// after a row's first sit 2 in from the row (ruling 9, where the craft_list
// drawing had them under the name), and a unit too wide for that indent
// splits at a space and keeps it (ruling 3, where the craft_ready drawing
// pulled `1x Chrysalis Filter-Membrane,` left to column 1).
func TestCraftRowsAreTheDrawings(t *testing.T) {
	for _, tc := range []struct {
		what, got, want string
	}{
		{"craft list at 80", craftListRowsText(80), `  Alchemy (apprentice)
    [X] Healing Salve             : 2x Healer's Root, 1x bottle
        need alchemy bench [alchemy bench], quick
    [X] Stamina Tonic             : 2x Bitter Thistle, 1x bottle
        need alchemy bench [alchemy bench], quick
  Blacksmithing (novice)
    [X] Basalt-Iron Blade         : 2x Basalt-Iron Ore, 1x Iron Ingot
        apprentice skill required [forge], moderate
    [X] Iron Dagger               : 1x Iron Ingot, 1x Leather Strip
        need forge [forge], quick
  Cooking (apprentice)
    [X] Grilled Meat              : 1x Raw Meat, 1x Salt Pouch
        need cooking fire [cooking fire], quick
    [X] Trail Rations             : 1x Raw Meat, 1x Salt Pouch
        need cooking fire [cooking fire], quick
  Enchanting (apprentice)
    [X] Honed-Edge (weapon)       : 1x Binding Paste, 1x Healer's Root
        need enchanting circle [enchanting circle], quick
  Jewelcrafting (apprentice)
    [X] Copper Ring               : 1x Copper Wire, 1x Gem Dust
        need jeweler bench [jeweler bench], quick
  Tailoring (apprentice)
    [X] Cloth Pants               : 2x Cloth Strip, 1x Thread Spool
        need loom [loom], quick
    [X] Linen Tunic               : 2x Cloth Strip, 1x Thread Spool
        need loom [loom], quick`},
		{"craft list at 42", craftListRowsText(42), `  Alchemy (apprentice)
    [X] Healing Salve:
        2x Healer's Root, 1x bottle
        need alchemy bench
        [alchemy bench], quick
    [X] Stamina Tonic:
        2x Bitter Thistle, 1x bottle
        need alchemy bench
        [alchemy bench], quick
  Blacksmithing (novice)
    [X] Basalt-Iron Blade:
        2x Basalt-Iron Ore, 1x Iron Ingot
        apprentice skill required
        [forge], moderate
    [X] Iron Dagger:
        1x Iron Ingot, 1x Leather Strip
        need forge [forge], quick
  Cooking (apprentice)
    [X] Grilled Meat:
        1x Raw Meat, 1x Salt Pouch
        need cooking fire
        [cooking fire], quick
    [X] Trail Rations:
        1x Raw Meat, 1x Salt Pouch
        need cooking fire
        [cooking fire], quick
  Enchanting (apprentice)
    [X] Honed-Edge (weapon):
        1x Binding Paste, 1x Healer's Root
        need enchanting circle
        [enchanting circle], quick
  Jewelcrafting (apprentice)
    [X] Copper Ring:
        1x Copper Wire, 1x Gem Dust
        need jeweler bench
        [jeweler bench], quick
  Tailoring (apprentice)
    [X] Cloth Pants:
        2x Cloth Strip, 1x Thread Spool
        need loom [loom], quick
    [X] Linen Tunic:
        2x Cloth Strip, 1x Thread Spool
        need loom [loom], quick`},
		{"craft list at 30", craftListRowsText(30), `  Alchemy (apprentice)
   [X] Healing Salve:
     2x Healer's Root,
     1x bottle
     need alchemy bench
     [alchemy bench], quick
   [X] Stamina Tonic:
     2x Bitter Thistle,
     1x bottle
     need alchemy bench
     [alchemy bench], quick
  Blacksmithing (novice)
   [X] Basalt-Iron Blade:
     2x Basalt-Iron Ore,
     1x Iron Ingot
     apprentice skill required
     [forge], moderate
   [X] Iron Dagger:
     1x Iron Ingot,
     1x Leather Strip
     need forge [forge], quick
  Cooking (apprentice)
   [X] Grilled Meat:
     1x Raw Meat,
     1x Salt Pouch
     need cooking fire
     [cooking fire], quick
   [X] Trail Rations:
     1x Raw Meat,
     1x Salt Pouch
     need cooking fire
     [cooking fire], quick
  Enchanting (apprentice)
   [X] Honed-Edge (weapon):
     1x Binding Paste,
     1x Healer's Root
     need enchanting circle
     [enchanting circle],
     quick
  Jewelcrafting (apprentice)
   [X] Copper Ring:
     1x Copper Wire,
     1x Gem Dust
     need jeweler bench
     [jeweler bench], quick
  Tailoring (apprentice)
   [X] Cloth Pants:
     2x Cloth Strip,
     1x Thread Spool
     need loom [loom], quick
   [X] Linen Tunic:
     2x Cloth Strip,
     1x Thread Spool
     need loom [loom], quick`},
		{"craft view at 80", craftReadyRowsText(80), `  [V] Elixir of Renewal         : 2x Healer's Root, 1x Bitter Thistle,
      1x Dustwalk Herb, 1x Chrysalis Core, 1x bottle
      [alchemy bench], lengthy
  [V] Healing Salve             : 2x Healer's Root, 1x bottle
      [alchemy bench], quick
  [V] Phial of Second Birth     : 1x Reduction Base, 1x bottle,
      1x Unmaking Distillate, 1x First-Bloom Nectar, 6x Blood-moss,
      4x Salt Pouch
      [alchemy bench], prolonged
  [V] Stamina Tonic             : 2x Bitter Thistle, 1x bottle
      [alchemy bench], quick
  [V] Vitalis Bandolier         : 1x Reinforced Harness, 1x Preservation Runes,
      1x Chrysalis Filter-Membrane, 1x Still-Glass Rosette, 6x Pine Pitch,
      6x Salt Pouch, 4x Beeswax
      [alchemy bench], prolonged`},
		{"craft view at 42", craftReadyRowsText(42), `  [V] Elixir of Renewal:
      2x Healer's Root, 1x Bitter Thistle,
      1x Dustwalk Herb, 1x Chrysalis Core,
      1x bottle
      [alchemy bench], lengthy
  [V] Healing Salve:
      2x Healer's Root, 1x bottle
      [alchemy bench], quick
  [V] Phial of Second Birth:
      1x Reduction Base, 1x bottle,
      1x Unmaking Distillate,
      1x First-Bloom Nectar,
      6x Blood-moss, 4x Salt Pouch
      [alchemy bench], prolonged
  [V] Stamina Tonic:
      2x Bitter Thistle, 1x bottle
      [alchemy bench], quick
  [V] Vitalis Bandolier:
      1x Reinforced Harness,
      1x Preservation Runes,
      1x Chrysalis Filter-Membrane,
      1x Still-Glass Rosette,
      6x Pine Pitch, 6x Salt Pouch,
      4x Beeswax
      [alchemy bench], prolonged`},
		{"craft view at 30", craftReadyRowsText(30), `  [V] Elixir of Renewal:
    2x Healer's Root,
    1x Bitter Thistle,
    1x Dustwalk Herb,
    1x Chrysalis Core,
    1x bottle
    [alchemy bench], lengthy
  [V] Healing Salve:
    2x Healer's Root,
    1x bottle
    [alchemy bench], quick
  [V] Phial of Second Birth:
    1x Reduction Base,
    1x bottle,
    1x Unmaking Distillate,
    1x First-Bloom Nectar,
    6x Blood-moss,
    4x Salt Pouch
    [alchemy bench], prolonged
  [V] Stamina Tonic:
    2x Bitter Thistle,
    1x bottle
    [alchemy bench], quick
  [V] Vitalis Bandolier:
    1x Reinforced Harness,
    1x Preservation Runes,
    1x Chrysalis
    Filter-Membrane,
    1x Still-Glass Rosette,
    6x Pine Pitch,
    6x Salt Pouch, 4x Beeswax
    [alchemy bench], prolonged`},
	} {
		if tc.got != tc.want {
			t.Errorf("%s is not the drawing:\n%s", tc.what, lineDiffForTest(tc.want, tc.got))
		}
	}
}

// lineDiffForTest marks each line of got that differs from want.
func lineDiffForTest(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < max(len(w), len(g)); i++ {
		wl, gl := "", ""
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		mark := "  "
		if wl != gl {
			mark = "! "
		}
		fmt.Fprintf(&b, "%s%-44q %q\n", mark, wl, gl)
	}
	return b.String()
}

// shippedRecipes reads every shipped recipe, and seeds the item registry with
// every shipped item's name and component tag, so items.ComponentTagName
// names each ingredient as the game does. Restored when the test ends.
func shippedRecipes(t *testing.T) []*crafting.RecipeSpec {
	t.Helper()
	root := filepath.Join("..", "..", "_datafiles", "world", "dogmud")
	specs := map[int]*items.ItemSpec{}
	require.NoError(t, filepath.Walk(filepath.Join(root, "items"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var item struct {
			ItemId       int    `yaml:"itemid"`
			Name         string `yaml:"name"`
			ComponentTag string `yaml:"component_tag"`
		}
		if yaml.Unmarshal(raw, &item) == nil && item.ItemId != 0 {
			specs[item.ItemId] = &items.ItemSpec{ItemId: item.ItemId, Name: item.Name, ComponentTag: item.ComponentTag}
		}
		return nil
	}))
	require.Greater(t, len(specs), 400, "the shipped items did not load")
	t.Cleanup(items.SeedItemsForTest(specs))

	recipes := []*crafting.RecipeSpec{}
	require.NoError(t, filepath.Walk(filepath.Join(root, "recipes"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var r crafting.RecipeSpec
		require.NoError(t, yaml.Unmarshal(raw, &r), path)
		recipes = append(recipes, &r)
		return nil
	}))
	require.Greater(t, len(recipes), 100, "the shipped recipes did not load")
	sort.Slice(recipes, func(i, j int) bool { return recipes[i].RecipeId < recipes[j].RecipeId })
	return recipes
}

// longestReason is the longest reason recipeStatus can give for r: its skill
// rank required, its station needed, or one of its ingredients missing.
func longestReason(r *crafting.RecipeSpec) string {
	reasons := []string{fmt.Sprintf("%s skill required", skills.GetSkillRankDescription(r.SkillMinimum)),
		"need " + strings.ReplaceAll(r.Station, "_", " ")}
	for _, ing := range r.Ingredients {
		reasons = append(reasons, "missing "+items.ComponentTagName(ing.ItemTag))
	}
	longest := ""
	for _, reason := range reasons {
		if len(reason) > len(longest) {
			longest = reason
		}
	}
	return longest
}

// Every shipped recipe fits at every width, in the list with its longest
// reason and in the bare view, and keeps its units whole: each ingredient
// with its count and `[station], speed` sit on one line unless wider than
// the room left at their indent, and the name stays on the first line
// unless wider than the room after `[X] `.
func TestCraftRowsFitEveryWidthWithShippedRecipes(t *testing.T) {
	recipes := shippedRecipes(t)
	widest := map[string]int{}
	for _, width := range goScreenWidths {
		formWidth := users.RedrawnFormWidth(width)
		for _, r := range recipes {
			for _, view := range []struct {
				indent int
				row    craftRow
			}{
				{4, newCraftRow(r, "X", longestReason(r))},
				{2, newCraftRow(r, "V", "")},
			} {
				indent := view.indent
				if formWidth < 42 && indent == 4 {
					indent = 3
				}
				lines := plainLines(strings.Join(craftRowLines(view.row, indent, formWidth), "\n"))
				text := strings.Join(lines, "\n")
				where := fmt.Sprintf("%s (indent %d) at %d", r.RecipeId, indent, width)
				for _, line := range lines {
					w := templates.WidestLine(line)
					widest[fmt.Sprint(width)] = max(widest[fmt.Sprint(width)], w)
					require.LessOrEqual(t, w, width, "%s:\n%s", where, text)
				}
				cont := indent + 4
				if formWidth < 42 {
					cont = indent + 2
				}
				units := append([]string{}, view.row.ingredients...)
				units = append(units, "["+view.row.station+"], "+view.row.speed)
				for _, unit := range units {
					if len(unit)+1 <= formWidth-cont {
						require.Contains(t, text, unit, "%s splits %q:\n%s", where, unit, text)
					}
				}
				if len(view.row.name)+1 <= formWidth-indent-4 {
					name := view.row.name + ":"
					if formWidth >= 80 {
						name = view.row.name // padded to its column
					}
					require.Contains(t, lines[0], name, "%s splits the name:\n%s", where, text)
				}
			}
		}
	}
	t.Logf("widest row line per width: %v", widest)
}

// craftCommand runs `craft <rest>` for user 1 knowing recipes, at width,
// and returns what was queued.
func craftCommand(t *testing.T, width int, rest string, recipes ...*crafting.RecipeSpec) string {
	t.Helper()
	user, room := getTestUserAndRoom(t)
	prevWidth, prevKnown := user.LineWidth, user.Character.KnownRecipes
	t.Cleanup(func() { user.LineWidth, user.Character.KnownRecipes = prevWidth, prevKnown })
	user.LineWidth = width
	user.Character.KnownRecipes = map[string]int{}
	for _, r := range recipes {
		crafting.RegisterRecipeForTest(r)
		id := r.RecipeId
		t.Cleanup(func() { crafting.UnregisterRecipeForTest(id) })
		user.Character.KnownRecipes[r.RecipeId] = 1
	}
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Craft(rest, user, room, 0)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
}

// `craft list`, knowing every shipped recipe, fits at every width and
// draws the form for it: at 80 each recipe takes a second line under its
// name, at 42 and 30 the name ends in its `:`.
func TestCraftListFitsEveryWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	recipes := shippedRecipes(t)
	for _, width := range goScreenWidths {
		out := craftCommand(t, width, "list", recipes...)
		require.Contains(t, out, "Crafting Recipes", "at %d", width)
		require.LessOrEqual(t, templates.WidestLine(out), width, "craft list at %d:\n%s", width, out)
		plain := strings.Join(plainLines(out), "\n")
		if width >= 80 {
			require.Contains(t, plain, "    [X] Healing Salve             : 2x Healer's Root, 1x bottle\n        ", "at %d:\n%s", width, plain)
		} else {
			require.Contains(t, plain, "[X] Healing Salve:\n", "at %d:\n%s", width, plain)
		}
	}
}

// The bare `craft` view fits and draws its form too, with a recipe that can
// be made anywhere from nothing.
func TestCraftViewFitsEveryWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	free := &crafting.RecipeSpec{RecipeId: "test-wayfarer-pack", Name: "Wayfarer's Bottomless Pack", Skill: "tailoring", TimeRounds: 20}
	for _, width := range goScreenWidths {
		out := craftCommand(t, width, "", free)
		require.LessOrEqual(t, templates.WidestLine(out), width, "craft at %d:\n%s", width, out)
		plain := strings.Join(plainLines(out), "\n")
		// The 26-letter name does not fit after `[V] ` at 30, so it splits
		// at a space and keeps that indent (ruling 3).
		want := "\n  [V] Wayfarer's Bottomless\n      Pack:\n    prolonged\n"
		if width >= 42 {
			want = "\n  [V] Wayfarer's Bottomless Pack:\n      prolonged\n"
		}
		require.Contains(t, plain, want, "at %d:\n%s", width, plain)
	}
}

// The closing line keeps its 80-column form where it fits, and its value
// (a unit) continues 2 in where it does not, as drawn.
func TestCraftFooterLines(t *testing.T) {
	for _, tc := range []struct {
		width int
		want  string
	}{
		{80, "Overall recipe knowledge: a handful of recipes"},
		{42, "Overall recipe knowledge:\n  a handful of recipes"},
		{30, "Overall recipe knowledge:\n  a handful of recipes"},
		{30, "Overall recipe knowledge:\n  near-complete mastery"},
	} {
		value := strings.SplitN(tc.want, ": ", 2)
		v := strings.TrimSpace(value[len(value)-1])
		if len(value) == 1 {
			v = strings.TrimSpace(strings.SplitN(tc.want, ":\n", 2)[1])
		}
		got := strings.Join(plainLines(strings.Join(craftFooterLines("cyan", "Overall recipe knowledge:", v, tc.width), "\n")), "\n")
		require.Equal(t, tc.want, got, "at %d", tc.width)
	}
}
