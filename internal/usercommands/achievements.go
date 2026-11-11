package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/achievements"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// achievementCategories is the display order.
var achievementCategories = []string{
	achievements.CategoryCombat,
	achievements.CategoryExploration,
	achievements.CategoryWealth,
	achievements.CategoryProgression,
	achievements.CategoryQuests,
}

// achievementProgressHint renders a locked achievement's progress toward its goal.
func achievementProgressHint(d achievements.Definition, c *characters.Character) string {
	if cur, tgt, numeric := achievements.Progress(d.Trigger, c); numeric {
		return fmt.Sprintf("%d/%d", cur, tgt)
	}
	return "not yet"
}

// Achievements lists the caller's accolades grouped by category, with a progress
// hint for locked ones and a running total.
func Achievements(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	filter := strings.ToLower(strings.TrimSpace(rest))
	defs := achievements.All()
	char := user.Character

	if len(defs) == 0 {
		user.SendText(messaging.CategorySystem, `There are no accolades to earn yet.`)
		return true, nil
	}

	entries := make([]achievementEntry, 0, len(defs))
	for _, d := range defs {
		e := achievementEntry{def: d, earned: char.HasAchievement(d.Id)}
		if !e.earned {
			e.hint = achievementProgressHint(d, char)
		}
		entries = append(entries, e)
	}

	// The list is a laid-out screen (a summary, bare category headings and
	// rows), so it rides CategoryScreen: on CategorySystem the normalizer
	// put a stop after every heading ("Combat.").
	for _, msg := range achievementsScreen(entries, achievements.PointsFor(char.Achievements),
		user.AchievementsExcluded(), filter, redrawnFormWidth(user.WrapWidth())) {
		user.SendText(messaging.CategoryScreen, msg)
	}

	return true, nil
}

// achievementEntry is one achievement as the list shows it: earned, or
// locked with its progress hint (achievementProgressHint).
type achievementEntry struct {
	def    achievements.Definition
	earned bool
	hint   string
}

// achievementsScreen is the achievements list, one message per line as it
// is sent (a heading carries the blank line above it), for a form formWidth
// columns wide (redrawnFormWidth: 80, 42 or 30). Plan 3b-3, from the drawing
// achievements (narrow-screens-3b3-mock): a line that fits the form is the
// 80-column line; one that does not breaks between words, a row's
// `[x] Name (points):` kept whole, continuing 2 past the `[x]` (ruling 2).
func achievementsScreen(entries []achievementEntry, points int, excluded bool, filter string, formWidth int) []string {
	earnedCount := 0
	for _, e := range entries {
		if e.earned {
			earnedCount++
		}
	}
	msgs := []string{strings.Join(messaging.PackUnits([]string{
		`<ansi fg="yellow-bold">Achievements</ansi>:`, `earned`,
		fmt.Sprintf(`<ansi fg="white-bold">%d/%d</ansi>,`, earnedCount, len(entries)),
		fmt.Sprintf(`<ansi fg="yellow">%d points</ansi>`, points)}, 0, 2, formWidth), "\n")}

	if excluded {
		msgs = append(msgs, messaging.WrapAnsi(`<ansi fg="white">Achievements are not tracked on this account.</ansi>`, formWidth))
	}

	for _, cat := range achievementCategories {
		if filter != "" && filter != cat {
			continue
		}
		var inCat []achievementEntry
		for _, e := range entries {
			if e.def.Category == cat {
				inCat = append(inCat, e)
			}
		}
		if len(inCat) == 0 {
			continue
		}

		// The blank line rides in the heading's own text: an empty SendText
		// delivers nothing.
		msgs = append(msgs, fmt.Sprintf("\n"+`<ansi fg="cyan-bold">%s%s</ansi>`, strings.ToUpper(cat[:1]), cat[1:]))

		for _, e := range inCat {
			msgs = append(msgs, strings.Join(achievementRowLines(e, formWidth), "\n"))
		}
	}
	return msgs
}

// achievementRowLines is one row of the list for a form formWidth wide: the
// 80-column line when it fits, else `[x] Name (points):` and the
// description's words packed at indent 2, continuing at 4. A locked row is
// grey on every line, and its progress hint follows the description two
// spaces on when it fits there, else takes a line of its own at 4.
func achievementRowLines(e achievementEntry, formWidth int) []string {
	d := e.def
	words := strings.Fields(d.Description)
	if e.earned {
		line := fmt.Sprintf(`  <ansi fg="green">[x] %s</ansi> <ansi fg="yellow">(%d)</ansi>: %s`, d.Name, d.Points, d.Description)
		if util.VisibleWidth(line) <= formWidth {
			return []string{line}
		}
		lead := fmt.Sprintf(`<ansi fg="green">[x] %s</ansi> <ansi fg="yellow">(%d)</ansi>:`, d.Name, d.Points)
		return messaging.PackUnits(append([]string{lead}, words...), 2, 4, formWidth)
	}
	hint := `<ansi fg="white">(` + e.hint + `)</ansi>`
	line := fmt.Sprintf(`  <ansi fg="black-bold">[ ] %s (%d): %s</ansi>  %s`, d.Name, d.Points, d.Description, hint)
	if util.VisibleWidth(line) <= formWidth {
		return []string{line}
	}
	lines := messaging.PackUnits(append([]string{fmt.Sprintf(`[ ] %s (%d):`, d.Name, d.Points)}, words...), 2, 4, formWidth)
	for i, l := range lines {
		text := strings.TrimLeft(l, " ")
		lines[i] = l[:len(l)-len(text)] + `<ansi fg="black-bold">` + text + `</ansi>`
	}
	last := len(lines) - 1
	if util.VisibleWidth(lines[last])+2+util.VisibleWidth(hint) <= formWidth {
		lines[last] += "  " + hint
	} else {
		lines = append(lines, "    "+hint)
	}
	return lines
}
