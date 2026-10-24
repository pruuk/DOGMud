package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// sendTrioOnlyCategories is every messaging.Category whose production sends
// have ALL been verified, by grep across internal/ and modules/ on this
// branch (2026-09-20), to originate only from messaging.Say (building a
// messaging.Trio literal) or from acteeDefenceLine (which itself only wraps
// Say). No other Send-family call -- SendText, SendTextVisual*,
// SendToTarget*, SendToSource*, SendToRoomOld, or a free-function wrapper
// such as sendMovementMessage (or the since-deleted sendAudioRoomText) -- ever carries one of these
// three as a literal argument in production code today.
//
// This is a SMALL, DELIBERATELY NARROW set, not "every narration category."
// A full survey of all ~59 messaging.Category values (2026-09-20) found only
// these three fully migrated; every other combat/spell/speech category still
// has at least one live bypass:
//
//   - CategoryHitMelee/HitBlunt/HitNaturalSharp/HitRanged/HitCaster/
//     HitUnarmed/Dodge/Parry/Block: produced by combat.AttackResult's
//     TaggedMessage buffer (internal/combat/attackresult.go,
//     CategoryForWeaponSubtype / CategoryForDefenseVerb) and drained by raw
//     SendText in internal/hooks/combat_verbosity.go -- PR 2's subject.
//   - CategoryGrappleFlow/CategorySubmission: internal/hooks/
//     Position_Messaging.go and Position_GrappleTick.go send both directly;
//     deferred (personal lines name the other grappler).
//   - CategorySpellFold/Disruption/Elemental/Enhancement/Mental/Vital/
//     Manifestation: internal/hooks/spell_resolution.go, four of five
//     applyPlayerEffect self-cast branches still pair a personal line with a
//     direct sendVisualRoomText room line.
//   - CategoryNPCDialogue: internal/questengine/bridge.go's Narrate sends
//     trigger observer lines directly; every line carries {actor} by
//     validation, so moving it would add name hiding in the dark.
//   - CategoryTauntSuccess/TauntResist/TauntFailure/Rally/Warcry/Shout/
//     Speech/Whisper/NPCDialogue (again, via the audio helper): sent through
//     sendAudioRoomText, a pre-M4 raw helper with its own nightvision-based
//     name hiding, never SendTrio.
//   - CategoryRoomEntry/RoomExit/Logout: sendMovementMessage /
//     SendTextVisualWithAudio / sendVisualRoomText, single-audience movement
//     announcements, never routed through Trio.
//   - CategoryGrappleHigh/OOC/Login/Toxin: zero production senders at all
//     (vacuous; not included -- a guard needs a real sender to ever go red).
//
// This guard states a true invariant today. As PR 2 migrates a category's
// last raw sender, add it here; if a regression reopens a bypass for one of
// these three, add the file below with a comment naming why, same as
// rawEventsMessageAllowed in raw_events_message_guard_test.go.
//
// M4d PR 3 re-survey (2026-09-20, this branch): PR 3 migrated five more
// production paths (ranged wait-round room lines, quest trigger narration,
// four self-cast spell branches, the position_control stamina warning and
// submission triple, and crafting's instant-complete room line) onto
// SendTrio. None of them closed out a whole category -- every one of the
// five still has at least one raw sender left outside SendTrio elsewhere in
// the same category, so sendTrioOnlyCategories is unchanged at three:
//
//   - CategoryHitRanged (ranged wait-round, PR 3 Task 1): still drained raw
//     by internal/hooks/combat_verbosity.go's drainParticipantLines (every
//     normal ranged swing, not just the wait round) and sent raw at
//     internal/usercommands/shoot.go:245 (`user.SendText(messaging.
//     CategoryHitRanged, line)`).
//   - CategoryNPCDialogue (quest Narrate, PR 3 Task 2): still sent raw at
//     internal/questengine/bridge.go:515 and :519 (QueueSequence's delayed
//     and immediate dialogue lines), internal/actions/actor_mob.go:52, and
//     internal/behaviortree/actions_dialogue.go:37,44,99.
//   - CategorySpellElemental/Enhancement/Mental/Vital/Manifestation (four
//     self-cast branches, PR 3 Task 3): the mob-target spell paths still
//     pair raw user.SendText / sendVisualRoomText calls carrying
//     spellSchoolCategory(spellData) for every opposed/attack cast.
//     Parity slices 3a and 3b moved every spell effect onto SendTrio
//     (spell_effects.go and spell_help_effects.go); the raw senders left
//     are applyMobEffect_charm (charm_spell.go:26), resolveSpell's
//     no-target and magic lines, resolveIdentify and
//     resolvePurgeAffliction's self-cast lines. CategorySpellFold was
//     untouched by PR 3 entirely.
//   - CategoryGrappleFlow/CategorySubmission (position_control, PR 3 Task
//     4): GrappleFlow is still sent raw at internal/hooks/
//     Position_GrappleTick.go:668,706,708,710 and internal/mobcommands/
//     flee.go:41. CategorySubmission -- despite sendSubmissionTriple now
//     being fully Say()-shaped -- is still sent raw at
//     internal/hooks/item_procs.go:275 (the condition-84 stagger shockwave,
//     a pre-existing sender PR 3 did not touch).
//   - CategorySystem/CategoryEmote (crafting instant-complete, PR 3 Task 5):
//     both are among the most widely raw-sent categories in the tree (e.g.
//     internal/usercommands/emote.go:17-48 for Emote; CategorySystem alone
//     has raw senders in over 200 production files) -- nowhere near closed
//     by one crafting call site.
//
// Honest result: zero categories added this pass. sendTrioOnlyCategories
// stays {Kick, Trip, Bash}; sendTrioOnlyAllowed stays empty.
//
// M5 PR 1 update (2026-09-22, this branch): commit 254aa49e9 migrated the
// item_procs.go:275 condition-84 stagger shockwave -- the sender the
// CategoryGrappleFlow/CategorySubmission bullet above names as the one raw
// CategorySubmission sender left after PR 3 -- onto SendTrio. That was
// CategorySubmission's last raw sender, so it joined sendTrioOnlyCategories
// below. The bullet above is a survey record of PR 3 and stays as written;
// this note is what supersedes it.
//
// Sight gates slice 5b (2026-09-29): mobcommands/darkness.go's
// sendAudioRoomText and sendAudioRoomTextHidingNames are deleted.
// The Taunt*,
// Rally, Warcry, Shout and Speech categories now leave through
// rooms.Room.SendTextHidingNames and SendCommunicationHidingNames, still not
// SendTrio, so sendTrioOnlyCategories is unchanged. The survey bullets above
// stay as the record of their dates.
var sendTrioOnlyCategories = []string{"Kick", "Trip", "Bash", "Submission"}

// sendTrioOnlyAllowed lists production files permitted to reference one of
// sendTrioOnlyCategories outside a Say(...)/acteeDefenceLine(...) producer
// shape. It is empty: today nothing bypasses SendTrio for Kick, Trip, or
// Bash. A future entry must name the file, why it bypasses, and which PR
// closes it -- the same shape as rawEventsMessageAllowed.
var sendTrioOnlyAllowed = map[string]string{}

// sendTrioOnlyCategoryRE matches a literal reference to one of the guarded
// categories, qualified with its package (production code outside
// internal/messaging always writes messaging.CategoryX).
var sendTrioOnlyCategoryRE = regexp.MustCompile(
	`messaging\.Category(` + strings.Join(sendTrioOnlyCategories, "|") + `)\b`)

// sendTrioOnlyProducerRE matches the shapes allowed to carry a guarded
// category on the same line: building a messaging.Line via Say, the
// skill-move defence helper that itself only calls Say, or the special-move
// store's call-site helpers (sendMoveEvent, lineOrNone) added in M4e-1 Task 7
// -- both of which, like acteeDefenceLine, only ever produce a line through
// Say or the zero-value NoLine. moveCategories{...} (M4e-1b,
// internal/usercommands/move_narration.go) is the player-side helper's
// per-role category literal, consumed only by sendMoveEvent/lineOrNone the
// same way; it never sends a category anywhere else. sendPartialMove (#631,
// both command packages' skill_move_defence.go) hands its category only to
// messaging.Say and sendMoveEvent.
var sendTrioOnlyProducerRE = regexp.MustCompile(`messaging\.Say\(|acteeDefenceLine\(|sendMoveEvent\(|sendPartialMove\(|lineOrNone\(|moveCategories\{`)

func TestNarrationTrioOnlyCategoriesLeaveOnlyThroughSendTrio(t *testing.T) {
	seen := map[string]bool{}
	var outside []string
	scanned := 0

	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			scanned++
			rel := filepath.ToSlash(path)
			for i, line := range strings.Split(string(src), "\n") {
				if !sendTrioOnlyCategoryRE.MatchString(line) {
					continue
				}
				if sendTrioOnlyProducerRE.MatchString(line) {
					continue
				}
				if _, ok := sendTrioOnlyAllowed[rel]; ok {
					seen[rel] = true
					continue
				}
				outside = append(outside, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	// A walk that found no files would pass with nothing outside.
	if scanned < 100 {
		t.Fatalf("read only %d Go files under %v; the walk is not seeing the tree, so a pass proves nothing", scanned, messagingSurfaceGoRoots)
	}

	// Each allowlist entry must still match, which also proves the pattern
	// can match at all: an empty "outside" list from a blind pattern proves
	// nothing. With no entries today this loop runs zero times; the pattern
	// is instead proven capable of matching by the sabotage step recorded in
	// this guard's commit message.
	for file, why := range sendTrioOnlyAllowed {
		if !seen[file] {
			t.Errorf("allowlist entry %s (%s) no longer references a guarded category; remove it", file, why)
		}
	}
	sort.Strings(outside)
	for _, o := range outside {
		t.Errorf("guarded narration category referenced outside SendTrio's producer shapes: %s\n  send through messaging.Say(...) into a messaging.Trio and messaging.SendTrio, not a raw Send call", o)
	}
}
