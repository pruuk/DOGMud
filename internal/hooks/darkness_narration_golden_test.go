package hooks

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: darkness_narration_golden_test.go
//
// M4d PR 2 deletes replaceDarknessMessages (NewRound_DoCombat_helpers.go:438)
// and hides identities at composition instead. This golden freezes what
// TODAY's substitution does -- through the real production seam,
// dispatchCritAndMessaging (NewRound_DoCombat_unified.go) -- so PR 2's diff
// against this file IS the review. Task 1 already filed the twelve
// hardcoded sentences replaceDarknessMessages sends into the M6 content
// ledger (commit 50f74d9d6); this golden is what proves, mechanically, that
// today's production path actually selects them.
//
// WHY dispatchCritAndMessaging AND NOT handleCombatRound. Driving a full
// round means driving real dice: NewRound_DoCombat_routing_test.go's own
// comment records that its assertions "deliberately avoid forcing a
// specific hit/miss outcome... without seeding the RNG," because there is
// no RNG-seeding seam in this codebase. A hit/miss/crit/fumble/defended-swing
// grid needs a DETERMINISTIC outcome per cell, which a live round cannot
// give without a forbidden dice loop. dispatchCritAndMessaging is the exact
// function that both calls replaceDarknessMessages AND runs the verbosity
// drain (drainParticipantLines / drainSpectatorLines) that turns
// AttackResult.MessagesTo* into what a real player actually reads -- so
// calling it directly, with a hand-built *combat.AttackResult, drives every
// production step downstream of "the swing already happened" without
// needing to roll for the swing itself. This is one level higher than a
// unit test of replaceDarknessMessages alone: it also proves the real
// srcCanSee/tgtCanSee computation (messaging.CanSeeSightImpairedOnly through
// real per-character conditions and real room darkness) and the real send
// path (events queue via UserRecord.SendText), not a hand-rolled substitute
// for either.
//
// AttackResult.ParryCritDetected / DodgeCritDetected / BlockCritDetected are
// left false throughout, so applyCritEffects (riposte / auto-trip /
// auto-bash) stays a no-op -- those are a DIFFERENT round-level signal from
// SwingEvent.DefenseCrit and are out of scope for this golden.
//
// LABEL FIXTURE. Following melee_defence_band_golden_test.go's convention:
// AttackResult.MessagesTo{Source,Target,SourceRoom} carry
// "AUTHORED|<outcome>|<role>" instead of real prose, so a row records WHICH
// LINE won (the authored pool line vs. one of the twelve hardcoded dark
// sentences), not prose that could drift for unrelated reasons. A
// hand-built SwingEvent cannot be moved by production randomness, which is
// why this grid can be pinned without simulating a single die roll.
var updateDarkness = flag.Bool("update-darkness", false, "rewrite testdata/darkness_narration.golden")

// seedDarknessSpectator adds a third player (user 3) to room 1 as a pure
// spectator -- never the attacker or defender -- so the grid can record
// what a bystander reads. The spectator carries NightVision, but under the
// graded lighting arc's window model that no longer buys SightFull in this
// truly dark room: a shifted window is still blind below its floor no
// matter how strong the shift (internal/messaging/window.go), and no
// vision condition can restore SightFull at light 0. So the spectator's
// visual room line is gated off in every cell, the same as an unsighted
// bystander's -- which is itself the answer to what this column was
// built to show: replaceDarknessMessages is gone (M4d PR 2), and nothing
// in the current production path treats MessagesToSourceRoom differently
// depending on WHO the darkness gate blocks.
//
// Since lighting plan 5c the spectator reads the audible fallback ("You hear
// fighting close by.") in every cell. sendUnsightedCombatSound
// used to skip anyone holding the nightvision FLAG, so this blind-in-fact
// spectator read nothing at all; it now follows sight (CanSeeShapes).

func seedDarknessSpectator(t *testing.T) func() {
	t.Helper()
	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u2 := users.GetByUserId(2)
	require.NotNil(t, u2)

	spectator := users.NewTestUser(3, "carol", "Carolyn", 1003)
	spectator.Character.RoomId = 1

	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{
		1: u1,
		2: u2,
		3: spectator,
	})

	room1 := rooms.LoadRoom(1)
	require.NotNil(t, room1)
	room1.AddPlayer(3)

	spectator.Character.Conditions.AddCondition(nightEyesConditionId, true)

	return func() {
		room1.RemovePlayer(3)
		cleanupUsers()
	}
}

// setDarkSight rewrites char's conditions so messaging.ParticipantSight(char,
// <a darkened room>) resolves to the requested SightDecision: NightVision
// for SightFull, InfraredVision for SightShapes, and no vision aid at all
// for SightNone. Requires seedNarrationConditions() to already be active
// (nightEyesConditionId / heatEyesConditionId must be registered).
func setDarkSight(char *characters.Character, sight messaging.SightDecision) {
	char.Conditions = conditions.New()
	switch sight {
	case messaging.SightFull:
		char.Conditions.AddCondition(nightEyesConditionId, true)
	case messaging.SightShapes:
		char.Conditions.AddCondition(heatEyesConditionId, true)
	}
}

// sightLabel names a SightDecision for the golden's row keys.
func sightLabel(s messaging.SightDecision) string {
	switch s {
	case messaging.SightFull:
		return "full"
	case messaging.SightShapes:
		return "shapes"
	case messaging.SightNone:
		return "none"
	}
	return "?"
}

// firstDrained renders a drained-line slice for the golden: "(none)" is a
// MEANINGFUL row (nothing was sent), matching melee_defence_band_golden_test.go's
// firstText convention.
func firstDrained(lines []string) string {
	if len(lines) == 0 {
		return "(none)"
	}
	return strings.Join(lines, " | ")
}

// darknessOutcome is one hand-built swing outcome. Each of the six entries
// below drives exactly one branch of replaceDarknessMessages' two switches
// (fumble, crit, deflected-partial, defense-crit-or-used, hit, default-miss),
// which is exactly the twelve hardcoded sentences (six branches x two
// sides) Task 1 filed in the M6 content ledger.
type darknessOutcome struct {
	name        string
	category    messaging.Category
	swing       combat.SwingEvent
	hit         bool
	cleanHit    bool
	damage      int
	defenseUsed combatvocab.Defence
}

func darknessOutcomes() []darknessOutcome {
	return []darknessOutcome{
		// se.Hit branch: "You strike blindly and connect!" / "Something strikes you in the dark!"
		{name: "hit", category: messaging.CategoryHitMelee,
			swing: combat.SwingEvent{Hit: true, Damage: 10}, hit: true, cleanHit: true, damage: 10},
		// se.Crit branch: "You land a devastating blow in the dark!" / "Something hits you hard in the dark!"
		{name: "crit", category: messaging.CategoryHitMelee,
			swing: combat.SwingEvent{Hit: true, Crit: true, Damage: 20}, hit: true, cleanHit: true, damage: 20},
		// se.DefenseUsed != "" && !se.DefenseCrit && se.Damage > 0: "turns your blow aside, but you feel it land" / "fend off something... still catches you"
		{name: "deflected", category: messaging.CategoryHitMelee,
			swing: combat.SwingEvent{Hit: true, DefenseUsed: combatvocab.DefenceDodge, Damage: 5},
			hit:   true, cleanHit: false, damage: 5, defenseUsed: combatvocab.DefenceDodge},
		// se.DefenseCrit || se.DefenseUsed != "": "Your attack is turned aside by something!" / "You fend off something in the dark!"
		{name: "defense_crit", category: messaging.CategoryDodge,
			swing: combat.SwingEvent{DefenseCrit: true, DefenseUsed: combatvocab.DefenceParry},
			hit:   false, cleanHit: false, defenseUsed: combatvocab.DefenceParry},
		// default (no Hit, no Crit, no Fumble, no DefenseUsed): "You swing wildly in the darkness!" / "You hear something whoosh past!"
		{name: "miss", category: messaging.CategoryDodge, swing: combat.SwingEvent{}, hit: false, cleanHit: false},
		// se.Fumble branch: "You stumble badly in the darkness!" / "You hear your attacker stumble!"
		{name: "fumble", category: messaging.CategoryHitMelee,
			swing: combat.SwingEvent{Fumble: true}, hit: false, cleanHit: false},
	}
}

// TestDarknessNarrationGolden crosses attacker sight x defender sight x
// outcome and records, per cell, what the attacker reads, what the
// defender reads, and what a spectator reads -- driven through the real
// dispatchCritAndMessaging seam. See the file-level comment for why this
// function and not a full handleCombatRound.
func TestDarknessNarrationGolden(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreConditions := seedNarrationConditions()
	defer restoreConditions()
	restoreSpectator := seedDarknessSpectator(t)
	defer restoreSpectator()

	darken(t, 1)

	room1 := rooms.LoadRoom(1)
	require.NotNil(t, room1)

	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u2 := users.GetByUserId(2)
	require.NotNil(t, u2)

	atk := actions.NewUserActorInRoom(u1, room1)
	def := actions.NewUserActorInRoom(u2, room1)

	sights := []messaging.SightDecision{messaging.SightFull, messaging.SightShapes, messaging.SightNone}
	outcomes := darknessOutcomes()

	var b strings.Builder
	fmt.Fprintf(&b, "# dark-versus-lit combat narration matrix -- internal/hooks dispatchCritAndMessaging\n")
	fmt.Fprintf(&b, "# Each cell drives the REAL production seam (dispatchCritAndMessaging, including\n")
	fmt.Fprintf(&b, "# replaceDarknessMessages and the verbosity drain) with a one-variant-per-outcome\n")
	fmt.Fprintf(&b, "# label fixture, so the golden records WHICH LINE won, not authored prose.\n")
	fmt.Fprintf(&b, "# columns: atk sight | def sight | outcome | reader => text\n")
	fmt.Fprintf(&b, "# PRE-FLIP (today): any cell where a participant's OWN sight isn't 'full' shows one\n")
	fmt.Fprintf(&b, "# of the twelve hardcoded dark sentences on THAT participant's line, replacing the\n")
	fmt.Fprintf(&b, "# AUTHORED pool line. The spectator line is never substituted by replaceDarknessMessages\n")
	fmt.Fprintf(&b, "# (it only ever touched MessagesToSource/MessagesToTarget, never MessagesToSourceRoom,\n")
	fmt.Fprintf(&b, "# and M4d PR 2 deleted it regardless) -- but under the graded lighting arc's window\n")
	fmt.Fprintf(&b, "# model, no vision condition restores SightFull in a truly dark room, so the spectator's\n")
	fmt.Fprintf(&b, "# own darkness gates their visual room line off in every cell, same as an unsighted\n")
	fmt.Fprintf(&b, "# bystander, and they read the audible fallback instead (lighting plan 5c: the fallback\n")
	fmt.Fprintf(&b, "# follows sight, not the nightvision flag).\n\n")

	for _, atkSight := range sights {
		for _, defSight := range sights {
			setDarkSight(u1.Character, atkSight)
			setDarkSight(u2.Character, defSight)
			for _, oc := range outcomes {
				res := &combat.AttackResult{
					Hit:                  oc.hit,
					CleanHit:             oc.cleanHit,
					DamageToTarget:       oc.damage,
					DefenderWasAttacked:  true,
					DefenseUsed:          oc.defenseUsed,
					SwingEvents:          []combat.SwingEvent{oc.swing},
					MessagesToSource:     []combat.TaggedMessage{{Category: oc.category, Text: fmt.Sprintf("AUTHORED|%s|actor", oc.name)}},
					MessagesToTarget:     []combat.TaggedMessage{{Category: oc.category, Text: fmt.Sprintf("AUTHORED|%s|actee", oc.name)}},
					MessagesToSourceRoom: []combat.TaggedMessage{{Category: messaging.CategoryHitMelee, Text: fmt.Sprintf("AUTHORED|%s|observer", oc.name)}},
				}

				// Drain anything left over from a prior cell before driving
				// this one, so a stray earlier line can never bleed forward.
				drainPlain(1)
				drainPlain(2)
				drainPlain(3)

				dispatchCritAndMessaging(atk, def, res)

				key := fmt.Sprintf("atk=%s|def=%s|%s", sightLabel(atkSight), sightLabel(defSight), oc.name)
				fmt.Fprintf(&b, "%s|attacker => %s\n", key, firstDrained(drainPlain(1)))
				fmt.Fprintf(&b, "%s|defender => %s\n", key, firstDrained(drainPlain(2)))
				fmt.Fprintf(&b, "%s|spectator => %s\n", key, firstDrained(drainPlain(3)))
			}
		}
	}

	got := b.String()
	path := filepath.Join("testdata", "darkness_narration.golden")
	if *updateDarkness {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		t.Logf("golden rewritten: %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden (run with -update-darkness to record): %v", err)
	}
	if string(want) != got {
		t.Fatalf("darkness narration matrix changed.\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}
