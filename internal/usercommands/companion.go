package usercommands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Companion handles the `companion` command in four forms:
//
//	companion                          — list all companions
//	companion <name>                   — detailed view (no hard numbers)
//	companion <name> assist on/off     — toggle auto-assist
//	companion <name> name <nickname>   — rename a companion
func Companion(rest string, user *users.UserRecord,
	room *rooms.Room, flags events.EventFlag) (bool, error) {

	companions := user.Character.Companions

	// ── Form 1: no args → list ──────────────────────────────────────────────
	if rest == "" {
		if len(companions) == 0 {
			refusal.NoCompanion(user)
			return true, nil
		}

		for _, msg := range companionList(companions, user.WrapWidth()) {
			user.SendText(messaging.CategorySystem, msg)
		}
		return true, nil
	}

	// ── Parse remaining forms ────────────────────────────────────────────────
	lowerRest := strings.ToLower(rest)
	var assistToggle string // "on", "off", or "" (no toggle)
	var renameNick string   // non-empty if "name <nick>" form
	compName := rest

	if strings.HasSuffix(lowerRest, " assist on") {
		assistToggle = "on"
		compName = rest[:len(rest)-len(" assist on")]
	} else if strings.HasSuffix(lowerRest, " assist off") {
		assistToggle = "off"
		compName = rest[:len(rest)-len(" assist off")]
	} else if idx := strings.Index(lowerRest, " name "); idx >= 0 {
		// "companion <name> name <nickname>"
		compName = rest[:idx]
		renameNick = strings.TrimSpace(rest[idx+len(" name "):])
	}
	compName = strings.TrimSpace(compName)

	comp := user.Character.GetCompanion(compName)
	if comp == nil {
		user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
			`You have no companion named "%s".`, refusal.Echo(compName),
		))
		return true, nil
	}

	// ── Form 4: rename companion ─────────────────────────────────────────────
	if renameNick != "" {
		// A bonded companion already has a name of its own.
		if comp.SourceType == characters.CompanionBonded {
			user.SendText(messaging.CategoryRefusal, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> already has a name.`, comp.Name,
			))
			return true, nil
		}
		if err := validateCompanionName(renameNick); err != nil {
			user.SendText(messaging.CategoryRefusal, err.Error())
			return true, nil
		}

		// Backfill BaseName for pre-existing companions that lack it.
		if comp.BaseName == "" {
			comp.BaseName = comp.Name
		}

		oldName := comp.Name
		newDisplayName := renameNick + " the " + comp.BaseName
		comp.Nickname = renameNick
		comp.Name = newDisplayName

		// Update the live mob's name so it shows in room/combat text.
		if mob := mobs.GetInstance(comp.InstanceId); mob != nil {
			mob.Character.Name = newDisplayName
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> is now known as `+
				`<ansi fg="mobname">%s</ansi>.`,
			oldName, newDisplayName,
		))
		return true, nil
	}

	// ── Form 3: toggle assist ────────────────────────────────────────────────
	if assistToggle != "" {
		comp.AutoAssist = assistToggle == "on"
		stateStr := "off"
		if comp.AutoAssist {
			stateStr = "on"
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s auto-assist is now `+
				`<ansi fg="yellow">%s</ansi>.`,
			comp.Name, stateStr,
		))
		return true, nil
	}

	// ── Form 2: detailed view ────────────────────────────────────────────────
	mob := mobs.GetInstance(comp.InstanceId)

	assistStr := "off"
	if comp.AutoAssist {
		assistStr = "on"
	}

	for _, line := range companionDetail(comp, mob, assistStr, user.WrapWidth()) {
		user.SendText(line.cat, line.text)
	}

	return true, nil
}

// Narrow forms of the companion screens (web client base spec, section 2),
// picked as templates.Process picks a template's forms: the 80-column screen
// goes to any reader it fits, as it always did. A reader it does not fit gets
// the form drawn to 42 columns, or to 30 below that: the heading rule runs
// the full width, the bars stack one per line, and words break only between
// whole names. Every form shows everything the 80-column screen does.

// companionNarrowWidth is the width to draw a companion screen to for a
// reader width columns wide: 0 when the 80-column lines fit, else 42, or 30
// (or the reader's width, if narrower still).
func companionNarrowWidth(width int, lines ...string) int {
	fits := true
	for _, line := range lines {
		if util.VisibleWidth(line) > width {
			fits = false
			break
		}
	}
	switch {
	case fits:
		return 0
	case width >= 42:
		return 42
	default:
		return min(width, 30)
	}
}

// packCompanionUnits lays units out in lines of at most width columns: the
// first line indented by first, the rest by rest, breaking only between
// units. A unit goes on the line in hand when it fits there (the first line
// measured from its own indent), else on a new line. Only a unit too long
// for a whole new line is wrapped between its words (messaging.WrapAnsi).
func packCompanionUnits(units []string, first, rest, width int) []string {
	lines := []string{}
	line, used := strings.Repeat(" ", first), first
	empty := true
	newLine := func() {
		lines = append(lines, line)
		line, used, empty = strings.Repeat(" ", rest), rest, true
	}
	place := func(word string) {
		w := util.VisibleWidth(word)
		sep := 1
		if empty {
			sep = 0
		}
		if used+sep+w > width && !empty {
			newLine()
			sep = 0
		}
		line += strings.Repeat(" ", sep) + word
		used += sep + w
		empty = false
	}
	for _, unit := range units {
		w := util.VisibleWidth(unit)
		sep := 1
		if empty {
			sep = 0
		}
		if used+sep+w <= width || w <= width-rest {
			place(unit)
			continue
		}
		if !empty {
			newLine()
		}
		for _, part := range strings.Split(messaging.WrapAnsi(unit, width-rest), "\n") {
			place(part)
		}
	}
	lines = append(lines, line)
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return lines
}

// companionRule is a heading on a rule that spans width: "━━━ title "
// filled with "━" to width when at least three fit after it. A longer title
// goes on its own line or lines, broken only between its parts (a name is
// one part), and a full-width rule follows.
func companionRule(width int, parts ...string) string {
	title := strings.Join(parts, ` `)
	head := `━━━ ` + title + ` `
	if util.VisibleWidth(head)+3 <= width {
		return `<ansi fg="yellow">` + head + strings.Repeat(`━`, width-util.VisibleWidth(head)) + `</ansi>`
	}
	lines := packCompanionUnits(parts, 0, 0, width)
	return `<ansi fg="yellow">` + strings.Join(lines, "\n") + "\n" + strings.Repeat(`━`, width) + `</ansi>`
}

// companionMutationLines is "Mutations: a, b, c" at indent, each entry (a
// name with its level and comma) kept whole. The entries hang under the first
// one when that leaves 20 columns and every entry fits there; else they sit
// 2 past the label; else, when an entry is too long even for that, under the
// label itself. Only an entry longer than a whole line at the label's indent
// is broken.
func companionMutationLines(label, mutList string, indent, width int) []string {
	names := strings.Split(mutList, ", ")
	for i := range names[:len(names)-1] {
		names[i] += ","
	}
	longest := 0
	for _, name := range names {
		longest = max(longest, util.VisibleWidth(name))
	}
	hang := indent
	for _, try := range []int{indent + util.VisibleWidth(label) + 1, indent + 2} {
		if width-try >= longest && (try == indent+2 || width-try >= 20) {
			hang = try
			break
		}
	}
	return packCompanionUnits(append([]string{label}, names...), indent, hang, width)
}

// companionList is the messages of the companion list for a reader width
// columns wide.
func companionList(companions []characters.CompanionInfo, width int) []string {
	type row struct {
		name, kind, offline, assist string
		mob                         *mobs.Mob
		bars                        [3]string
		mutList                     string
	}
	rows := make([]row, 0, len(companions))
	wide := []string{`━━━ Companions ━━━`}
	for _, comp := range companions {
		assistStr := "off"
		if comp.AutoAssist {
			assistStr = "on"
		}
		r := row{name: `<ansi fg="mobname">` + comp.Name + `</ansi>`, kind: `[` + string(comp.SourceType) + `]`,
			assist: `(assist: ` + assistStr + `)`, mob: mobs.GetInstance(comp.InstanceId)}
		if r.mob == nil {
			r.offline = `<ansi fg="239">[offline]</ansi>`
			wide = append(wide, fmt.Sprintf(`  %s %s %s %s`, r.name, r.kind, r.offline, r.assist))
		} else {
			c := r.mob.Character
			r.bars = [3]string{
				users.RenderVitalBar(c.Health, c.HealthMax.Value, 0),
				users.RenderVitalBar(c.Stamina, c.StaminaMax.Value, 0),
				users.RenderVitalBar(c.Conviction, c.ConvictionMax.Value, 0),
			}
			r.mutList = formatCompanionMutations(c.Mutations)
			wide = append(wide, fmt.Sprintf(`  %s %s %s`, r.name, r.kind, r.assist),
				fmt.Sprintf(`    HP: %s  SP: %s  CP: %s`, r.bars[0], r.bars[1], r.bars[2]))
			if r.mutList != "" {
				wide = append(wide, `    Mutations: `+r.mutList)
			}
		}
		rows = append(rows, r)
	}

	// The pipeline has always wrapped a long mutations line at 80 columns,
	// so only the laid-out lines decide there; below 80 every line does.
	measured := wide
	if width >= users.MaxAutoWrapWidth {
		measured = []string{}
		for _, line := range wide {
			if !strings.HasPrefix(line, `    Mutations: `) {
				measured = append(measured, line)
			}
		}
	}
	formWidth := companionNarrowWidth(width, measured...)
	if formWidth == 0 {
		// As it always was: the heading, then a name line, a bars line
		// and a mutations line per companion.
		msgs := []string{`<ansi fg="yellow">━━━ Companions ━━━</ansi>`}
		return append(msgs, wide[1:]...)
	}

	msgs := []string{companionRule(formWidth, `Companions`)}
	for _, r := range rows {
		units := []string{r.name, r.kind}
		if r.offline != "" {
			units = append(units, r.offline)
		}
		msgs = append(msgs, strings.Join(packCompanionUnits(append(units, r.assist), 2, 4, formWidth), "\n"))
		if r.mob == nil {
			continue
		}
		msgs = append(msgs, `    HP: `+r.bars[0], `    SP: `+r.bars[1], `    CP: `+r.bars[2])
		if r.mutList != "" {
			msgs = append(msgs, strings.Join(companionMutationLines(`Mutations:`, r.mutList, 4, formWidth), "\n"))
		}
	}
	return msgs
}

// companionLine is one message of the companion detail view.
type companionLine struct {
	cat  messaging.Category
	text string
}

// companionDetail is the detail view of one companion for a reader width
// columns wide. mob is nil when the companion is not present.
func companionDetail(comp *characters.CompanionInfo, mob *mobs.Mob, assistStr string, width int) []companionLine {
	heading := fmt.Sprintf(`<ansi fg="yellow">━━━ %s [%s] ━━━</ansi>`, comp.Name, comp.SourceType)
	assist := fmt.Sprintf(`  Auto-Assist: %s`, assistStr)
	if mob == nil {
		absent := `  This companion is not currently present.`
		formWidth := companionNarrowWidth(width, heading, absent, assist)
		if formWidth == 0 {
			return []companionLine{{messaging.CategorySystem, heading},
				{messaging.CategorySystem, absent}, {messaging.CategoryScreen, assist}}
		}
		return []companionLine{{messaging.CategorySystem, companionDetailRule(comp, formWidth)},
			{messaging.CategorySystem, strings.Join(packCompanionUnits(strings.Fields(absent), 2, 2, formWidth), "\n")},
			{messaging.CategoryScreen, assist}}
	}

	c := mob.Character
	vitals := [3]string{vitalDesc(c.Health, c.HealthMax.Value), vitalDesc(c.Stamina, c.StaminaMax.Value),
		vitalDesc(c.Conviction, c.ConvictionMax.Value)}
	stats := [6]string{statQualityDesc(c.Stats.Strength.ValueAdj), statQualityDesc(c.Stats.Dexterity.ValueAdj),
		statQualityDesc(c.Stats.Perception.ValueAdj), statQualityDesc(c.Stats.Vitality.ValueAdj),
		statQualityDesc(c.Stats.Willpower.ValueAdj), statQualityDesc(c.Stats.Charisma.ValueAdj)}
	mutList := formatCompanionMutations(c.Mutations)

	wide := []string{heading,
		// Vitals descriptions (no hard numbers)
		fmt.Sprintf(`  Health: %-10s Stamina: %-10s Conviction: %s`, vitals[0], vitals[1], vitals[2]),
		// Stat descriptions using the same tiers as the player status command
		fmt.Sprintf(`  <ansi fg="yellow">Strength:</ansi>   %-13s <ansi fg="yellow">Dexterity:</ansi>  %-13s <ansi fg="yellow">Perception:</ansi> %s`,
			stats[0], stats[1], stats[2]),
		fmt.Sprintf(`  <ansi fg="yellow">Vitality:</ansi>   %-13s <ansi fg="yellow">Willpower:</ansi>  %-13s <ansi fg="yellow">Charisma:</ansi>   %s`,
			stats[3], stats[4], stats[5]),
	}
	if mutList != "" {
		wide = append(wide, fmt.Sprintf(`  <ansi fg="yellow">Mutations:</ansi>  %s`, mutList))
	}
	wide = append(wide, assist)

	formWidth := companionNarrowWidth(width, wide...)
	if formWidth == 0 {
		lines := []companionLine{{messaging.CategorySystem, wide[0]}}
		for _, text := range wide[1:] {
			lines = append(lines, companionLine{messaging.CategoryScreen, text})
		}
		return lines
	}

	// One label and value per line, the label column 13 wide, as the
	// status sheet's 30-column form lists them.
	pair := func(label, value string, yellow bool) string {
		padded := fmt.Sprintf(`%-13s`, label)
		if yellow {
			padded = `<ansi fg="yellow">` + padded + `</ansi>`
		}
		return `  ` + padded + value
	}
	lines := []companionLine{{messaging.CategorySystem, companionDetailRule(comp, formWidth)}}
	for i, label := range []string{`Health:`, `Stamina:`, `Conviction:`} {
		lines = append(lines, companionLine{messaging.CategoryScreen, pair(label, vitals[i], false)})
	}
	for i, label := range []string{`Strength:`, `Dexterity:`, `Perception:`, `Vitality:`, `Willpower:`, `Charisma:`} {
		lines = append(lines, companionLine{messaging.CategoryScreen, pair(label, stats[i], true)})
	}
	if mutList != "" {
		lines = append(lines, companionLine{messaging.CategoryScreen,
			strings.Join(companionMutationLines(`<ansi fg="yellow">Mutations:</ansi>  `, mutList, 2, formWidth), "\n")})
	}
	return append(lines, companionLine{messaging.CategoryScreen, pair(`Auto-Assist:`, assistStr, false)})
}

// companionDetailRule is the detail view's heading at formWidth: the name
// and kind on a full-width rule, as companionRule draws it.
func companionDetailRule(comp *characters.CompanionInfo, formWidth int) string {
	return companionRule(formWidth, comp.Name, `[`+string(comp.SourceType)+`]`)
}

// formatCompanionMutations renders a companion's mutation list as a
// comma-separated string of display names, with "(Ln)" suffix for levels
// greater than 1. Returns "" if the mob has no mutations.
func formatCompanionMutations(muts map[string]int) string {
	if len(muts) == 0 {
		return ""
	}

	ids := make([]string, 0, len(muts))
	for id := range muts {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		spec := mutations.GetMutation(id)
		name := id
		if spec != nil && spec.Name != "" {
			name = spec.Name
		}
		if lvl := muts[id]; lvl > 1 {
			parts = append(parts, fmt.Sprintf("%s (L%d)", name, lvl))
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}

// vitalDesc returns a qualitative description of a vital resource pool.
func vitalDesc(current, max int) string {
	if max <= 0 {
		return "unknown"
	}
	pct := float64(current) / float64(max)
	switch {
	case pct >= 0.95:
		return "pristine"
	case pct >= 0.75:
		return "strong"
	case pct >= 0.50:
		return "steady"
	case pct >= 0.25:
		return "waning"
	case pct >= 0.10:
		return "critical"
	default:
		return "failing"
	}
}

// statQualityDesc returns the same descriptive tier used by the player
// status command. Matches the statQuality template function exactly.
func statQualityDesc(value int) string {
	switch {
	case value <= 60:
		return "feeble"
	case value <= 75:
		return "poor"
	case value <= 90:
		return "modest"
	case value <= 110:
		return "average"
	case value <= 130:
		return "keen"
	case value <= 150:
		return "exceptional"
	case value <= 200:
		return "remarkable"
	case value <= 300:
		return "transcendent"
	default:
		return "godlike"
	}
}

// validateCompanionName checks a proposed companion nickname against the
// banned-names list, mob names, existing player character names, and existing
// companion nicknames. Names are freed when companions die or are dismissed.
func validateCompanionName(name string) error {
	// Companion names use a stricter character set than general validation
	// (letters only, 2-20 chars). Run that gate first.
	if len(name) < 2 || len(name) > 20 {
		return fmt.Errorf("Companion names must be between 2 and 20 characters.")
	}
	for _, ch := range name {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') {
			return fmt.Errorf("Companion names may only contain letters.")
		}
	}

	// Standard collision checks (banned, mob, player, companion).
	if err := users.ValidateActorName(name, users.ValidateActorOpts{}); err != nil {
		return err
	}
	return nil
}
