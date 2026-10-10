package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The discriminator this guard rests on, pinned so the compiler checks it
// rather than a comment asserting it. Every event-path add ends in a source
// string; no direct, silent add does. If someone gives Character.AddCondition
// a source parameter, or drops one from UserRecord.AddCondition, these stop
// compiling and the guard is fixed deliberately instead of quietly going
// blind.
var (
	_ func(int, bool) error                 = (*characters.Character)(nil).AddCondition
	_ func(int, float64) error              = (*characters.Character)(nil).AddConditionScaled
	_ func(int, int, float64, string) error = (*characters.Character)(nil).AddConditionMagnitude
	_ func(int, string)                     = (*users.UserRecord)(nil).AddCondition
	_ func(int, float64, string)            = (*users.UserRecord)(nil).AddConditionScaled
	_ func(int, int, float64, string)       = (*users.UserRecord)(nil).AddConditionMagnitude
	_ func(int, string)                     = (*mobs.Mob)(nil).AddCondition
	_ func(int, int, float64, string)       = (*mobs.Mob)(nil).AddConditionMagnitude
	_ func(int, float64, string)            = (*users.UserRecord)(nil).AddConditionTickScaled
	_ func(int, float64, string)            = (*mobs.Mob)(nil).AddConditionTickScaled
	_ func(int, string)                     = (*actions.UserActor)(nil).AddCondition
	_ func(int, string)                     = (*actions.MobActor)(nil).AddCondition
)

// Slice C delivery path: a player condition must travel the events.Condition
// event, so the Condition_ApplyConditions hook runs and narrates the start
// through conditions.ConditionSpec.StartUserNotice. A direct character-level
// add (Character.AddCondition / Character.AddConditionScaled, or
// Conditions.AddCondition beneath them) applies the condition in place and
// queues nothing, so the hook never runs and the holder reads no line at
// all. That is not a theoretical gap: a playtest drank a Purging Draught and
// took fifty rounds of Purging Weakness in silence, and the same defect was
// hiding in the quest reward path, the Bloom crash and withdrawal, the
// arrest, and sleep.
//
// How the two are told apart WITHOUT relying on receiver names: every
// event-path method ends in a source string, and no direct one does. The
// signatures are pinned by the compiler above, not asserted here.
//
//	users.UserRecord.AddCondition(conditionId int, source string)
//	users.UserRecord.AddConditionScaled(conditionId int, durationMult float64, source string)
//	users.UserRecord.AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
//	mobs.Mob.AddCondition(conditionId int, source string)
//	mobs.Mob.AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
//	users.UserRecord.AddConditionTickScaled / mobs.Mob.AddConditionTickScaled(conditionId int, scale float64, source string)
//	actions.Actor.AddCondition(conditionId int, source string)   // UserActor + MobActor
//
// AddConditionTickScaled has no silent character-level twin, so the call
// pattern below does not scan it: every call to it queues the event.
//
//	characters.Character.AddCondition(conditionId int, isPermanent bool)      // silent
//	characters.Character.AddConditionScaled(conditionId int, durationMult float64) // silent
//	characters.Character.AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) // silent
//	conditions.Conditions.AddCondition / AddConditionScaled                  // silent
//
// A call passes when its ARITY and its final argument both fit the event-path
// shape: two arguments for AddCondition, three for AddConditionScaled, with a
// last argument that could be a string (a literal, or an identifier or
// selector, so a variable source is never mistaken for a silent add). The
// bool literals are excluded from that, since `false` is an identifier to a
// text scanner and is exactly what the silent Character.AddCondition takes.
// Checking arity as well as the argument is what keeps the two-argument
// Character.AddConditionScaled(id, mult) caught, whose bare float variable
// would otherwise read as a source.
//
// Every other call is a silent add and needs a reason in the allowlist
// below, keyed "file|line". Legitimate reasons: the holder is a mob (no
// client to read a line), the condition is flagged silent-start and its
// applier narrates the moment itself, the condition has to be in place
// before the function returns, or the condition is secret. When a
// legitimate direct add moves, update its line number here; when a new one
// appears, either route it through the event path or record why it cannot
// be.
//
// Character.AddConditionMagnitude, UserRecord.AddConditionMagnitude and
// Mob.AddConditionMagnitude share one four-argument shape ending in a source
// string, so arity cannot tell the silent character door from the
// event-queuing user and mob doors apart the way it does for
// AddCondition/AddConditionScaled; isEventPathCall reads every
// AddConditionMagnitude call as a direct add, the safe reading, and every
// former-condition producer site OUTSIDE the primitive packages is
// allowlisted by hand with a reason worded like "former combat condition
// (warcry/rally): silent-start record, the shout narrates; must apply
// synchronously so the fan-out and the same-round combat read it". The
// allowlist below is a census of producers outside primitivePackages, not of
// every producer: primitivePackages exempts internal/characters wholesale
// (it defines the primitive being called), so the three former-condition
// producers inside it, the prone-recovery
// AddConditionMagnitude(conditions.ConditionIdRecovering, ...) calls in
// internal/characters/skills.go (lines 76, 99, 103), never reach this walk
// and carry no allowlist entry. The allowlist also records EVENT-door
// magnitude calls, which narrate correctly and are listed only because
// arity cannot tell them from the silent door: see the light-spell entry
// for internal/hooks/light_spell.go.
var conditionApplyPathAllowlist = map[string]string{
	// ── The sanctioned consumer of the event (re-keyed messaging M6 slice 1,
	// when the family rival read landed above the adds) ──────────────────
	"internal/hooks/Condition_ApplyConditions.go|122": "this IS the hook the event feeds; it is where every routed condition is finally applied",
	"internal/hooks/Condition_ApplyConditions.go|124": "this IS the hook the event feeds; it is where every routed condition is finally applied",
	"internal/hooks/Condition_ApplyConditions.go|126": "this IS the hook the event feeds; it is where every routed condition is finally applied",

	// ── silent-start conditions whose applier narrates the moment itself ────
	"internal/actions/combat_throttle.go|149": "condition 89 is silent-start; the throttle move narrates the choke as it lands and must apply it in the same tick",
	"internal/actions/sleep.go|60":            "condition 15 is silent-start; Sleep reads the Sleeping flag back for its own idempotence check, so it sends the start line itself",

	// ── former combat conditions: warcry and rally are now one record each,
	// applied via AddConditionMagnitude (conditions.ConditionIdWarcry /
	// conditions.ConditionIdRally), both silent-start; the shout narrates
	// itself and must apply synchronously so the fan-out and the same-round
	// combat read it ────────────────────────────────────────────────────────
	"internal/actions/combat_warcry.go|121": "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/actions/combat_rally.go|119":  "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/usercommands/warcry.go|57":    "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/usercommands/warcry.go|90":    "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/usercommands/warcry.go|118":   "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/usercommands/rally.go|57":     "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/usercommands/rally.go|86":     "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",
	"internal/usercommands/rally.go|114":    "former combat condition (warcry/rally): silent-start record, the shout narrates; must apply synchronously so the fan-out and the same-round combat read it",

	// ── former combat condition: enchant withdrawal ─────────────────────────
	"internal/usercommands/skill.disenchant.go|71": "former combat condition (withdrawal): the disenchant command narrates; must apply synchronously so Validate clamps the pool now",

	// ── mob holders: no client, so no line could reach anyone ───────────────
	"internal/usercommands/character.go|423":         "the holder is a MOB (m.Character), and condition 99 is a perma-gear pin, not something a player reads",
	"internal/behaviortree/actions_item_proc.go|419": "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
	"internal/hooks/manifester_companions.go|40":     "the holder is a MOB (a summoned companion), not a player",

	// ── secret conditions: silence is the authored intent ───────────────────
	"internal/hooks/Life_Cascades.go|131": "condition 81 Respawn Grace is secret:true, so StartUserNotice is empty by design and the event would narrate nothing anyway",

	// ── the event path cannot express what the call needs ───────────────────
	"internal/hooks/Awareness_Cascades.go|64": "condition 9 must be applied PERMANENT so the awareness state machine owns its lifecycle; the event path has no permanent form, and the transition callback holds only a Character",

	// ── routing would narrate the wrong thing, or narrate it repeatedly ─────
	"internal/hooks/pinnacle_tick.go|339": "the bandolier continuously re-applies any slotted potion condition that has lapsed, so routing would re-narrate each potion's start line on every lapse; the pinnacle announces attunement once itself",
	"internal/hooks/pinnacle_tick.go|353": "the bandolier continuously re-applies any slotted potion condition that has lapsed, so routing would re-narrate each potion's start line on every lapse; the pinnacle announces attunement once itself",
	"internal/justice/arrest.go|645":      "RestoreJailOnLogin re-applies an already-running sentence after the RemoveCondition above, so the hook would read it as a fresh application and clang the cell door shut on every login",

	// ── the condition must be in place before the function returns ─────────
	"internal/justice/arrest.go|395": "silent-start, the arrest narrates; no-go and no-aggro-target are read in the same round dispatch",

	// ── the applier's caller narrates, because internal/combat cannot ───────
	// internal/combat holds only a *characters.Character and sends no player
	// text anywhere in the package, so ResolveSubmissionOutcome reports the
	// conditions it applied and Position_SubmissionTick delivers each
	// authored start line to a player victim through narrateSubmissionEffects.
	"internal/combat/submission_outcome.go|327": "silent-start; the submission hook narrates the start right after the outcome; must apply synchronously",
	"internal/combat/submission_outcome.go|338": "silent-start; the submission hook narrates the start right after the outcome; must apply synchronously",

	// ── former combat conditions: grapple exposure and prone recovery are now
	// quiet one-round records (Task 5) ──────────────────────────────────────
	"internal/combat/grapple_move.go|63": "former combat condition (one-round penalty): quiet record; must apply synchronously inside the round tick",

	// ── former combat condition: Regenerating is now one record (Task 7;
	// re-keyed slice 1b, same shift as above; re-keyed again Task 10 and
	// Task 10's follow-up; re-keyed again counters slice Task 3, same
	// deletion as above; re-keyed again messaging M4d Task 6, same shift as
	// above; re-keyed again messaging M4d PR 3 Task 3, same shift as above;
	// re-keyed again parity slice 2, same deletion as above; re-keyed again
	// spell effects 3a Task 1 and Task 2, same shifts as above; re-keyed
	// again spell effects 3a Task 4, same shift as above; re-keyed again
	// spell effects 3a Task 5, same shift as above; re-keyed again parity
	// slice 3a Task 7, same guard widening as above; re-keyed again parity
	// slice 3a Task 8, same deletion as above; re-keyed again parity slice 3b
	// Task 1, same collapse as above, and Task 2, same move as above;
	// collapsed again parity slice 3b Task 3, when applySpellHeal replaced the
	// PM/MM and PP heal arms and MP gained the heal; re-keyed the surviving MS
	// row for the same line shift; re-keyed again Task 4, when
	// applySpellShield's new math and configs imports shifted
	// spell_help_effects.go and the deleted PP shield arm shifted
	// spell_resolution.go; re-keyed again Task 5, same deletion as above;
	// the MS row DELETED by Task 6, same deletion as above, a mob's
	// self-cast heal now reaching applySpellHeal's row; re-keyed again
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above; re-keyed again messaging M6 slice 1, when
	// spellConditionsNarrateStart landed above it and again when the shield
	// arm moved to the event door and its Conviction Ward row was deleted; the
	// spell row DELETED messaging M6 slice 1, when each heal spell began
	// landing its own heal through the condition event, leaving the two
	// feeding rows) ───────
	"internal/mobcommands/consume.go|46": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|58": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",

	// ── former combat condition: the spell dot is now one record (Task 8;
	// re-keyed slice 1b when the dot moved to every round; re-keyed again
	// Task 10 and Task 10's follow-up; re-keyed again counters slice
	// Task 3, same deletion as above; re-keyed again messaging M4d Task 6,
	// same shift as above; re-keyed again messaging M4d PR 3 Task 3, same
	// shift as above; re-keyed again parity slice 2, same deletion as
	// above; re-keyed again spell effects 3a Task 1 when the MP switch moved
	// into applyMobOnPlayerArms, and Task 2 when the damage arms that sat
	// above both dot calls moved into applySpellDamage; collapsed to ONE row
	// by spell effects 3a Task 3, when applySpellDot replaced the PM/MM and
	// MP dot arms and PP gained the dot; re-keyed again spell effects 3a
	// Task 4 when applySpellKnockdown's new imports shifted spell_effects.go,
	// and Task 5 when the configs and util imports did; re-keyed again parity
	// slice 3a Task 7, same guard widening as above; re-keyed again spell
	// effects 3a when creditSpellDamage landed above applySpellDot; re-keyed
	// again parity slice 3b Task 2 when the dispatcher gained its condition
	// case, and again Task 3 when it gained its heal case, and again Task 4
	// when it gained its shield case, and again Task 5 when its doc comment
	// grew and the per-pairing fallthrough became the default arm; moved to
	// AddConditionMagnitudeBy and re-keyed messaging M6 slice 1, when the
	// dot began to carry its caster and creditMobHarm landed above it) ──────
	"internal/hooks/spell_effects.go|355": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",

	// ── magnitude potions (lighting plan 5c): the player's and the mob's drink
	// each carried this call (usercommands/drink.go and mobcommands/drink.go)
	// until drink path unification made both thin wrappers over one shared
	// body. actor is a DrinkActor, whose two implementers are the UserActor
	// and MobActor doors below; both queue events.Condition, so the drinker
	// reads the start line. The AI companion and the survival planner drink
	// through it. Re-keyed when the room lines moved to
	// SendTextVisualHidingNames and their comment grew ────────────────────────
	"internal/actions/drink.go|354": "potion at its item magnitude scaled by potency, player or mob: the EVENT door (DrinkActor.AddConditionMagnitude reaches users.UserRecord / mobs.Mob AddConditionMagnitude, both queue events.Condition); listed only because arity cannot tell it from the silent character door",

	// ── the DrinkActor doors (drink path unification): a.User is a
	// *users.UserRecord and a.Mob is a *mobs.Mob, whose AddConditionMagnitude
	// both queue events.Condition, so the drinker reads the start line ──────
	"internal/actions/actor_user.go|75": "DrinkActor magnitude door for a player: the EVENT door (users.UserRecord.AddConditionMagnitude queues events.Condition); listed only because arity cannot tell it from the silent character door",
	"internal/actions/actor_mob.go|74":  "DrinkActor magnitude door for a mob: the EVENT door (mobs.Mob.AddConditionMagnitude queues events.Condition); listed only because arity cannot tell it from the silent character door",

	// ── admin setcondition of a magnitude-scaled condition (lighting plan 5c
	// final review): target is a *users.UserRecord or a *mobs.Mob, both of
	// which queue events.Condition ───────────────────────────────────────
	"internal/usercommands/admin.setcondition.go|38": "admin-applied scaled condition at a new character's spell value: the EVENT door (users.UserRecord / mobs.Mob AddConditionMagnitude both queue events.Condition); listed only because arity cannot tell it from the silent character door",

	// ── former combat condition: Bleeding is now one stacking record (Task 9;
	// re-keyed slice 1b; re-keyed again counters slice Task 3 when the
	// drain-area counter field and its exit call were deleted) ───────────
	"internal/actions/combat_drain.go|147":           "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
	"internal/actions/combat_drain.go|314":           "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
	"internal/actions/combat_hamstring.go|138":       "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
	"internal/actions/combat_maul.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
	"internal/actions/combat_rake.go|132":            "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
	"internal/actions/combat_throttle.go|146":        "former combat condition (bleed): silent-start record, the move narrates; must apply synchronously within the move's resolution",
	"internal/behaviortree/actions_item_proc.go|359": "former combat condition (bleed): silent-start record, the proc's item narrates; must apply synchronously within the move's resolution",
}

// primitivePackages define Character.AddCondition / Conditions.AddCondition
// themselves, so every call inside them is the primitive or its own internal
// plumbing.
var primitivePackages = []string{
	filepath.Join("internal", "conditions"),
	filepath.Join("internal", "characters"),
}

// AddConditionMagnitudeBy is the character door with a caster (messaging M6
// slice 1); it applies in place exactly as AddConditionMagnitude does.
var conditionAddCallPattern = regexp.MustCompile(`\.(AddCondition(?:Scaled|Magnitude(?:By)?)?)\(`)

// identifierArgPattern matches a bare identifier or field selector, which is
// how a variable source reaches these calls: src, reason, source, evt.Source.
var identifierArgPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// couldBeSourceArg reports whether a call's final argument could be a source
// string. A string literal always can. A bare identifier or selector can too,
// so an event-path call passing a variable is never reported as silent; that
// costs a little precision on a call like AddConditionScaled(id, mult) whose
// last argument is a bare float variable, which is why each remaining direct
// add is still allowlisted by hand.
//
// The bool literals are the exception that matters: `true` and `false` are
// identifiers in Go's grammar, and they are exactly what
// Character.AddCondition(id, isPermanent) is given, so accepting them would
// blind the guard to the single most common silent add in the codebase.
func couldBeSourceArg(arg string) bool {
	if strings.HasPrefix(arg, `"`) || strings.HasPrefix(arg, "`") {
		return true
	}
	switch arg {
	case "true", "false", "nil":
		return false
	}
	return identifierArgPattern.MatchString(arg)
}

// callArgs splits the top-level arguments of the call whose opening paren is at
// openParen, skipping over nested calls, composite literals and string bodies.
// ok is false when the closing paren cannot be found, in which case the caller
// must flag rather than assume.
func callArgs(src string, openParen int) (args []string, ok bool) {
	depth := 0
	argStart := openParen + 1
	for i := openParen; i < len(src); i++ {
		switch src[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 && src[i] == ')' {
				if last := strings.TrimSpace(src[argStart:i]); last != "" || len(args) > 0 {
					args = append(args, last)
				}
				return args, true
			}
		case ',':
			if depth == 1 {
				args = append(args, strings.TrimSpace(src[argStart:i]))
				argStart = i + 1
			}
		case '"', '`', '\'':
			quote := src[i]
			for i++; i < len(src); i++ {
				if src[i] == '\\' && quote == '"' {
					i++
					continue
				}
				if src[i] == quote {
					break
				}
			}
		}
	}
	return nil, false
}

// isEventPathCall reports whether a matched call is one of the event-path adds.
// Both the ARITY and the final argument have to fit, which is what keeps the
// rule sharp in each direction: AddConditionScaled takes a source only in its
// three-argument form, so the silent two-argument
// Character.AddConditionScaled(id, mult) is still caught even though a bare
// float variable looks like an identifier, while a correct call passing a
// variable source is never reported as silent. The second result is false
// when the call could not be parsed.
func isEventPathCall(src string, method string, openParen int) (eventPath bool, parsed bool) {
	if method == "AddConditionMagnitude" || method == "AddConditionMagnitudeBy" {
		// The character door and the user door share a four-argument shape
		// ending in a source string, so arity cannot tell them apart. Treat
		// every call as a direct add: the safe reading, since a producer
		// site that is actually the event door gets allowlisted by hand.
		return false, true
	}
	args, ok := callArgs(src, openParen)
	if !ok {
		return false, false
	}
	want := 2
	if method == "AddConditionScaled" {
		want = 3
	}
	if len(args) != want {
		return false, true
	}
	return couldBeSourceArg(args[len(args)-1]), true
}

func TestPlayerConditionsTravelTheEventPath(t *testing.T) {
	var problems []string
	seen := map[string]bool{}

	for _, root := range []string{"internal", "modules"} {
		scanned := 0
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// A test elsewhere can create and remove a temp file under
					// the tree while packages test in parallel; a vanished
					// entry has nothing to scan.
					return nil
				}
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			for _, p := range primitivePackages {
				if strings.HasPrefix(path, p+string(filepath.Separator)) {
					return nil
				}
			}
			scanned++

			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(raw)
			// Slash-normalised so a key reads the same on every platform.
			slashPath := filepath.ToSlash(path)

			for _, loc := range conditionAddCallPattern.FindAllStringSubmatchIndex(src, -1) {
				openParen := loc[1] - 1
				eventPath, parsed := isEventPathCall(src, src[loc[2]:loc[3]], openParen)
				if parsed && eventPath {
					continue
				}
				lineNo := 1 + strings.Count(src[:loc[0]], "\n")
				lineHead := strings.TrimSpace(src[strings.LastIndexByte(src[:loc[0]], '\n')+1 : loc[0]])
				// A commented-out call applies nothing. Without this, the T7
				// placeholder note in submission_outcome.go reads as a real
				// silent add and would earn a meaningless allowlist entry.
				if strings.HasPrefix(lineHead, "//") || strings.HasPrefix(lineHead, "*") {
					continue
				}
				key := fmt.Sprintf("%s|%d", slashPath, lineNo)
				if _, ok := conditionApplyPathAllowlist[key]; ok {
					seen[key] = true
					continue
				}
				lineStart := strings.LastIndexByte(src[:loc[0]], '\n') + 1
				lineEnd := lineStart + strings.IndexByte(src[lineStart:], '\n')
				if lineEnd < lineStart {
					lineEnd = len(src)
				}
				rule := "an event-path condition add ends in a source string; this call does not, so it applies the condition in place, Condition_ApplyConditions never runs, and the holder reads nothing."
				advice := fmt.Sprintf("Route it through users.UserRecord.AddCondition / AddConditionScaled (or the mobs.Mob / actions.Actor equivalent, which all take a source string), or add %q to conditionApplyPathAllowlist with a reason.", key)
				if method := src[loc[2]:loc[3]]; method == "AddConditionMagnitude" || method == "AddConditionMagnitudeBy" {
					rule = "the character door applies in place and the user door queues the event, but they share one four-argument shape, so this guard reads every AddConditionMagnitude call as a direct add (the safe reading). Record why in conditionApplyPathAllowlist, or confirm the call is the user door and record that instead."
					advice = "Routing it through users.UserRecord.AddConditionMagnitude does not silence this guard; allowlist it either way, noting which door it is."
				}
				problems = append(problems, fmt.Sprintf(
					"%s: %s\n      THE RULE: %s\n      %s",
					key, strings.TrimSpace(src[lineStart:lineEnd]), rule, advice))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("cannot walk %s: %v", root, err)
		}
		if scanned == 0 {
			t.Fatalf("no non-test .go files scanned under %s; the guard would pass vacuously", root)
		}
	}

	// A stale allowlist entry is a silent hole: the line it pardons has moved,
	// so the real add at the new line is pardoned by nothing, and the entry now
	// pardons whatever happens to sit there instead.
	var stale []string
	for key := range conditionApplyPathAllowlist {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		problems = append(problems, fmt.Sprintf(
			"%s: allowlisted but no direct condition add is on that line any more; find where it moved and update the key", key))
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("%d condition delivery path problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}
