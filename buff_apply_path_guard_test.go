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
// string; no direct, silent add does. If someone gives Character.AddBuff a
// source parameter, or drops one from UserRecord.AddBuff, these stop compiling
// and the guard is fixed deliberately instead of quietly going blind.
var (
	_ func(int, bool) error      = (*characters.Character)(nil).AddBuff
	_ func(int, float64) error   = (*characters.Character)(nil).AddBuffScaled
	_ func(int, string)          = (*users.UserRecord)(nil).AddBuff
	_ func(int, float64, string) = (*users.UserRecord)(nil).AddBuffScaled
	_ func(int, string)          = (*mobs.Mob)(nil).AddBuff
	_ func(int, string)          = (*actions.UserActor)(nil).AddBuff
	_ func(int, string)          = (*actions.MobActor)(nil).AddBuff
)

// Slice C delivery path: a player buff must travel the events.Buff event, so
// the Buff_ApplyBuffs hook runs and narrates the start through
// buffs.BuffSpec.StartUserNotice. A direct character-level add
// (Character.AddBuff / Character.AddBuffScaled, or Buffs.AddBuff beneath them)
// applies the buff in place and queues nothing, so the hook never runs and the
// holder reads no line at all. That is not a theoretical gap: a playtest drank
// a Purging Draught and took fifty rounds of Purging Weakness in silence, and
// the same defect was hiding in the quest reward path, the Bloom crash and
// withdrawal, the arrest, and sleep.
//
// How the two are told apart WITHOUT relying on receiver names: every
// event-path method ends in a source string, and no direct one does. The
// signatures are pinned by the compiler above, not asserted here.
//
//	users.UserRecord.AddBuff(buffId int, source string)
//	users.UserRecord.AddBuffScaled(buffId int, durationMult float64, source string)
//	mobs.Mob.AddBuff(buffId int, source string)
//	actions.Actor.AddBuff(buffId int, source string)   // UserActor + MobActor
//
//	characters.Character.AddBuff(buffId int, isPermanent bool)      // silent
//	characters.Character.AddBuffScaled(buffId int, durationMult float64) // silent
//	buffs.Buffs.AddBuff / AddBuffScaled                            // silent
//
// A call passes when its ARITY and its final argument both fit the event-path
// shape: two arguments for AddBuff, three for AddBuffScaled, with a last
// argument that could be a string (a literal, or an identifier or selector, so
// a variable source is never mistaken for a silent add). The bool literals are
// excluded from that, since `false` is an identifier to a text scanner and is
// exactly what the silent Character.AddBuff takes. Checking arity as well as
// the argument is what keeps the two-argument Character.AddBuffScaled(id, mult)
// caught, whose bare float variable would otherwise read as a source.
//
// Every other call is a silent add and needs a reason in the allowlist below,
// keyed "file|line". Legitimate reasons: the holder is a mob (no client to read
// a line), the buff is flagged silent-start and its applier narrates the moment
// itself, the buff has to be in place before the function returns, or the buff
// is secret. When a legitimate direct add moves, update its line number here;
// when a new one appears, either route it through the event path or record why
// it cannot be.
var buffApplyPathAllowlist = map[string]string{
	// ── The sanctioned consumer of the event ────────────────────────────────
	"internal/hooks/Buff_ApplyBuffs.go|81": "this IS the hook the event feeds; it is where every routed buff is finally applied",
	"internal/hooks/Buff_ApplyBuffs.go|83": "this IS the hook the event feeds; it is where every routed buff is finally applied",

	// ── silent-start buffs whose applier narrates the moment itself ─────────
	"internal/actions/combat_rally.go|116":    "buff 80 is silent-start; the rally move narrates the party rally and the synchronous apply is what its own messaging reads",
	"internal/actions/combat_warcry.go|118":   "buff 79 is silent-start; the warcry move narrates the cry and the synchronous apply is what its own messaging reads",
	"internal/actions/combat_throttle.go|147": "buff 89 is silent-start; the throttle move narrates the choke as it lands and must apply it in the same tick",
	"internal/actions/sleep.go|60":            "buff 15 is silent-start; Sleep reads the Sleeping flag back for its own idempotence check, so it sends the start line itself",
	"internal/usercommands/rally.go|57":       "party member gets buff 80, which is silent-start; the rally command narrates it and relies on the synchronous apply",
	"internal/usercommands/rally.go|87":       "party member gets buff 79, which is silent-start; the rally command narrates it and relies on the synchronous apply",
	"internal/usercommands/warcry.go|57":      "party member gets buff 79, which is silent-start; the warcry command narrates it and relies on the synchronous apply",
	"internal/usercommands/warcry.go|91":      "party member gets buff 80, which is silent-start; the warcry command narrates it and relies on the synchronous apply",

	// ── mob holders: no client, so no line could reach anyone ───────────────
	"internal/usercommands/rally.go|117":         "charmed mob, not a player; a mob holder has no client to read a start line",
	"internal/usercommands/warcry.go|121":        "charmed mob, not a player; a mob holder has no client to read a start line",
	"internal/usercommands/character.go|413":     "the holder is a MOB (m.Character), and buff 99 is a perma-gear pin, not something a player reads",
	"internal/hooks/item_procs.go|264":           "the holder is a MOB (m.Character); an item proc stunning a mob has nobody to tell",
	"internal/hooks/manifester_companions.go|40": "the holder is a MOB (a summoned companion), not a player",

	// ── secret buffs: silence is the authored intent ────────────────────────
	"internal/hooks/Life_Cascades.go|87": "buff 81 Respawn Grace is secret:true, so StartUserNotice is empty by design and the event would narrate nothing anyway",

	// ── the event path cannot express what the call needs ───────────────────
	"internal/hooks/Awareness_Cascades.go|57": "buff 9 must be applied PERMANENT so the awareness state machine owns its lifecycle; the event path has no permanent form, and the transition callback holds only a Character",

	// ── routing would narrate the wrong thing, or narrate it repeatedly ─────
	"internal/hooks/pinnacle_tick.go|335": "the bandolier continuously re-applies any slotted potion buff that has lapsed, so routing would re-narrate each potion's start line on every lapse; the pinnacle announces attunement once itself",
	"internal/hooks/pinnacle_tick.go|349": "the bandolier continuously re-applies any slotted potion buff that has lapsed, so routing would re-narrate each potion's start line on every lapse; the pinnacle announces attunement once itself",
	"internal/justice/arrest.go|634":      "RestoreJailOnLogin re-applies an already-running sentence after the RemoveBuff above, so the hook would read it as a fresh application and clang the cell door shut on every login",

	// ── the buff must be in place before the function returns ───────────────
	"internal/justice/arrest.go|395": "silent-start, the arrest narrates; no-go and no-aggro-target are read in the same round dispatch",

	// ── the applier's caller narrates, because internal/combat cannot ───────
	// internal/combat holds only a *characters.Character and sends no player
	// text anywhere in the package, so ResolveSubmissionOutcome reports the
	// buffs it applied and Position_SubmissionTick delivers each authored
	// start line to a player victim through narrateSubmissionEffects.
	"internal/combat/submission_outcome.go|327": "silent-start; the submission hook narrates the start right after the outcome; must apply synchronously",
	"internal/combat/submission_outcome.go|338": "silent-start; the submission hook narrates the start right after the outcome; must apply synchronously",
}

// primitivePackages define Character.AddBuff / Buffs.AddBuff themselves, so
// every call inside them is the primitive or its own internal plumbing.
var primitivePackages = []string{
	filepath.Join("internal", "buffs"),
	filepath.Join("internal", "characters"),
}

var buffAddCallPattern = regexp.MustCompile(`\.(AddBuff(?:Scaled)?)\(`)

// identifierArgPattern matches a bare identifier or field selector, which is
// how a variable source reaches these calls: src, reason, source, evt.Source.
var identifierArgPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// couldBeSourceArg reports whether a call's final argument could be a source
// string. A string literal always can. A bare identifier or selector can too,
// so an event-path call passing a variable is never reported as silent; that
// costs a little precision on a call like AddBuffScaled(id, mult) whose last
// argument is a bare float variable, which is why each remaining direct add is
// still allowlisted by hand.
//
// The bool literals are the exception that matters: `true` and `false` are
// identifiers in Go's grammar, and they are exactly what
// Character.AddBuff(id, isPermanent) is given, so accepting them would blind
// the guard to the single most common silent add in the codebase.
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
// rule sharp in each direction: AddBuffScaled takes a source only in its
// three-argument form, so the silent two-argument Character.AddBuffScaled(id,
// mult) is still caught even though a bare float variable looks like an
// identifier, while a correct call passing a variable source is never reported
// as silent. The second result is false when the call could not be parsed.
func isEventPathCall(src string, method string, openParen int) (eventPath bool, parsed bool) {
	args, ok := callArgs(src, openParen)
	if !ok {
		return false, false
	}
	want := 2
	if method == "AddBuffScaled" {
		want = 3
	}
	if len(args) != want {
		return false, true
	}
	return couldBeSourceArg(args[len(args)-1]), true
}

func TestPlayerBuffsTravelTheEventPath(t *testing.T) {
	var problems []string
	seen := map[string]bool{}

	for _, root := range []string{"internal", "modules"} {
		scanned := 0
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
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

			for _, loc := range buffAddCallPattern.FindAllStringSubmatchIndex(src, -1) {
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
				if _, ok := buffApplyPathAllowlist[key]; ok {
					seen[key] = true
					continue
				}
				lineStart := strings.LastIndexByte(src[:loc[0]], '\n') + 1
				lineEnd := lineStart + strings.IndexByte(src[lineStart:], '\n')
				if lineEnd < lineStart {
					lineEnd = len(src)
				}
				problems = append(problems, fmt.Sprintf(
					"%s: %s\n      THE RULE: an event-path buff add ends in a source string; this call does not, so it applies the buff in place, Buff_ApplyBuffs never runs, and the holder reads nothing.\n      Route it through users.UserRecord.AddBuff / AddBuffScaled (or the mobs.Mob / actions.Actor equivalent, which all take a source string), or add %q to buffApplyPathAllowlist with a reason.",
					key, strings.TrimSpace(src[lineStart:lineEnd]), key))
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
	for key := range buffApplyPathAllowlist {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		problems = append(problems, fmt.Sprintf(
			"%s: allowlisted but no direct buff add is on that line any more; find where it moved and update the key", key))
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("%d buff delivery path problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}
