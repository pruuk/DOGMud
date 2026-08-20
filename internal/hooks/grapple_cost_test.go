package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func setGrappleCostConfig(t *testing.T) configs.Balance {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.GrappleStaminaCostPerRound = 2
	cfg.Balance.GrappleControllerCostMultiplier = 1
	cfg.Balance.GrappleControlledCostMultiplier = 2
	cfg.Balance.GrappleStaminaPenaltyMax = 0
	cfg.Balance.GrappleEncumbrancePenaltyMax = 0
	cfg.Balance.CostSkillMultAtZero = 1.1
	cfg.Balance.CostSkillMultAtMid = 1
	cfg.Balance.CostSkillMultAtCap = 0.4
	cfg.Balance.CostSkillMidRank = 25
	cfg.Balance.CostSkillCapRank = 100
	cfg.Balance.CostEncumbranceKnee = 0.75
	cfg.Balance.CostEncumbranceKneeMult = 1.5
	cfg.Balance.CostEncumbranceMax = 5
	cfg.Balance.CostTotalMultiplierMax = 6
	// U6b Task 14: the drift skill term is SkillWeight (was hardcoded
	// 2.2/2.0) and the aggressor edge is GrappleAggressorDriftBonus.
	// Pin both so score expectations below are deterministic.
	cfg.Balance.SkillWeight = 5
	cfg.Balance.GrappleAggressorDriftBonus = 1.038
	configs.SetConfigForTest(t, cfg)
	return cfg.Balance
}

func grappleCostPair(t *testing.T, controllerID, controlledID int) (*characters.Character, *characters.Character) {
	t.Helper()
	controller := makeGrappleCharacter(t, 100, 100, 25)
	controlled := makeGrappleCharacter(t, 100, 100, 25)
	controller.SetUserId(controllerID)
	controlled.SetUserId(controlledID)
	prepareGrappleCostPair(t, controller, controlled)
	return controller, controlled
}

func prepareGrappleCostPair(t *testing.T, controller, controlled *characters.Character) {
	t.Helper()
	controller.Stamina = 100
	controlled.Stamina = 100
	controller.StaminaMax.Value = 100
	controlled.StaminaMax.Value = 100
	if err := position.TransitionPair(controller, controlled, position.Clinch,
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}); err != nil {
		t.Fatalf("enter Clinch: %v", err)
	}
	if err := position.TransitionPair(controller, controlled, position.Mount,
		state.TransitionReason{Trigger: position.TriggerTakedownMount}); err != nil {
		t.Fatalf("enter Mount: %v", err)
	}
	ctrlData, _ := controller.Position.GrappleData()
	ctrlData.IsAggressor = true
	controller.Position.SetGrappleData(ctrlData)
}

func loadGrapplerToFraction(c *characters.Character, itemID int, fraction float64) {
	if fraction <= 0 {
		return
	}
	weight := c.CarryCapacity() * fraction
	c.Items = []items.Item{{
		ItemId: itemID,
		Spec: &items.ItemSpec{
			ItemId: itemID,
			Name:   "grapple cost test weight",
			Weight: weight,
		},
	}}
}

// Catches allowing a corrupted symmetric grapple to alias both participant
// roles to one Character. The tick must force-break the invalid solo state
// before either role quotes, messages, or rolls.
func TestGrappleMaintenanceRejectsSelfLinkedPairBeforeAdmission(t *testing.T) {
	setGrappleCostConfig(t)
	cases := []struct {
		name    string
		userID  int
		stamina int
	}{
		{name: "funded", userID: 1600, stamina: 3},
		{name: "empty", userID: 1601, stamina: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := users.NewTestUser(tc.userID, tc.name+"-self-grappler", "Self Grappler", uint64(tc.userID))
			cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{tc.userID: u})
			defer cleanupUsers()
			if err := u.Character.Validate(); err != nil {
				t.Fatalf("validate character: %v", err)
			}
			loadGrapplerToFraction(u.Character, 99200+tc.userID, 0.35)
			self := state.ActorRef{UserId: tc.userID}
			if err := u.Character.Position.TransitionToClinch(position.GrappleData{Partner: self},
				state.TransitionReason{Trigger: position.TriggerGrappleEntry}); err != nil {
				t.Fatalf("create corrupted self-linked Clinch: %v", err)
			}
			if partner := resolvePartner(u.Character); partner != u.Character {
				t.Fatalf("fixture partner = %p, want self %p", partner, u.Character)
			}
			u.Character.Stamina = tc.stamina
			snapshot := characters.DriftRollSnapshot{Round: 999, MarginAttacker: 17}
			u.Character.LastDriftRoll = snapshot
			_ = events.DrainQueuedMessagesForTest(tc.userID)

			processGrappleTick(events.NewRound{})

			if u.Character.Position.State() != position.Standing {
				t.Fatalf("self-linked position = %s, want Standing force-break",
					u.Character.Position.State())
			}
			if u.Character.Stamina != tc.stamina {
				t.Fatalf("self-linked tick changed stamina %d -> %d",
					tc.stamina, u.Character.Stamina)
			}
			if u.Character.LastDriftRoll != snapshot {
				t.Fatalf("self-linked tick changed drift snapshot %+v -> %+v",
					snapshot, u.Character.LastDriftRoll)
			}
			if got := shortageMessageCount(events.DrainQueuedMessagesForTest(tc.userID)); got != 0 {
				t.Fatalf("self-linked tick emitted %d shortage messages, want none", got)
			}

			// The invalid tick must not even advance private fractional carry. At
			// this load a fresh 0.5 base remains sub-integer, while the corrupted
			// pair's two role commits would leave enough carry to charge one.
			u.Character.Stamina = 1
			diagnostic := u.Character.CommitCost(u.Character.QuoteActionCost(characters.ActionCostRequest{
				Action: costs.ActionGrappleMaintain,
				Pool:   characters.PoolStamina,
				Base:   0.5,
				Units:  1,
			}), characters.CostPartial)
			if diagnostic.Charged != 0 {
				t.Fatalf("self-linked tick advanced fractional carry: diagnostic charged %d, want 0",
					diagnostic.Charged)
			}
		})
	}
}

// Catches charging a rounded private integer instead of quoting the
// role-adjusted maintenance base through encumbrance and inverse Unarmed.
func TestGrappleMaintenanceQuotesEachRoleAtEveryModeledLoad(t *testing.T) {
	setGrappleCostConfig(t)
	cases := []struct {
		name                           string
		load                           float64
		wantController, wantControlled int
	}{
		{name: "empty", load: 0, wantController: 2, wantControlled: 4},
		{name: "typical", load: 0.35, wantController: 2, wantControlled: 4},
		{name: "knee", load: 0.75, wantController: 3, wantControlled: 6},
		{name: "capacity", load: 1, wantController: 10, wantControlled: 20},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller, controlled := grappleCostPair(t, 1100+i*2, 1101+i*2)
			loadGrapplerToFraction(controller, 99100+i*2, tc.load)
			loadGrapplerToFraction(controlled, 99101+i*2, tc.load)

			processGrapplePair(controller, controlled)

			controllerSpent := 100 - controller.Stamina
			controlledSpent := 100 - controlled.Stamina
			if controllerSpent != tc.wantController || controlledSpent != tc.wantControlled {
				t.Fatalf("spent controller/controlled %d/%d, want %d/%d",
					controllerSpent, controlledSpent, tc.wantController, tc.wantControlled)
			}
			if controlledSpent <= controllerSpent {
				t.Fatalf("controlled spent %d, want more than controller %d",
					controlledSpent, controllerSpent)
			}
		})
	}
}

// Catches resetting fractional carry, rounding each role-adjusted price, or
// multiplying the controlled role after a controller-sized quote has already
// split whole debit from carry. At 35% load the shared encumbrance multiplier
// is 1.2333..., so controller/controlled prices are 2.4666.../4.9333....
func TestGrappleMaintenancePreservesIndependentFractionalCarryAcrossRounds(t *testing.T) {
	setGrappleCostConfig(t)
	controller, controlled := grappleCostPair(t, 1650, 1651)
	loadGrapplerToFraction(controller, 99300, 0.35)
	loadGrapplerToFraction(controlled, 99301, 0.35)

	wantControllerDebit := []int{2, 2, 3, 2, 3, 2}
	wantControlledDebit := []int{4, 5, 5, 5, 5, 5}
	wantControllerSpent := []int{2, 4, 7, 9, 12, 14}
	wantControlledSpent := []int{4, 9, 14, 19, 24, 29}
	previousController := 100
	previousControlled := 100
	for round := range wantControllerDebit {
		processGrapplePairWithContest(controller, controlled, fixedGrappleContest(0))

		controllerDebit := previousController - controller.Stamina
		controlledDebit := previousControlled - controlled.Stamina
		controllerSpent := 100 - controller.Stamina
		controlledSpent := 100 - controlled.Stamina
		if controllerDebit != wantControllerDebit[round] ||
			controlledDebit != wantControlledDebit[round] {
			t.Fatalf("round %d debit controller/controlled %d/%d, want %d/%d",
				round+1, controllerDebit, controlledDebit,
				wantControllerDebit[round], wantControlledDebit[round])
		}
		if controllerSpent != wantControllerSpent[round] ||
			controlledSpent != wantControlledSpent[round] {
			t.Fatalf("round %d cumulative controller/controlled %d/%d, want %d/%d",
				round+1, controllerSpent, controlledSpent,
				wantControllerSpent[round], wantControlledSpent[round])
		}
		previousController = controller.Stamina
		previousControlled = controlled.Stamina
	}
}

func fixedGrappleContest(margin float64) func(float64, []contest.Entry) contest.Result {
	return func(float64, []contest.Entry) contest.Result {
		result := contest.Result{Margin: margin, Contested: true}
		result.AttackRoll.StdDev = 1
		return result
	}
}

// Catches wiring one participant's short result to both scores, or leaving
// Unarmed in the short participant's score.
//
// U6b Task 14: this test previously pinned the hardcoded 2.2/2.0 skill
// coefficients (want 210 = 100 + 2.2×50; want 180 = 100 + 2.0×40) — a
// defect. Now: skill × SkillWeight (5) both sides, and the aggressor
// (the controller here, marked by prepareGrappleCostPair) carries the
// GrappleAggressorDriftBonus (1.038) whole-score multiplier — including
// on a short round, where only the skill term is stripped.
func TestGrappleMaintenanceShortageRemovesOnlyThatParticipantsSkill(t *testing.T) {
	setGrappleCostConfig(t)
	cases := []struct {
		name                                     string
		shortController                          bool
		wantControllerScore, wantControlledScore float64
	}{
		{name: "controller short", shortController: true, wantControllerScore: 100 * 1.038, wantControlledScore: 300},
		{name: "controlled short", shortController: false, wantControllerScore: 350 * 1.038, wantControlledScore: 100},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller, controlled := grappleCostPair(t, 1200+i*2, 1201+i*2)
			controller.Skills[string(skills.UnarmedCombat)] = 50
			controlled.Skills[string(skills.UnarmedCombat)] = 40
			if tc.shortController {
				controller.Stamina = 0
			} else {
				controlled.Stamina = 0
			}

			var gotControllerScore, gotControlledScore float64
			processGrapplePairWithContest(controller, controlled,
				func(attack float64, defenses []contest.Entry) contest.Result {
					gotControllerScore = attack
					if len(defenses) != 1 {
						t.Fatalf("defenses = %d, want one", len(defenses))
					}
					gotControlledScore = defenses[0].Score
					return fixedGrappleContest(0)(attack, defenses)
				})

			if !grappleScoreApproxEqual(gotControllerScore, tc.wantControllerScore) ||
				!grappleScoreApproxEqual(gotControlledScore, tc.wantControlledScore) {
				t.Fatalf("contest scores controller/controlled %.3f/%.3f, want %.3f/%.3f",
					gotControllerScore, gotControlledScore,
					tc.wantControllerScore, tc.wantControlledScore)
			}
		})
	}
}

// Catches charging after outcome dispatch: an escape must not bypass either
// participant's maintenance commit, and partial payment must floor both pools.
//
// U6b Task 14: the injected margin was -2.5 at stdDev 1, which only reached
// the escape band (z ≤ -2.0) through the √2-less z the live code was missing
// (a pinned defect: -2.5/√2 ≈ -1.77 is a reversal, not an escape). -3.0
// normalises to -2.12 under the corrected maths and stays an escape.
func TestGrappleMaintenanceChargesBothBeforeEscapeResolution(t *testing.T) {
	setGrappleCostConfig(t)
	controller, controlled := grappleCostPair(t, 1300, 1301)
	controller.Stamina = 1
	controlled.Stamina = 2
	contestCalled := false

	processGrapplePairWithContest(controller, controlled,
		func(attack float64, defenses []contest.Entry) contest.Result {
			contestCalled = true
			if controller.Stamina != 0 || controlled.Stamina != 0 {
				t.Fatalf("pools at contest controller/controlled %d/%d, want 0/0",
					controller.Stamina, controlled.Stamina)
			}
			return fixedGrappleContest(-3.0)(attack, defenses)
		})

	if !contestCalled {
		t.Fatal("contest runner was not called")
	}
	if controller.Stamina < 0 || controlled.Stamina < 0 {
		t.Fatalf("negative pool after escape: controller/controlled %d/%d",
			controller.Stamina, controlled.Stamina)
	}
	if controller.Position.State() != position.Standing || controlled.Position.State() != position.Standing {
		t.Fatalf("escape did not resolve to Standing: controller/controlled %s/%s",
			controller.Position.State(), controlled.Position.State())
	}
}

func shortageMessageCount(messages []string) int {
	count := 0
	for _, message := range messages {
		if strings.Contains(message, "trained control") {
			count++
		}
	}
	return count
}

// Catches sending the private shortage line to the other grappler or room,
// and catches a reversal path repeating it after roles swap.
func TestGrappleMaintenanceShortageMessageUsesParticipantAudienceOnce(t *testing.T) {
	setGrappleCostConfig(t)
	controllerUser := users.NewTestUser(1400, "grapple-controller", "Controller", 1400)
	controlledUser := users.NewTestUser(1401, "grapple-controlled", "Controlled", 1401)
	observerUser := users.NewTestUser(1402, "grapple-observer", "Observer", 1402)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{
		1400: controllerUser,
		1401: controlledUser,
		1402: observerUser,
	})
	defer cleanupUsers()
	prepareGrappleCostPair(t, controllerUser.Character, controlledUser.Character)
	controllerUser.Character.Stamina = 0
	_ = events.DrainQueuedMessagesForTest(1400)
	_ = events.DrainQueuedMessagesForTest(1401)
	_ = events.DrainQueuedMessagesForTest(1402)

	processGrapplePairWithContest(controllerUser.Character, controlledUser.Character,
		fixedGrappleContest(-1.5))

	if got := shortageMessageCount(events.DrainQueuedMessagesForTest(1400)); got != 1 {
		t.Fatalf("short participant messages = %d, want exactly one", got)
	}
	if got := shortageMessageCount(events.DrainQueuedMessagesForTest(1401)); got != 0 {
		t.Fatalf("other participant received %d shortage messages, want none", got)
	}
	if got := shortageMessageCount(events.DrainQueuedMessagesForTest(1402)); got != 0 {
		t.Fatalf("observer received %d shortage messages, want none", got)
	}
}

// Catches hard-wiring the controller as the only participant eligible for the
// private warning when the controlled fighter is the one who came up short.
func TestGrappleMaintenanceControlledShortageMessageUsesOwnAudienceOnce(t *testing.T) {
	setGrappleCostConfig(t)
	controllerUser := users.NewTestUser(1450, "grapple-controller-2", "Controller", 1450)
	controlledUser := users.NewTestUser(1451, "grapple-controlled-2", "Controlled", 1451)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{
		1450: controllerUser,
		1451: controlledUser,
	})
	defer cleanupUsers()
	prepareGrappleCostPair(t, controllerUser.Character, controlledUser.Character)
	controlledUser.Character.Stamina = 0
	_ = events.DrainQueuedMessagesForTest(1450)
	_ = events.DrainQueuedMessagesForTest(1451)

	processGrapplePairWithContest(controllerUser.Character, controlledUser.Character,
		fixedGrappleContest(0))

	if got := shortageMessageCount(events.DrainQueuedMessagesForTest(1450)); got != 0 {
		t.Fatalf("controller received %d controlled-shortage messages, want none", got)
	}
	if got := shortageMessageCount(events.DrainQueuedMessagesForTest(1451)); got != 1 {
		t.Fatalf("controlled participant messages = %d, want exactly one", got)
	}
}

// Catches fabricating a private player message for NPC maintenance shortage.
func TestGrappleMaintenanceNPCShortageEmitsNoPlayerText(t *testing.T) {
	setGrappleCostConfig(t)
	observer := users.NewTestUser(1500, "grapple-observer", "Observer", 1500)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1500: observer})
	defer cleanupUsers()
	controller := makeGrappleCharacter(t, 100, 100, 25)
	controlled := makeGrappleCharacter(t, 100, 100, 25)
	controller.MobInstanceId = 1501
	controlled.MobInstanceId = 1502
	prepareGrappleCostPair(t, controller, controlled)
	controller.Stamina = 0
	controlled.Stamina = 0
	_ = events.DrainQueuedMessagesForTest(1500)

	processGrapplePairWithContest(controller, controlled, fixedGrappleContest(0))

	if got := shortageMessageCount(events.DrainQueuedMessagesForTest(1500)); got != 0 {
		t.Fatalf("NPC shortage emitted %d private player messages, want none", got)
	}
}
