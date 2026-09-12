package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
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
// event-path method takes a source string as its last argument, and no direct
// one does.
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
// So a call whose last argument is a string is the event path and passes; every
// other call is a silent add and needs a reason in the allowlist below, keyed
// "file|line". Legitimate reasons: the holder is a mob (no client to read a
// line), the buff is flagged silent-start and its applier narrates the moment
// itself, a synchronous apply is read back in the same tick, or the buff is
// secret. When a legitimate direct add moves, update its line number here;
// when a new one appears, either route it through the event path or record why
// it cannot be.
var buffApplyPathAllowlist = map[string]string{
	// ── The sanctioned consumer of the event ────────────────────────────────
	"internal/hooks/Buff_ApplyBuffs.go|76": "this IS the hook the event feeds; it is where every routed buff is finally applied",
	"internal/hooks/Buff_ApplyBuffs.go|78": "this IS the hook the event feeds; it is where every routed buff is finally applied",

	// ── silent-start buffs whose applier narrates the moment itself ─────────
	"internal/actions/combat_rally.go|116":    "buff 80 is silent-start; the rally move narrates the party rally and the synchronous apply is what its own messaging reads",
	"internal/actions/combat_warcry.go|118":   "buff 79 is silent-start; the warcry move narrates the cry and the synchronous apply is what its own messaging reads",
	"internal/actions/combat_throttle.go|147": "buff 89 is silent-start; the throttle move narrates the choke as it lands and must apply it in the same tick",
	"internal/actions/sleep.go|59":            "buff 15 is silent-start; Sleep reads the Sleeping flag back for idempotence and the schedule executor expects it applied on return, so Sleep sends the start line itself",
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
	"internal/justice/arrest.go|614":      "RestoreJailOnLogin re-applies an already-running sentence after the RemoveBuff above, so the hook would read it as a fresh application and clang the cell door shut on every login",

	// ── OPEN DEFECT, recorded rather than hidden ────────────────────────────
	// Buffs 83 and 84 carry authored start_user_text that reaches nobody:
	// internal/combat holds only a *characters.Character here and sends no
	// player text anywhere in the package, and no other code narrates the
	// stun or the broken limb. Flagging them silent-start would assert that
	// the applier narrates, which is false. Awaiting an owner decision on
	// where a submission's consequences should be told to the victim.
	"internal/combat/submission_outcome.go|271": "OPEN: buff 83 Broken Limb start_user_text reaches nobody; internal/combat has no player-text path and nothing else narrates it",
	"internal/combat/submission_outcome.go|282": "OPEN: buff 84 Stunned start_user_text reaches nobody; internal/combat has no player-text path and nothing else narrates it",
}

// primitivePackages define Character.AddBuff / Buffs.AddBuff themselves, so
// every call inside them is the primitive or its own internal plumbing.
var primitivePackages = []string{
	filepath.Join("internal", "buffs"),
	filepath.Join("internal", "characters"),
}

var buffAddCallPattern = regexp.MustCompile(`\.AddBuff(?:Scaled)?\(`)

// lastArgIsStringSource reports whether the call starting at the byte just
// after its opening paren passes a string as its final argument, which is what
// every event-path AddBuff signature takes and no direct one does. The second
// result is false when the call's closing paren cannot be found, in which case
// the caller must flag rather than assume.
func lastArgIsStringSource(src string, openParen int) (isEventPath bool, parsed bool) {
	depth := 0
	lastComma := -1
	for i := openParen; i < len(src); i++ {
		switch src[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 && src[i] == ')' {
				last := src[openParen+1 : i]
				if lastComma >= 0 {
					last = src[lastComma+1 : i]
				}
				last = strings.TrimSpace(last)
				return strings.HasPrefix(last, `"`) || strings.HasPrefix(last, "`") || last == "source", true
			}
		case ',':
			if depth == 1 {
				lastComma = i
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
	return false, false
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

			for _, loc := range buffAddCallPattern.FindAllStringIndex(src, -1) {
				openParen := loc[1] - 1
				eventPath, parsed := lastArgIsStringSource(src, openParen)
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
					"%s: %s\n      a player buff applied this way is SILENT: route it through users.UserRecord.AddBuff / AddBuffScaled (or the Mob / Actor equivalent, which all take a source string), or add %q to buffApplyPathAllowlist with a reason",
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
