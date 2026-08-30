package users

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/util"
)

//
// This file contains vars/receiver methods for the UserRecord struct dealing with the prmopt.
// This just makes it easier to find and make adjustments to. It got annoying searching userrecord.go
// NOTE: NOT to be confused with an interactive question/answer prompt.
//
// Prompt Helpfile: templates/help/set-prompt.template
//

var (
	promptDefaultCompiled      = ``
	fightPromptDefaultCompiled = ``
	promptColorRegex           = regexp.MustCompile(`\{(\d*)(?::)?(\d*)?\}`)
	promptFindTagsRegex        = regexp.MustCompile(`\{[a-zA-Z%:\-]+\}`)
)

// canSeeInRoomFn reports whether a character can clearly see in the
// room they currently occupy (composes blindness, room lighting, and
// NightVision — see messaging.CanSeeClearly). Registered at boot from
// main.go, which can import rooms/ + messaging/ (users/ cannot import
// rooms/ — that would be an import cycle). nil = "can see" (boot- and
// test-safe default). Follows the same callback pattern as
// characters.SetUserUntargetableCheck.
//
// The fight prompt uses this to hide the combat target's identity,
// health, and position from blind / dark-room players, matching the
// combat-text darkness gating in NewRound_DoCombat.
var canSeeInRoomFn func(c *characters.Character) bool

// SetCanSeeInRoomCheck registers the prompt visibility check. Repeated
// registrations overwrite; pass nil to disable (tests).
func SetCanSeeInRoomCheck(fn func(c *characters.Character) bool) {
	canSeeInRoomFn = fn
}

// canSeeTargetForPrompt returns whether the player can currently see
// well enough for the fight prompt to reveal the combat target's
// identity / health / position. Defaults to true when no check is
// registered (boot, tests).
func (u *UserRecord) canSeeTargetForPrompt() bool {
	if canSeeInRoomFn == nil {
		return true
	}
	return canSeeInRoomFn(u.Character)
}

// RenderVitalBar is the exported version of renderVitalBar.
func RenderVitalBar(current, max, reserved int) string {
	return renderVitalBar(current, max, reserved)
}

// renderVitalBar returns a 10-block ANSI progress bar for a vital stat.
// Color breakpoints match the web client vitals window gradient:
//
//	>60% → green, >30% → gold, ≤30% → red
//
// THE TEN BLOCKS SPAN THE RAW MAXIMUM, AND THE RESERVED SHARE TAKES ITS OWN
// PROPORTIONAL SLICE AT THE RIGHT-HAND END. The bar's whole length still means
// "your whole pool"; the band fenced off at the end means "this part is spoken
// for and is not coming back while it is spoken for"; and the blocks to the
// left of the band are the reachable pool, filled from current / (max -
// reserved). That is the same three-region model webclient-pure.html draws
// (buildVitalsSeg: a usable region sized to the unreserved fraction, a dark
// drain masking its spent part, a crosshatched reserved block after it), so
// the prompt and the web Vitals panel now say the same thing about the same
// character at the same moment.
//
// TWO BUGS ARE BEING HELD OFF AT ONCE HERE, AND THEY PULL IN OPPOSITE
// DIRECTIONS. Both were found in play, a day apart.
//
//	Over-report (2026-08-15). The bar once scaled its filled count against the
//	RAW max and drew the reserved share as a ▓ band. internal/util's ASCII
//	downgrade table maps BOTH '█' and '▓' to '#' (unicodeToAscii in util.go),
//	so at 272 of 440 health with 176 reserved a player on an ASCII client saw
//	########## and read a full bar while bleeding, and at 101 of 440 saw seven
//	of ten segments lit at 23% health. A player could bleed out reading a
//	healthy gauge.
//
//	Under-report (2026-08-16). The first fix dropped the band entirely and
//	measured the ten blocks against the reachable pool. That is honest about
//	damage but silent about reservation: a player fielding two flesh golems saw
//	three completely full bars in the prompt and a clearly banded set in the web
//	Vitals panel, with nothing on screen to explain why companions were being
//	refused. Correct under its own model, and disagreeing with the other
//	surface.
//
// The glyph table is what makes having both possible. '█' → '#', '░' → '.',
// and '▒' → ':' are three DISTINCT ASCII characters, so the three regions stay
// three regions after the downgrade. '▓' is the one shade that must never be
// used for this, because it collapses onto filled. The ASCII split is pinned by
// a test that runs the real util.ConvertToAscii, not a local approximation,
// because a silent collapse in that table is exactly how the over-report shipped.
//
// ROUNDING, AND THE TENSION IN IT:
//
//   - The FILLED count floors, so ten filled blocks means exactly full and
//     nothing else. Rounding would show a full bar from 95% upward, which is a
//     smaller version of the over-report.
//   - The filled count is measured against the USABLE blocks, not against ten,
//     so widening or narrowing the band by a rounding step can never steal the
//     last filled block from someone at genuinely full reachable health: at
//     current == effectiveMax, filled == usable exactly, whatever the band did.
//   - The BAND rounds to nearest, then is clamped to at least one block whenever
//     reserved > 0, so a real reservation is never a zero-width band. A tiny
//     reservation therefore reads slightly larger than it is. That is the
//     deliberate trade: a band that is a shade too wide costs nothing, a band
//     that vanishes is the bug this function exists to fix.
//   - The band is also capped at nine blocks so the gauge itself never
//     disappears. A near-totally reserved character keeps one block that still
//     empties as they take damage.
//
// Color stays measured against the reachable pool, matching the status sheet's
// vitals row (characters.EffectivePoolMaxNamed) so the two agree on how hurt
// the player is.
func renderVitalBar(current, max, reserved int) string {
	if max <= 0 {
		max = 1
	}
	if current < 0 {
		current = 0
	}
	if reserved < 0 {
		reserved = 0
	}

	effectiveMax := max - reserved
	if effectiveMax < 1 {
		effectiveMax = 1
	}
	if current > effectiveMax {
		current = effectiveMax
	}

	// The reserved band: proportional slice of the whole bar, never zero-width
	// when a real reservation exists, never wide enough to swallow the gauge.
	reservedBlocks := 0
	if reserved > 0 {
		reservedBlocks = int(math.Round(float64(reserved) / float64(max) * 10.0))
		if reservedBlocks < 1 {
			reservedBlocks = 1
		}
		if reservedBlocks > 9 {
			reservedBlocks = 9
		}
	}
	usableBlocks := 10 - reservedBlocks

	filledBlocks := int(math.Floor(float64(current) / float64(effectiveMax) * float64(usableBlocks)))
	if filledBlocks > usableBlocks {
		filledBlocks = usableBlocks
	}
	if filledBlocks < 0 {
		filledBlocks = 0
	}
	emptyBlocks := usableBlocks - filledBlocks

	// Color based on current vs effective max. 256-color indices chosen to match
	// the web vitals gradient (Material colors) as closely as the cube allows —
	// softer than pure 82/226/196: green ≈ #4caf50, gold ≈ #ffeb3b, red ≈ #f44336.
	effectivePct := float64(current) / float64(effectiveMax) * 100.0
	var barColor string
	switch {
	case effectivePct > 60:
		barColor = "71" // green ≈ vitals #4caf50
	case effectivePct > 30:
		barColor = "221" // gold/yellow ≈ vitals #ffeb3b
	default:
		barColor = "203" // red ≈ vitals #f44336
	}

	result := ``
	if filledBlocks > 0 {
		result += fmt.Sprintf(`<ansi fg="%s">%s</ansi>`,
			barColor,
			strings.Repeat("█", filledBlocks))
	}

	if emptyBlocks > 0 {
		result += fmt.Sprintf(`<ansi fg="238">%s</ansi>`,
			strings.Repeat("░", emptyBlocks))
	}

	if reservedBlocks > 0 {
		// Mid grey, deliberately lighter than the near-black empty blocks, so the
		// band reads as a distinct fenced-off region rather than more emptiness.
		// This mirrors the web panel, where the reserved block carries a lighter
		// crosshatch over a dark ground.
		result += fmt.Sprintf(`<ansi fg="244">%s</ansi>`,
			strings.Repeat("▒", reservedBlocks))
	}

	return result
}

// getPromptToggle returns the toggle state for a fight prompt element.
// Returns true (on) by default when not explicitly set.
func (u *UserRecord) getPromptToggle(key string) bool {
	val := u.GetConfigOption(`fprompt-tog-` + key)
	if val == nil {
		return true
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return true
}

// targetHealthDesc returns a descriptive label and color class for a target's health percentage.
func targetHealthDesc(health, maxHealth int) (string, string) {
	if maxHealth <= 0 {
		maxHealth = 1
	}
	if health <= 0 {
		return "dead", "health-0"
	}
	pct := int(float64(health) / float64(maxHealth) * 100.0)
	switch {
	case pct >= 80:
		return "healthy", "health-100"
	case pct >= 60:
		return "bruised", "health-60"
	case pct >= 40:
		return "wounded", "health-40"
	case pct >= 20:
		return "badly wounded", "health-20"
	default:
		return "near death", "health-10"
	}
}

// BuildFightPromptTemplate assembles the fight prompt template string from enabled toggles.
// It is exported so that usercommands/set.go can rebuild and cache it when a toggle changes.
func (u *UserRecord) BuildFightPromptTemplate() string {
	var b strings.Builder
	b.WriteString(`{8}[{t}`)
	if u.getPromptToggle(`bars`) {
		b.WriteString(` {255}HP:{hpbar} SP:{stbar} CP:{cvbar}`)
	}
	if u.getPromptToggle(`pos`) {
		b.WriteString(`{pos}`)
	}
	b.WriteString(`{8}]`)
	if u.getPromptToggle(`target`) {
		b.WriteString(` {255}» {target}`)
	}
	showTargetHealth := u.getPromptToggle(`targethealth`)
	showTargetPos := u.getPromptToggle(`targetpos`)
	if showTargetPos || showTargetHealth {
		b.WriteString(`{8}[`)
		if showTargetPos {
			b.WriteString(`{targetpos}`)
		}
		if showTargetPos && showTargetHealth {
			b.WriteString(`{8}|`)
		}
		if showTargetHealth {
			b.WriteString(`{targethealth}`)
		}
		b.WriteString(`{8}]`)
	}
	if u.getPromptToggle(`tank`) {
		b.WriteString(`{tank}`)
	}
	b.WriteString(`{casting}`)
	b.WriteString(`{239}{h}{8}:`)
	return b.String()
}

func (u *UserRecord) GetCommandPrompt() string {

	promptOut := ``

	if u.activePrompt != nil {

		if activeQuestion := u.activePrompt.GetNextQuestion(); activeQuestion != nil {
			promptOut = activeQuestion.String()
		}
	}

	goAhead := ``
	if connections.GetClientSettings(u.ConnectionId()).SendTelnetGoAhead {
		goAhead = term.TelnetGoAhead.String()
	}

	if len(promptOut) == 0 {

		if promptDefaultCompiled == `` {
			promptDefaultCompiled = util.ConvertColorShortTags(configs.GetTextFormatsConfig().Prompt.String())
		}

		var customPrompt any = nil
		// Use IsInCombat() (CombatPhase-aware) instead of the raw
		// Aggro field — the prompt should render the fight format
		// whenever the player is engaged, even if the legacy Aggro
		// pointer happens to be transiently nil between rounds.
		// IsInCombat falls back to Aggro for legacy code paths that
		// haven't wired CombatPhase yet.
		var inCombat bool = u.Character.IsInCombat()

		if inCombat {
			customPrompt = u.GetConfigOption(`fprompt-compiled`)
			if customPrompt == nil {
				// Toggle-driven cache (kept current by set.go when a toggle changes)
				cached := u.GetConfigOption(`fprompt-default-compiled`)
				if cached == nil {
					// No toggle customizations — use the server config default
					if fightPromptDefaultCompiled == `` {
						fightPromptDefaultCompiled = util.ConvertColorShortTags(configs.GetTextFormatsConfig().FightPrompt.String())
					}
					customPrompt = fightPromptDefaultCompiled
				} else {
					customPrompt = cached
				}
			}
		}

		// No other custom prompts? try the default setting
		if customPrompt == nil {
			customPrompt = u.GetConfigOption(`prompt-compiled`)
		}

		if customPrompt != nil {
			if ansiPrompt, ok := customPrompt.(string); ok {
				promptOut = u.ProcessPromptString(ansiPrompt)
			}
		}

		// Still nothing? Default to ... default
		if len(promptOut) == 0 {
			promptOut = u.ProcessPromptString(promptDefaultCompiled)
		}

	}

	unsent, suggested := u.GetUnsentText()
	if len(suggested) > 0 {
		suggested = `<ansi fg="suggested-text">` + suggested + `</ansi>`
	}
	return term.AnsiMoveCursorColumn.String() + term.AnsiEraseLine.String() + promptOut + unsent + suggested + goAhead

}

func (u *UserRecord) ProcessPromptString(promptStr string) string {

	promptOut := strings.Builder{}

	var hpPct, mpPct int = -1, -1
	var hpClass, mpClass string

	// Lazily resolve whether the player can see their target (blindness /
	// dark room without special vision). Computed at most once per render
	// and only when a target-derived token is actually present, so the
	// out-of-combat default prompt pays no room-lookup cost.
	canSeeChecked := false
	canSeeTarget := true
	sees := func() bool {
		if !canSeeChecked {
			canSeeTarget = u.canSeeTargetForPrompt()
			canSeeChecked = true
		}
		return canSeeTarget
	}

	promptLen := len(promptStr)
	tagStartPos := -1

	for i := 0; i < promptLen; i++ {
		if promptStr[i] == '{' {
			tagStartPos = i
			continue
		}
		if promptStr[i] == '}' {

			switch promptStr[tagStartPos : i+1] {

			case `{\n}`:
				promptOut.WriteString("\n")

			case `{hp}`:
				if len(hpClass) == 0 {
					hpClass = fmt.Sprintf(`health-%d`, util.QuantizeTens(u.Character.DisplayHealth(), u.Character.HealthMax.Value))
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, hpClass, u.Character.DisplayHealth()))

			case `{hp:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.DisplayHealth()))
			case `{HP}`:
				if len(hpClass) == 0 {
					hpClass = fmt.Sprintf(`health-%d`, util.QuantizeTens(u.Character.DisplayHealth(), u.Character.HealthMax.Value))
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, hpClass, u.Character.HealthMax.Value))
			case `{HP:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.HealthMax.Value))
			case `{hp%}`:
				if hpPct == -1 {
					hpPct = int(math.Floor(float64(u.Character.DisplayHealth()) / float64(u.Character.HealthMax.Value) * 100))
				}
				if len(hpClass) == 0 {
					hpClass = fmt.Sprintf(`health-%d`, util.QuantizeTens(u.Character.DisplayHealth(), u.Character.HealthMax.Value))
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d%%</ansi>`, hpClass, hpPct))

			case `{hp%:-}`:
				if hpPct == -1 {
					hpPct = int(math.Floor(float64(u.Character.DisplayHealth()) / float64(u.Character.HealthMax.Value) * 100))
				}
				promptOut.WriteString(strconv.Itoa(hpPct))
				promptOut.WriteString(`%`)

			case `{mp}`:
				if len(mpClass) == 0 {
					mpClass = fmt.Sprintf(`mana-%d`, util.QuantizeTens(u.Character.Conviction, u.Character.ConvictionMax.Value))
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, mpClass, u.Character.Conviction))

			case `{mp:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.Conviction))

			case `{MP}`:
				if len(mpClass) == 0 {
					mpClass = fmt.Sprintf(`mana-%d`, util.QuantizeTens(u.Character.Conviction, u.Character.ConvictionMax.Value))
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, mpClass, u.Character.ConvictionMax.Value))

			case `{MP:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.ConvictionMax.Value))

			case `{mp%}`:
				if mpPct == -1 {
					mpPct = int(math.Floor(float64(u.Character.Conviction) / float64(u.Character.ConvictionMax.Value) * 100))
				}
				if len(mpClass) == 0 {
					mpClass = fmt.Sprintf(`mana-%d`, util.QuantizeTens(u.Character.Conviction, u.Character.ConvictionMax.Value))
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d%%</ansi>`, mpClass, mpPct))

			case `{mp%:-}`:
				if mpPct == -1 {
					mpPct = int(math.Floor(float64(u.Character.Conviction) / float64(u.Character.ConvictionMax.Value) * 100))
				}
				promptOut.WriteString(strconv.Itoa(mpPct))
				promptOut.WriteString(`%`)

			case `{st}`:
				stClass := fmt.Sprintf(`health-%d`, util.QuantizeTens(u.Character.Stamina, u.Character.StaminaMax.Value))
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, stClass, u.Character.Stamina))

			case `{st:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.Stamina))

			case `{ST}`:
				stClass := fmt.Sprintf(`health-%d`, util.QuantizeTens(u.Character.Stamina, u.Character.StaminaMax.Value))
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, stClass, u.Character.StaminaMax.Value))

			case `{ST:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.StaminaMax.Value))

			case `{cv}`:
				cvClass := fmt.Sprintf(`mana-%d`, util.QuantizeTens(u.Character.Conviction, u.Character.ConvictionMax.Value))
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, cvClass, u.Character.Conviction))

			case `{cv:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.Conviction))

			case `{CV}`:
				cvClass := fmt.Sprintf(`mana-%d`, util.QuantizeTens(u.Character.Conviction, u.Character.ConvictionMax.Value))
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d</ansi>`, cvClass, u.Character.ConvictionMax.Value))

			case `{CV:-}`:
				promptOut.WriteString(strconv.Itoa(u.Character.ConvictionMax.Value))

			case `{hpbar}`:
				promptOut.WriteString(renderVitalBar(u.Character.Health, u.Character.HealthMax.Value,
					u.Character.GetPoolReservation("health", u.Character.HealthMax.Value)))

			case `{stbar}`:
				promptOut.WriteString(renderVitalBar(u.Character.Stamina, u.Character.StaminaMax.Value,
					u.Character.GetPoolReservation("stamina", u.Character.StaminaMax.Value)))

			case `{cvbar}`:
				promptOut.WriteString(renderVitalBar(u.Character.Conviction, u.Character.ConvictionMax.Value,
					u.Character.GetPoolReservation("conviction", u.Character.ConvictionMax.Value)))

			case `{pet_hp}`:
				if len(u.Character.Companions) > 0 && u.Character.Companions[0].InstanceId > 0 {
					if pet := mobs.GetInstance(u.Character.Companions[0].InstanceId); pet != nil && pet.Character.HealthMax.Value > 0 {
						petClass := fmt.Sprintf(`health-%d`, util.QuantizeTens(pet.Character.DisplayHealth(), pet.Character.HealthMax.Value))
						pct := int(math.Floor(float64(pet.Character.DisplayHealth()) / float64(pet.Character.HealthMax.Value) * 100))
						promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d%%</ansi>`, petClass, pct))
					}
				}

			case `{pet_sp}`:
				if len(u.Character.Companions) > 0 && u.Character.Companions[0].InstanceId > 0 {
					if pet := mobs.GetInstance(u.Character.Companions[0].InstanceId); pet != nil && pet.Character.StaminaMax.Value > 0 {
						petClass := fmt.Sprintf(`health-%d`, util.QuantizeTens(pet.Character.Stamina, pet.Character.StaminaMax.Value))
						pct := int(math.Floor(float64(pet.Character.Stamina) / float64(pet.Character.StaminaMax.Value) * 100))
						promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d%%</ansi>`, petClass, pct))
					}
				}

			case `{pet_cp}`:
				if len(u.Character.Companions) > 0 && u.Character.Companions[0].InstanceId > 0 {
					if pet := mobs.GetInstance(u.Character.Companions[0].InstanceId); pet != nil && pet.Character.ConvictionMax.Value > 0 {
						petClass := fmt.Sprintf(`mana-%d`, util.QuantizeTens(pet.Character.Conviction, pet.Character.ConvictionMax.Value))
						pct := int(math.Floor(float64(pet.Character.Conviction) / float64(pet.Character.ConvictionMax.Value) * 100))
						promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%d%%</ansi>`, petClass, pct))
					}
				}

			case `{target}`:
				// Source target from CombatPhase (canonical) with
				// fallback to legacy Aggro inside CurrentCombatTarget.
				// Robust against transient Aggro=nil between rounds.
				//
				// Hide the target's identity when the player can't see it
				// (blind / dark room without special vision) — the name is
				// info they don't have. Matches combat-text darkness gating.
				if !sees() {
					promptOut.WriteString(`<ansi fg="mobname">an unseen foe</ansi>`)
				} else {
					tRef := u.Character.CurrentCombatTarget()
					if tRef.MobInstanceId > 0 {
						if m := mobs.GetInstance(tRef.MobInstanceId); m != nil {
							promptOut.WriteString(fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, m.Character.Name))
						}
					} else if tRef.UserId > 0 {
						if target := GetByUserId(tRef.UserId); target != nil {
							promptOut.WriteString(fmt.Sprintf(`<ansi fg="username">%s</ansi>`, target.Character.Name))
						}
					}
				}

			case `{targethealth}`:
				if !sees() {
					break // can't see the target — no health read
				}
				tRef := u.Character.CurrentCombatTarget()
				var tHealth, tMax int
				if tRef.MobInstanceId > 0 {
					if m := mobs.GetInstance(tRef.MobInstanceId); m != nil {
						tHealth, tMax = m.Character.Health, m.Character.HealthMax.Value
					}
				} else if tRef.UserId > 0 {
					if target := GetByUserId(tRef.UserId); target != nil {
						tHealth, tMax = target.Character.Health, target.Character.HealthMax.Value
					}
				}
				if tMax > 0 {
					desc, color := targetHealthDesc(tHealth, tMax)
					promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, color, desc))
				}

			case `{targetpos}`:
				if !sees() {
					break // can't see the target — no position read
				}
				// FSM-driven: same color/abbrev helpers used by {pos}.
				tRef := u.Character.CurrentCombatTarget()
				var tPos position.State
				var tPosFound bool
				if tRef.MobInstanceId > 0 {
					if m := mobs.GetInstance(tRef.MobInstanceId); m != nil && m.Character.Position != nil {
						tPos = m.Character.Position.State()
						tPosFound = true
					}
				} else if tRef.UserId > 0 {
					if target := GetByUserId(tRef.UserId); target != nil && target.Character.Position != nil {
						tPos = target.Character.Position.State()
						tPosFound = true
					}
				}
				if tPosFound && tPos != position.Standing {
					promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%s</ansi>`,
						positionPromptColor(tPos), positionPromptAbbrev(tPos)))
				}

			case `{tank}`:
				if !sees() {
					break // can't see the target (or who it's fighting)
				}
				tRef := u.Character.CurrentCombatTarget()
				if tRef.MobInstanceId > 0 {
					if m := mobs.GetInstance(tRef.MobInstanceId); m != nil {
						mRef := m.Character.CurrentCombatTarget()
						if mRef.UserId > 0 && mRef.UserId != u.UserId {
							if tankUser := GetByUserId(mRef.UserId); tankUser != nil {
								tankBar := renderVitalBar(
									tankUser.Character.Health,
									tankUser.Character.HealthMax.Value,
									tankUser.Character.GetPoolReservation("health",
										tankUser.Character.HealthMax.Value))
								promptOut.WriteString(fmt.Sprintf(
									` <ansi fg="255">T:</ansi><ansi fg="username">%s</ansi> <ansi fg="255">Thp:</ansi>%s`,
									tankUser.Character.Name, tankBar))
							}
						}
					}
				}

			case `{ap}`:
				promptOut.WriteString(strconv.Itoa(u.Character.ActionPoints))

			case `{h}`:
				hiddenFlag := ``
				if u.Character.IsHidden() {
					hiddenFlag = `H`
				}
				promptOut.WriteString(hiddenFlag)

			case `{pos}`:
				// Combat position prompt. Chunk 4b R6: FSM-driven (14 states
				// — Standing / Prone / Supine / 11 grapples). Hidden when
				// Standing to keep the prompt short.
				if !u.Character.IsStanding() && u.Character.Position != nil {
					s := u.Character.Position.State()
					promptOut.WriteString(fmt.Sprintf(
						`<ansi fg="%s">%s</ansi>`,
						positionPromptColor(s),
						positionPromptAbbrev(s),
					))
				}

			case `{casting}`:
				if u.Character.Activity != nil {
					if cs, ok := u.Character.Activity.CastingData(); ok {
						spellName := cs.SpellId
						if sd := spells.GetSpell(cs.SpellId); sd != nil {
							spellName = sd.Name
						}
						promptOut.WriteString(fmt.Sprintf(
							`<ansi fg="cyan"> [%s %d/%d]</ansi>`,
							spellName, cs.FoldsAccumulated, cs.FoldsNeeded))
					}
				}

			case `{g}`:
				promptOut.WriteString(strconv.Itoa(u.Character.Gold))

			case `{enc}`:
				weight := u.Character.GetCarriedWeight()
				capacity := u.Character.CarryCapacity()
				var encLabel, encColor string
				if capacity <= 0 {
					encLabel, encColor = "crushed", "magenta-bold"
				} else {
					ratio := weight / capacity
					switch {
					case ratio <= 0.25:
						encLabel, encColor = "light", "green"
					case ratio <= 0.50:
						encLabel, encColor = "moderate", "yellow"
					case ratio <= 0.75:
						encLabel, encColor = "heavy", "red"
					case ratio <= 1.00:
						encLabel, encColor = "overburdened", "red-bold"
					default:
						encLabel, encColor = "crushed", "magenta-bold"
					}
				}
				promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, encColor, encLabel))

			case `{tox}`:
				// Toxicity band tier word, colored by severity. Omitted
				// when clear so it stays silent in default prompts.
				band := u.Character.ToxicityBand()
				if band > 0 {
					bandName := u.Character.ToxicityBandName()
					var toxColor string
					switch band {
					case 1:
						toxColor = "yellow"
					case 2:
						toxColor = "red"
					default:
						toxColor = "red-bold"
					}
					promptOut.WriteString(fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, toxColor, bandName))
				}

			case `{i}`:
				promptOut.WriteString(strconv.Itoa(len(u.Character.Items)))

			case `{I}`:
				promptOut.WriteString(strconv.Itoa(int(u.Character.CarryCapacity())))

			case `{w}`:
				if u.Character.CombatPhase != nil {
					promptOut.WriteString(strconv.Itoa(u.Character.RoundsWaiting()))
				} else {
					promptOut.WriteString(`0`)
				}

			case `{t}`:
				gd := gametime.GetDate()
				promptOut.WriteString(gd.String(true))

			case `{T}`:
				gd := gametime.GetDate()
				promptOut.WriteString(gd.String())

			}
			tagStartPos = -1
			continue
		}

		if tagStartPos == -1 {
			promptOut.WriteByte(promptStr[i])
		}
	}

	return promptOut.String()
}

// positionPromptColor returns the ANSI color name for the {pos}
// prompt token, replacing the legacy
// CombatPosition.GetPositionColor (sunset in chunk 4b S5).
func positionPromptColor(s position.State) string {
	switch s {
	case position.Standing:
		return "white"
	case position.Prone, position.Supine:
		return "yellow"
	case position.Clinch, position.BackStanding:
		return "orange"
	default: // all 9 ground-grapple states
		return "red"
	}
}

// positionPromptAbbrev keeps the {pos} token narrow. State names
// longer than ~5 chars get an abbreviated form; the full names are
// available via Position.State().String() for any caller that
// needs them.
func positionPromptAbbrev(s position.State) string {
	switch s {
	case position.BackStanding:
		return "B.Std"
	case position.BackGround:
		return "B.Gnd"
	case position.SideControl:
		return "SC"
	case position.KneeOnBelly:
		return "KOB"
	case position.NorthSouth:
		return "N-S"
	case position.HalfGuard:
		return "H.Gd"
	default:
		return s.String()
	}
}
