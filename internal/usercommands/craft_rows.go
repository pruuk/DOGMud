package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// craftRow is one recipe as `craft list` and the bare `craft` view show it:
// its status, name, ingredients, why it cannot be made here (if it cannot),
// and its station and speed.
type craftRow struct {
	indicator   string   // "V" (can be made here) or "X"
	name        string   // the recipe name, an enchanting one with its slot
	ingredients []string // "2x Healer's Root", one per ingredient
	reason      string   // why it cannot be made here; "" when it can
	station     string   // "alchemy bench"; "" when it needs none
	speed       string   // craftTimeDesc
}

// newCraftRow is r as a row with the given status.
func newCraftRow(r *crafting.RecipeSpec, indicator, reason string) craftRow {
	row := craftRow{indicator: indicator, name: r.Name, reason: reason,
		station: strings.ReplaceAll(r.Station, "_", " "), speed: craftTimeDesc(r.TimeRounds)}
	// Enchanting recipes target an equipped item; the name carries the
	// slot for at-a-glance lookup.
	if crafting.IsEnchantingRecipe(r) && r.TargetType != "" {
		row.name = fmt.Sprintf("%s (%s)", r.Name, r.TargetType)
	}
	for _, ing := range r.Ingredients {
		row.ingredients = append(row.ingredients, fmt.Sprintf("%dx %s", ing.Quantity, items.ComponentTagName(ing.ItemTag)))
	}
	return row
}

// craftRowLines lays row out for a form formWidth columns wide (80, 42 or
// 30, redrawnFormWidth), its first line indented by indent: 4 in `craft
// list`, under a skill heading, and 2 in the bare `craft` view (3 and 2 at
// 30). Plan 3b-3, from the drawings craft_list (narrow-screens-3b-mock) and
// craft_ready (narrow-screens-3b3-mock), ruling 9:
//
//   - 80: the status and the name padded to 26 columns, then ": " and the
//     ingredients, wrapping under the name; then a line under the name
//     with the reason and `[station], speed`.
//   - 42: `[X] Name:`, then the ingredients on lines of their own under the
//     name, then the reason and `[station], speed`.
//   - 30: as 42, but the lines after the first sit 2 in from the row.
//
// An ingredient with its count, and `[station], speed`, are units: a line
// breaks between them, never inside one, unless one is wider than the room
// left at its indent; then it splits at a space and keeps the indent
// (ruling 3). So does the name, at the indent after `[X] `.
func craftRowLines(row craftRow, indent, formWidth int) []string {
	colour := "green"
	if row.reason != "" {
		colour = "red"
	}
	status := fmt.Sprintf(`<ansi fg="%s">[%s]</ansi>`, colour, row.indicator)
	under := indent + 4 // the column the name starts in
	cont := under
	if formWidth < 42 {
		cont = indent + 2
	}

	ingredients := make([]string, len(row.ingredients))
	for i, ing := range row.ingredients {
		ingredients[i] = ing
		if i < len(row.ingredients)-1 {
			ingredients[i] += ","
		}
	}
	tail := []string{}
	if row.reason != "" {
		tail = append(tail, `<ansi fg="red">`+row.reason+`</ansi>`)
	}
	where := row.speed
	if row.station != "" {
		where = "[" + row.station + "], " + row.speed
	}
	tail = append(tail, `<ansi fg="dark-cyan">`+where+`</ansi>`)

	var lines []string
	if formWidth >= 80 {
		lead := fmt.Sprintf(`%s <ansi fg="white">%-26s</ansi>:`, status, row.name)
		lines = packUnits(append([]string{lead}, ingredients...), indent, cont, formWidth)
	} else {
		pad := strings.Repeat(" ", indent)
		for i, part := range strings.Split(messaging.WrapAnsi(`<ansi fg="white">`+row.name+`</ansi>:`, formWidth-under), "\n") {
			if i == 0 {
				lines = append(lines, pad+status+" "+part)
				continue
			}
			lines = append(lines, strings.Repeat(" ", under)+part)
		}
		if len(ingredients) > 0 {
			lines = append(lines, packUnits(ingredients, cont, cont, formWidth)...)
		}
	}
	return append(lines, packUnits(tail, cont, cont, formWidth)...)
}

// craftFooterLines is a closing "Label: value" line of a craft screen, in
// colour: as one line when it fits width, else broken between its words
// with the value, a unit, continuing 2 in (ruling 2).
func craftFooterLines(colour, label, value string, width int) []string {
	line := label + " " + value
	if util.VisibleWidth(line) <= width {
		return []string{`<ansi fg="` + colour + `">` + line + `</ansi>`}
	}
	lines := packUnits(append(strings.Fields(label), value), 0, 2, width)
	for i := range lines {
		lines[i] = `<ansi fg="` + colour + `">` + lines[i] + `</ansi>`
	}
	return lines
}
