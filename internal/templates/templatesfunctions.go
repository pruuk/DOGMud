package templates

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/mattn/go-runewidth"
)

var (
	funcMap = template.FuncMap{
		"pad":       pad,
		"padLeft":   padLeft,
		"padRight":  padRight,
		"padRightX": padRightX,
		"join":      join,
		"lte":       func(a, b int) bool { return a <= b },
		"gte":       func(a, b int) bool { return a >= b },
		"lt":        func(a, b int) bool { return a < b },
		"uc":        func(s string) string { return strings.Title(s) },
		"firstletter": func(a string) string {
			if len(a) == 0 {
				return ""
			}
			return string(a[0])
		},
		"repeat": func(what string, count int) string {
			if count < 1 {
				return ""
			}
			return strings.Repeat(what, count)
		},
		"add": func(start int, num ...int) int {
			for _, n := range num {
				start += n
			}
			return start
		},
		"sub": func(start int, num ...int) int {
			for _, n := range num {
				start -= n
			}
			return start
		},
		"mult": func(start int, num ...int) int {
			for _, n := range num {
				start *= n
			}
			return start
		},
		"multfloat": func(start float32, num ...float32) float32 {
			for _, n := range num {
				start *= n
			}
			return start
		},
		"cellpad": func(padding int, s string) string {
			if padding < 1 {
				return s
			}
			pad := strings.Repeat(" ", padding)
			return pad + s + pad
		},
		"lowercase": func(s string) string {
			return strings.ToLower(s)
		},
		// The map legend's colour slug, shared with the minimap and the
		// map command so the legend's tag matches theirs (#455).
		"mapslug": mapper.LegendSlug,
		// The three spell-reference values a player used to be shown as raw
		// numbers. Each DELEGATES to internal/combat rather than restating the
		// bands here: the `spells` command already describes cost with
		// GetConvictionCostDescription, and a helpfile that invented its own
		// vocabulary would drift from it silently -- the same failure mode that
		// let the admin progression dashboard diverge from production for months.
		"convictioncost": combat.GetConvictionCostDescription,
		"castlength":     combat.GetCastLengthDescription,
		"waittime":       combat.GetWaitRoundsDescription,
		"mapkeys": func(m map[string]string) []string {
			keys := make([]string, len(m))
			i := 0
			for k := range m {
				keys[i] = k
				i++
			}
			return keys
		},
		"intmapkeys": func(m map[string]int) []string {
			keys := make([]string, len(m))
			i := 0
			for k := range m {
				keys[i] = k
				i++
			}
			return keys
		},
		"uidToCharacter": func(userId int) *characters.Character {
			u := users.GetByUserId(userId)
			return u.Character
		},
		"idToMobCharacter": func(mobId int) string {
			if m := mobs.GetInstance(mobId); m != nil {
				return m.Character.Name
			}
			return "glorble"
		},
		"idsOtherThan": idsOtherThan,
		"tnl":          TNL,
		"pct":          pct,
		"numberFormat": numberFormat,
		"mod":          func(a, b int) int { return a % b },
		"divFloat":     func(a int, b int) float64 { return float64(a) / float64(b) },
		"stringor":     stringOr,
		"splitstring":  util.SplitStringNL,
		// screenwidth is the reader's width; Process replaces it per render
		// (forms.go). Rendered with no reader, it is FullWidth.
		"screenwidth": func() int { return FullWidth },
		"min":         func(a, b int) int { return min(a, b) },
		// visiblewidth is the columns s takes on screen, colour tags not
		// counted, so a narrow form can ask whether a coloured value fits.
		"visiblewidth": util.VisibleWidth,
		// trimspace drops the padding a localized label carries, so a 30
		// form can follow it with one space in every language.
		"trimspace":   strings.TrimSpace,
		"wraptext":    wrapText,
		"centerlines": centerLines,
		"wrapspans":   wrapSpans,
		"wrapbreak":   wrapBreak,
		"ansiparse":   TplAnsiParse,
		"conditionname": func(conditionId int) string {
			conditionSpec := conditions.GetConditionSpec(conditionId)
			if conditionSpec == nil {
				return "Unknown"
			}
			return conditionSpec.Name
		},
		"conditionduration": func(conditionId int) string {
			conditionSpec := conditions.GetConditionSpec(conditionId)
			if conditionSpec == nil {
				return "Unknown"
			}

			if conditionSpec.RoundInterval == 1 && conditionSpec.TriggerCount == 1 {
				return `Activates once`
			}

			var roundCt string
			if conditionSpec.RoundInterval > 1 {
				roundCt = fmt.Sprintf(`%d rounds`, conditionSpec.RoundInterval)
			} else {
				roundCt = `round`
			}
			return fmt.Sprintf("Activates every %s (%dx total)", roundCt, conditionSpec.TriggerCount)
		},
		"formatdiceroll": func(roll string) string {
			a, d, s, b, _ := util.ParseDiceRoll(roll)
			return util.FormatDiceRoll(a, d, s, b, []int{})
		},
		"profession": func(char characters.Character) string {
			return skills.GetTitle(char.Mutations, char.GetAllSkillRanks(), char.Stats)
		},
		"title": func(char characters.Character) string {
			return skills.GetTitle(char.Mutations, char.GetAllSkillRanks(), char.Stats)
		},
		"roundstotime": func(rounds int) string {
			if rounds >= conditions.TriggersLeftUnlimited {
				return `Unlimited`
			}
			return formatDuration(rounds * int(configs.GetTimingConfig().RoundSeconds))
		},
		"secondsFrom": func(t time.Time) int {
			// return the number of seconds unti the given time
			return int(time.Until(t).Seconds())
		},
		"intstrlen": func(i ...int) int {
			totalLen := 0
			for _, n := range i {
				totalLen += len(strconv.Itoa(n))
			}
			return totalLen
		},
		"healthStr": func(h int, hMax int, padTo ...int) string {
			// U5b-2: pools store overkill; displays never show it. QuantizeTens
			// has no negative guard, so an unclamped h also yields a malformed
			// ANSI class.
			if h < 0 {
				h = 0
			}
			padding := ``
			if len(padTo) > 0 {
				padding = strings.Repeat(" ", padTo[0]-(len(strconv.Itoa(h))+len(strconv.Itoa(hMax))+1))
			}
			hLevel := util.QuantizeTens(h, hMax)
			return fmt.Sprintf(`<ansi fg="health-%d">%d</ansi>/<ansi fg="health-%d">%d</ansi>%s`, hLevel, h, hLevel, hMax, padding)
		},
		"manaStr": func(m int, mMax int, padTo ...int) string {

			padding := ``
			if len(padTo) > 0 {
				padding = strings.Repeat(" ", padTo[0]-(len(strconv.Itoa(m))+len(strconv.Itoa(mMax))+1))
			}

			mLevel := util.QuantizeTens(m, mMax)
			return fmt.Sprintf(`<ansi fg="mana-%d">%d</ansi>/<ansi fg="mana-%d">%d</ansi>%s`, mLevel, m, mLevel, mMax, padding)
		},
		"colorpattern": func(s string, pattern string, colorizeStyle ...string) string {
			style := ``
			if len(colorizeStyle) > 0 {
				style = colorizeStyle[0]
			}

			if style == `words` {
				return colorpatterns.ApplyColorPattern(s, pattern, colorpatterns.Words)
			} else if style == `stretch` {
				return colorpatterns.ApplyColorPattern(s, pattern, colorpatterns.Stretch)
			}
			return colorpatterns.ApplyColorPattern(s, pattern)
		},
		"zodiac": func(year int) string {
			return gametime.GetZodiac(year)
		},
		"month": func(month int) string {
			return gametime.MonthName(month)
		},
		"map": makeMap,
		"t":   language.T,

		// Qualitative description helpers (Stage 18)
		"statQuality": func(value int) string {
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
		},
		// vitalQuality(current, max[, padTo]) — returns a colored qualitative label,
		// optionally right-padded to padTo visual characters (like healthStr does).
		"vitalQuality": func(current int, max int, padTo ...int) string {
			if max <= 0 {
				return "unknown"
			}
			pct := float64(current) / float64(max) * 100
			var label, color string
			switch {
			case pct <= 15:
				label, color = "critical", "red"
			case pct <= 35:
				label, color = "low", "red"
			case pct <= 60:
				label, color = "moderate", "yellow"
			case pct <= 85:
				label, color = "healthy", "green"
			default:
				label, color = "full", "green"
			}
			result := `<ansi fg="` + color + `">` + label + `</ansi>`
			if len(padTo) > 0 && padTo[0] > len(label) {
				result += strings.Repeat(" ", padTo[0]-len(label))
			}
			return result
		},
		"armorQuality": func(armor int) string {
			switch {
			case armor <= 0:
				return "none"
			case armor < 10:
				return "thin"
			case armor < 25:
				return "light"
			case armor < 50:
				return "medium"
			case armor < 80:
				return "heavy"
			default:
				return "solid"
			}
		},
		// toxicityQuality(bandName[, padTo]) — returns a colored label for the
		// toxicity band name returned by Character.ToxicityBandName(). Color
		// escalates with severity; optionally padded to padTo visual characters
		// (matching the vitalQuality convention used in the status template).
		"toxicityQuality": func(bandName string, padTo ...int) string {
			var color string
			switch bandName {
			case "sour":
				color = "black-bold" // feedback only, no penalty
			case "unsettled":
				color = "white" // feedback only, no penalty
			case "queasy":
				color = "yellow" // first band that carries a penalty
			case "sick":
				color = "red"
			case "critical":
				color = "red-bold"
			default: // "clear"
				color = "green"
			}
			result := `<ansi fg="` + color + `">` + bandName + `</ansi>`
			if len(padTo) > 0 && padTo[0] > len(bandName) {
				result += strings.Repeat(" ", padTo[0]-len(bandName))
			}
			return result
		},
		// encumbranceQuality(carried, capacity[, padTo]) returns a colored
		// burden word for how heavily the character is loaded, matching what
		// the `inventory` command prints. Both share characters.EncumbranceTier
		// so the two screens can never disagree about the same load.
		// Optionally padded to padTo visual characters, following the
		// vitalQuality/toxicityQuality convention used in the status template.
		"encumbranceQuality": func(carried, capacity float64, padTo ...int) string {
			label, color := characters.EncumbranceTier(carried, capacity)
			result := `<ansi fg="` + color + `">` + label + `</ansi>`
			if len(padTo) > 0 && padTo[0] > len(label) {
				result += strings.Repeat(" ", padTo[0]-len(label))
			}
			return result
		},
		// reservationQuality(bandName[, padTo]) returns a colored word for how
		// much of a pool is held in reserve by gear and companions, from
		// Character.ReservationBandName. Optionally padded to padTo visual
		// characters, following the vitalQuality / toxicityQuality /
		// encumbranceQuality convention used in the status template.
		//
		// Words only, never a number. The band ladder is keyed to the reservation
		// CAP, not to the pool, so every rung reports remaining headroom.
		// "near limit" is the last warning before "at limit", which means the
		// ceiling has been reached and nothing further can be added -- the one
		// thing a player actually needs to act on.
		"reservationQuality": func(bandName string, padTo ...int) string {
			var color string
			switch bandName {
			case "notable":
				color = "yellow"
			case "heavy":
				color = "red"
			case "near limit", "at limit":
				color = "red-bold"
			default: // "none", "slight", "modest"
				color = "green"
			}
			result := `<ansi fg="` + color + `">` + bandName + `</ansi>`
			if len(padTo) > 0 && padTo[0] > len(bandName) {
				result += strings.Repeat(" ", padTo[0]-len(bandName))
			}
			return result
		},
		// effectiveMitigation is a character's mitigation on one channel
		// ("physical", "magical" or "conviction") bent through that channel's
		// curve (#465), the value combat applies. The status sheet passes it
		// to mitigationQuality. An unknown name fails the render.
		"effectiveMitigation": func(c *characters.Character, channel string) (float64, error) {
			switch channel {
			case "physical":
				return combat.EffectiveMitigationFor(c, combat.ChannelPhysical), nil
			case "magical":
				return combat.EffectiveMitigationFor(c, combat.ChannelMagical), nil
			case "conviction":
				return combat.EffectiveMitigationFor(c, combat.ChannelConviction), nil
			}
			return 0, fmt.Errorf("effectiveMitigation: unknown channel %q", channel)
		},
		"mitigationQuality": mitigationQuality,
		"defenseQuality":    defenseQuality,
		"damageQuality": func(mult float64) string {
			switch {
			case mult < 0.3:
				return "negligible striking power"
			case mult < 0.6:
				return "feeble striking power"
			case mult < 1.0:
				return "light striking power"
			case mult < 1.5:
				return "moderate striking power"
			case mult < 2.5:
				return "strong striking power"
			case mult < 4.0:
				return "devastating striking power"
			default:
				return "legendary striking power"
			}
		},
		"spellDamageQuality": func(mult float64) string {
			switch {
			case mult < 0.5:
				return "negligible arcane resonance"
			case mult < 0.8:
				return "faint arcane resonance"
			case mult < 1.2:
				return "mild arcane resonance"
			case mult < 1.6:
				return "moderate arcane resonance"
			case mult < 2.5:
				return "strong arcane resonance"
			case mult < 4.0:
				return "intense arcane resonance"
			default:
				return "legendary arcane resonance"
			}
		},
		"statModDescription": func(statName string, value int) string {
			var prefix string
			switch {
			case value >= 20:
				prefix = "greatly bolsters"
			case value >= 10:
				prefix = "bolsters"
			case value >= 1:
				prefix = "slightly bolsters"
			case value <= -20:
				prefix = "greatly saps"
			case value <= -10:
				prefix = "saps"
			case value <= -1:
				prefix = "slightly saps"
			default:
				return ""
			}
			return prefix + " your " + statName
		},
		"mutationLevel":   mutations.LevelDescription,
		"skillRank":       skills.GetSkillRankDescription,
		"durationQuality": DurationQuality,
	}
)

// DurationQuality is how long rounds left lasts, in words and colour, as
// the `cooldowns` and conditions screens show a timer: a player never sees
// the raw round count (the player-copy rule). `skills extra` uses it too.
func DurationQuality(rounds int) string {
	switch {
	case rounds <= 0:
		return `<ansi fg="red">fading</ansi>`
	case rounds <= 3:
		return `<ansi fg="red">fading fast</ansi>`
	case rounds <= 10:
		return `<ansi fg="yellow">briefly active</ansi>`
	case rounds <= 30:
		return `<ansi fg="green">active</ansi>`
	default:
		return `<ansi fg="green">well established</ansi>`
	}
}

// wrapText wraps s to width columns with messaging.WrapAnsi, the game's one
// wrapper: it breaks only at spaces, so "(following)" never loses its
// bracket to the line above as splitstring's can, and it reopens a colour
// tag on each line. Every line after the first starts with prefix.
//
//	{{ wraptext .Description 36 "      " }}
func wrapText(s string, width int, prefix ...string) string {
	pad := ""
	if len(prefix) > 0 {
		pad = prefix[0]
	}
	return strings.ReplaceAll(messaging.WrapAnsi(s, width), "\n", "\n"+pad)
}

// wrapSpans wraps as wrapText does, but keeps each colour span (a name, an
// exit, a count) on one line, together with any text glued to it ("Name..."),
// unless that unit is wider than width; then the span's words wrap as usual.
// A span inside another is a unit too, so an item named inside a coloured
// sentence stays whole while the sentence wraps (plan 3b-3). A name is never
// split while it fits a line by itself (owner, 2026-11-10).
//
//	{{ wrapspans .TrackingString screenwidth }}
func wrapSpans(s string, width int, prefix ...string) string {
	const keep = "\x1f" // one column to WrapAnsi, and never a break
	return strings.ReplaceAll(wrapText(protectSpans(s, width, keep), width, prefix...), keep, " ")
}

// protectSpans is wrapSpans' first half: s with the spaces of every colour
// span that fits width (with any text glued to it) swapped for keep, and a
// break space added where glued text meets a span that must wrap. The help
// layout pass uses it too, so both treat a span the same way.
func protectSpans(s string, width int, keep string) string {
	runes := visibleRunes(s)
	// prot[i]: the space at rune i is protected. brkBefore[i] and brkAfter[i]:
	// a break space is added before or after rune i.
	prot := make([]bool, len(runes))
	brkBefore := make([]bool, len(runes))
	brkAfter := make([]bool, len(runes))
	type spanCore struct{ first, last int }
	cores := []spanCore{}
	// opens holds where each open span's text starts, innermost last, so a
	// span is seen as it closes: an inner one before the one around it.
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
		// The span's visible runes, then the unit: the span widened over
		// whatever is glued to it up to the nearest space each side.
		first, last := -1, -1
		for i, v := range runes {
			if v.start >= open && v.end <= t[0] {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		if first < 0 {
			continue
		}
		// A single space at either end of a span is a separator, not part of
		// the name: it stays a break point, or the span fuses with whatever
		// follows it ("Help for " + "master's component case"). A run of two
		// or more is a label's padding and stays protected, since a fold
		// would collapse it (the mail "NOTE:    " label).
		if first < last && runes[first].r == ' ' && runes[first+1].r != ' ' {
			first++
		}
		if last > first && runes[last].r == ' ' && runes[last-1].r != ' ' {
			last--
		}
		if first == last && runes[first].r == ' ' {
			continue
		}
		from, to := first, last
		for from > 0 && runes[from-1].r != ' ' {
			from--
		}
		for to < len(runes)-1 && runes[to+1].r != ' ' {
			to++
		}
		widthOf := func(a, b int) (w int) {
			for _, v := range runes[a:b] {
				w += util.VisibleWidth(string(v.r))
			}
			return w
		}
		lw, sw, rw := widthOf(from, first), widthOf(first, last+1), widthOf(last+1, to+1)
		if sw > width {
			continue
		}
		cores = append(cores, spanCore{first, last})
		for i := first; i <= last; i++ {
			if runes[i].r == ' ' {
				prot[i] = true
			}
		}
		if lw+sw+rw <= width {
			continue
		}
		// The unit is wider than a line, so a break must fall inside it,
		// and the span is protected: give the wrapper one where the glue
		// meets the span. A side gets its break when the span with that
		// side alone overflows (forced: the glue word and the span word
		// cannot share a line). When neither side overflows alone, only
		// one side gets it (the right, else the left): the glue word plus
		// the span, and the other glue, still overflow, so it is forced
		// too. Every added space is a forced break, never a stray space.
		needLeft := lw > 0 && lw+sw > width
		needRight := rw > 0 && sw+rw > width
		if !needLeft && !needRight {
			needRight = rw > 0
			needLeft = !needRight
		}
		if needLeft {
			brkAfter[first-1] = true
		}
		if needRight {
			brkBefore[last+1] = true
		}
	}

	// Each span above was checked against its own glue only. Spans written
	// back to back chain into one word, so a word that is still wider than a
	// line and holds protected spaces gets them handed back, then given again
	// to one span at a time, the widest first, while the word they make fits.
	runeW := func(i int) int { return util.VisibleWidth(string(runes[i].r)) }
	sep := func(i int) bool { return runes[i].r == ' ' && !prot[i] }
	wordAt := func(i int) (a, b int) {
		a, b = i, i
		for a > 0 && !sep(a-1) && !brkAfter[a-1] && !brkBefore[a] {
			a--
		}
		for b < len(runes)-1 && !sep(b+1) && !brkAfter[b] && !brkBefore[b+1] {
			b++
		}
		return a, b
	}
	wordW := func(a, b int) (w int) {
		for i := a; i <= b; i++ {
			w += runeW(i)
		}
		return w
	}
	for i := 0; i < len(runes); i++ {
		if sep(i) {
			continue
		}
		a, b := wordAt(i)
		i = b
		if wordW(a, b) <= width {
			continue
		}
		held := false
		for j := a; j <= b; j++ {
			if prot[j] {
				held, prot[j] = true, false
			}
		}
		if !held {
			continue
		}
		inside := []spanCore{}
		for _, c := range cores {
			if c.first >= a && c.last <= b {
				inside = append(inside, c)
			}
		}
		sort.SliceStable(inside, func(x, y int) bool {
			return inside[x].last-inside[x].first > inside[y].last-inside[y].first
		})
		for _, c := range inside {
			set := []int{}
			for j := c.first; j <= c.last; j++ {
				if runes[j].r == ' ' && !prot[j] {
					prot[j] = true
					set = append(set, j)
				}
			}
			if wa, wb := wordAt(c.first); wordW(wa, wb) > width {
				for _, j := range set {
					prot[j] = false
				}
			}
		}
	}

	what := make([]string, len(runes))
	for i, v := range runes {
		what[i] = s[v.start:v.end]
		if prot[i] {
			what[i] = keep
		}
		if brkBefore[i] {
			what[i] = " " + what[i]
		}
		if brkAfter[i] {
			what[i] += " "
		}
	}
	return rebuild(s, runes, what)
}

// wrapBreak wraps as wrapText does, but first breaks any word longer than
// width into pieces width wide, so no line runs past: a word is split only
// when it is longer than a whole line. Signs and runes use it, since a
// player writes them.
//
//	{{ wrapbreak $text (sub screenwidth 2) "  " }}
func wrapBreak(s string, width int, prefix ...string) string {
	runes := visibleRunes(s)
	what := make([]string, len(runes))
	cols := 0
	for i, v := range runes {
		what[i] = s[v.start:v.end]
		if v.r == ' ' || v.r == '\n' {
			cols = 0
			continue
		}
		w := util.VisibleWidth(what[i])
		if cols+w > width && cols > 0 {
			// Only inside a word wider than a line: such a word is the only
			// one that ever fills a whole line by itself.
			what[i] = " " + what[i]
			cols = 0
		}
		cols += w
	}
	return wrapText(rebuild(s, runes, what), width, prefix...)
}

// visibleRune is one rune of s that shows on screen, and its bytes in s.
type visibleRune struct {
	r          rune
	start, end int
}

// visibleRunes lists the runes of s outside its colour tags.
func visibleRunes(s string) []visibleRune {
	tags := ansiTag.FindAllStringIndex(s, -1)
	runes := []visibleRune{}
	for i, t := 0, 0; i < len(s); {
		if t < len(tags) && i == tags[t][0] {
			i = tags[t][1]
			t++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		runes = append(runes, visibleRune{r, i, i + size})
		i += size
	}
	return runes
}

// rebuild is s with each visible rune i replaced by what[i], its colour tags
// kept where they were.
func rebuild(s string, runes []visibleRune, what []string) string {
	var b strings.Builder
	at := 0
	for i, v := range runes {
		b.WriteString(s[at:v.start])
		b.WriteString(what[i])
		at = v.end
	}
	b.WriteString(s[at:])
	return b.String()
}

// centerLines centres s on a screen width columns wide: on one line when it
// fits with 2 columns clear each side; else one piece to a line when every
// piece fits with 1 column clear each side; else wrapped with
// messaging.WrapAnsi with 2 columns clear. A piece ends at a sentence end or
// at a run of two or more spaces, the author's own break (the wildfire
// alert spaces "!!!", its sentence and "!!!" apart); wrapped, a run is one
// space. Each line is padded on the left only. The room view's alerts use it
// (owner, 2026-11-10: an alert is its text centred between two full-width
// rules).
//
//	{{ centerlines $alertStr screenwidth }}
func centerLines(s string, width int) string {
	s = strings.TrimSpace(s)
	span := width - 4
	text, wrapAt := s, span
	if util.VisibleWidth(s) > span {
		text, wrapAt = respace(s, true), width-2
		for _, line := range strings.Split(text, "\n") {
			if util.VisibleWidth(line) > width-2 {
				text, wrapAt = respace(s, false), span
				break
			}
		}
	}
	lines := []string{}
	for _, line := range strings.Split(messaging.WrapAnsi(text, wrapAt), "\n") {
		w := util.VisibleWidth(line)
		if w == 0 {
			continue
		}
		lines = append(lines, strings.Repeat(" ", max(0, (width-w)/2))+line)
	}
	return strings.Join(lines, "\n")
}

// ansiTag matches one colour tag, opening or closing.
var ansiTag = regexp.MustCompile(`</?ansi[^>]*>`)

// respace rewrites each run of spaces in s as one space, or, when breaks is
// set, as a line break where the run ends a sentence (after ".", "!" or
// "?") or is two or more spaces long. Colour tags are passed over and kept,
// so a colour pattern that tags every letter and space (the wildfire's
// flame) is read by its letters; WrapAnsi then closes and reopens a colour
// that spans a break.
func respace(s string, breaks bool) string {
	runes := visibleRunes(s)
	// what[i] is what visible rune i becomes: itself, "" (dropped), " " or "\n".
	what := make([]string, len(runes))
	for i := 0; i < len(runes); i++ {
		if runes[i].r != ' ' {
			what[i] = s[runes[i].start:runes[i].end]
			continue
		}
		j := i
		for j < len(runes) && runes[j].r == ' ' {
			what[j] = ""
			j++
		}
		what[i] = " "
		if breaks && (j-i >= 2 || (i > 0 && strings.ContainsRune(".!?", runes[i-1].r))) {
			what[i] = "\n"
		}
		i = j - 1
	}
	return rebuild(s, runes, what)
}

// Usage:
//
//	{{ implode .items "," }}
//		OUTPUT: "item1,item2,item3"
func join(items []string, sep string) string {
	return strings.Join(items, sep)
}

// Usage:
//
//	{{ padLeft 10 }}
//		OUTPUT: "		  "
//	{{ padLeft 10 "hello" "-" }}
//		OUTPUT: "-----hello"
func padLeft(totalWidth int, stringArgs ...string) string {
	var stringIn string = ""
	var padString string = " "

	if len(stringArgs) > 0 {
		stringIn = stringArgs[0]
		if len(stringArgs) > 1 {
			padString = stringArgs[1]
		}
	}

	stringInWidth := runewidth.StringWidth(stringIn)

	if stringInWidth >= totalWidth {
		return stringIn
	}
	paddingLength := totalWidth - stringInWidth
	if paddingLength < 1 {
		return stringIn
	}
	return strings.Repeat(padString, paddingLength) + stringIn
}

// Usage:
//
//	{{ padRight 10 }}
//		OUTPUT: "		  "
//	{{ padRight 10 "hello" "-" }}
//		OUTPUT: "hello-----"
func padRight(totalWidth int, stringArgs ...string) string {
	var stringIn string = ""
	var padString string = " "

	if len(stringArgs) > 0 {
		stringIn = stringArgs[0]
		if len(stringArgs) > 1 {
			padString = stringArgs[1]
		}
	}

	stringInWidth := runewidth.StringWidth(stringIn)

	if stringInWidth >= totalWidth {
		return stringIn
	}
	paddingLength := totalWidth - stringInWidth
	if paddingLength < 1 {
		return stringIn
	}
	return stringIn + strings.Repeat(padString, paddingLength)
}

// mitigationQuality maps a mitigation fraction to a word band, "none" to
// "fortified" (from 70%). identify passes one item's raw points (divided by
// 100), so its top band is "fortified" however high the points go.
func mitigationQuality(pct float64) string {
	percent := pct * 100
	switch {
	case percent <= 0:
		return "none"
	case percent < 10:
		return "thin"
	case percent < 25:
		return "light"
	case percent < 40:
		return "moderate"
	case percent < 55:
		return "strong"
	case percent < 70:
		return "heavy"
	default:
		return "fortified"
	}
}

// defenseQualityImpregnable is where the status sheet's top Defenses band
// starts (#479, owner ruling): effective mitigation of 85% and up.
const defenseQualityImpregnable = 0.85

// defenseQuality bands a character's effective mitigation for the status
// sheet: mitigationQuality's bands plus "impregnable" from 85%, so builds
// between 70% and the 90% cap no longer all read "fortified". It is
// status-only; identify keeps mitigationQuality.
func defenseQuality(pct float64) string {
	if pct >= defenseQualityImpregnable {
		return "impregnable"
	}
	return mitigationQuality(pct)
}

func padRightX(input, padding string, length int) string {

	padLen := runewidth.StringWidth(padding)
	inputLen := runewidth.StringWidth(input)

	if length < inputLen {
		length = inputLen
	}

	// Calculate how many times the padding string should be repeated
	paddingRepeats := int(math.Ceil((float64(length) - float64(inputLen)) / float64(padLen)))
	finalPadLength := length - inputLen

	// Repeat the padding string to fill the gap
	pad := strings.Repeat(padding, paddingRepeats)

	if runewidth.StringWidth(pad)+inputLen > length {
		pad = string([]rune(pad)[:finalPadLength])
	}

	// Trim the padded string to the desired length

	// Concatenate the input and the padding
	result := input + pad

	return result
}

// Usage:
//
//	{{ pad 10 }}
//		OUTPUT: "          "
//	{{ pad 11 "hello" "-" }}
//		OUTPUT: "---hello---"
func pad(totalWidth int, stringArgs ...string) string {
	var stringIn string = ""
	var padString string = " "

	if len(stringArgs) > 0 {
		stringIn = stringArgs[0]
		if len(stringArgs) > 1 {
			padString = stringArgs[1]
		}
	}

	stringInWidth := runewidth.StringWidth(stringIn)

	if stringInWidth >= totalWidth {
		return stringIn
	}
	paddingLength := totalWidth - stringInWidth
	leftPad := paddingLength >> 1
	if leftPad < 1 {
		return stringIn
	}
	if paddingLength-leftPad < 1 {
		return strings.Repeat(padString, leftPad) + stringIn
	}
	return strings.Repeat(padString, leftPad) + stringIn + strings.Repeat(padString, paddingLength-leftPad)
}

func idsOtherThan(allIds []uint64, excludeId uint64) []uint64 {
	finalIds := make([]uint64, 0)
	for _, id := range allIds {
		if id != excludeId {
			finalIds = append(finalIds, id)
		}
	}
	return finalIds
}

func TNL(userId int) string {
	return `N/A`
}

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return (a * 100) / b
}

func numberFormat(num int) string {
	return util.FormatNumber(num)
}

func TplAnsiParse(input string) string {
	return AnsiParse(input)
}

func stringOr(a string, b string, padding ...int) string {
	str := a
	if str == "" {
		str = b
	}
	if len(padding) > 0 {
		str = padRight(padding[0], str)
	}
	return str
}

func formatDuration(seconds int) string {
	days := seconds / (24 * 3600)
	hours := (seconds % (24 * 3600)) / 3600
	minutes := (seconds % 3600) / 60
	seconds = seconds % 60

	result := ""
	if days > 0 {
		if days == 1 {
			result += strconv.Itoa(days) + " day "
		} else {
			result += strconv.Itoa(days) + " days "
		}
	}
	if hours > 0 || days > 0 { // Include hours if there are any days
		if hours == 1 {
			result += strconv.Itoa(hours) + " hour "
		} else {
			result += strconv.Itoa(hours) + " hours "
		}
	}
	if minutes > 0 || hours > 0 || days > 0 { // Include minutes if there are any hours or days
		if minutes == 1 {
			result += strconv.Itoa(minutes) + " minute "
		} else {
			result += strconv.Itoa(minutes) + " minutes "
		}
	}

	if seconds > 0 {
		if seconds == 1 {
			result += strconv.Itoa(seconds) + " second"
		} else {
			result += strconv.Itoa(seconds) + " seconds"
		}
	}

	if result == `` {
		result = `0 seconds`
	}

	return strings.TrimSpace(result)
}

func makeMap(kvs ...any) map[any]any {
	m := make(map[any]any)
	for i := 0; i < len(kvs)-1; i += 2 {
		m[kvs[i]] = kvs[i+1]
	}

	return m
}
