package usercommands

// attack_ambush_denial_test.go — U10d, the VOICE of a refused melee ambush.
//
// The ranged half of the surprise opener has told the player when the shared
// special-move timer refused it since Task 14 (shoot_narration_test.go). The
// melee half said nothing at all, and the silence was not merely a missing
// line: SetAggro cascades Hidden -> Revealing whatever the aggro type is
// (internal/hooks/Awareness_Cascades.go), so a refused melee ambusher spent
// their cover, swung ordinarily, and was told neither thing.
//
// Two assertions carry this file, and the SECOND is the one that matters most:
//
//  1. a hidden attacker whose special-move timer is already claimed is told
//     the ambush was refused;
//  2. an ordinary attacker who never hid is NOT told anything of the kind. A
//     naive implementation that speaks whenever the opener is simply absent
//     would announce a failed ambush to every attacker in the game.
//
// Both engagement sites in Attack are covered — player to mob and player to
// player — because they are separate call sites of actions.EngageAggroType and
// a fix applied to only one of them is exactly the shape of bug this pins.

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hideForAmbushTest puts a character into awareness.Hidden.
//
// It drives the Awareness machine and does NOT add buff #9: Character.IsHidden
// reads the machine alone, and the buff registry these tests seed does not
// carry #9. Mirrors addHiddenBuff in internal/actions/shadow_test.go.
func hideForAmbushTest(t *testing.T, c *characters.Character) {
	t.Helper()
	if c.Awareness == nil {
		c.Awareness = awareness.NewMachine()
	}
	r := state.TransitionReason{Trigger: "test_setup"}
	c.Awareness.ForceVisible(r)
	_ = c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r)
	c.Awareness.ResolveConcealment(true, r)
	require.True(t, c.IsHidden(),
		"fixture must actually be hidden, or the ambush is never asked for")
}

// claimSpecialMoveTimer pre-claims the shared cooldown the opener needs, the
// way a recent special move (or a reload, on the ranged side) would have.
func claimSpecialMoveTimer(t *testing.T, c *characters.Character) {
	t.Helper()
	if c.Cooldowns == nil {
		c.Cooldowns = characters.Cooldowns{}
	}
	c.Cooldowns["special-move"] = 5
	require.False(t, c.TryCooldown("special-move", "5 rounds"),
		"fixture must actually deny the timer, or nothing refuses the opener")
	c.Cooldowns["special-move"] = 5
}

// wireRevealCascadeForTest replicates production's Combat Phase to Awareness
// cascade (wireAwarenessFromCombatPhase in internal/hooks) on one character.
//
// It is copied rather than imported because internal/hooks imports
// internal/usercommands, so this package cannot import it back. Without it a
// fixture character keeps their cover through SetAggro and the reveal line is
// correctly withheld, which is itself an assertion below.
func wireRevealCascadeForTest(c *characters.Character) {
	if c.CombatPhase == nil {
		c.CombatPhase = combatphase.NewMachine()
	}
	c.CombatPhase.Inner().AfterTransition("test_awareness_combat_cascade",
		func(from, to combatphase.State, r state.TransitionReason) {
			if from != combatphase.Idle || to != combatphase.Engaging {
				return
			}
			if c.Awareness.State() == awareness.Hidden {
				_ = c.Awareness.TransitionToRevealing(state.TransitionReason{
					Trigger: awareness.TriggerCombatEntered,
				})
			}
		})
}

// attackAndCollect runs Attack and returns everything the attacker was told.
func attackAndCollect(t *testing.T, user *users.UserRecord, room *rooms.Room, target string) string {
	t.Helper()
	user.Character.Aggro = nil
	events.DrainQueuedMessagesForTest(user.UserId) // discard fixture noise

	handled, err := Attack(target, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
}

// ---------------------------------------------------------------------------
// 1. The refusal is spoken
// ---------------------------------------------------------------------------

func TestAttack_HiddenOnCooldown_TellsThePlayerTheAmbushWasRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	hideForAmbushTest(t, user.Character)
	claimSpecialMoveTimer(t, user.Character)

	out := attackAndCollect(t, user, room, "skeleton")

	assert.Contains(t, out, surpriseMeleeDeniedText,
		"a hidden attacker whose special-move timer was already claimed gets an ORDINARY "+
			"swing and loses their cover for it. Saying nothing makes the ambush read as broken")
	require.NotNil(t, user.Character.Aggro, "the attack itself still happens")
	assert.Equal(t, characters.DefaultAttack, user.Character.Aggro.Type,
		"precondition: the opener really was refused, so the line is not a false alarm")
}

// The PvP engagement is a SECOND call site of actions.EngageAggroType, and a
// fix wired into only the mob branch would leave it silent.
func TestAttack_HiddenOnCooldownAgainstAPlayer_TellsThePlayerToo(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	hideForAmbushTest(t, user.Character)
	claimSpecialMoveTimer(t, user.Character)

	out := attackAndCollect(t, user, room, "Bobrick")

	assert.Contains(t, out, surpriseMeleeDeniedText,
		"the player-versus-player engagement path must speak the refusal too")
	require.NotNil(t, user.Character.Aggro)
	assert.Equal(t, 2, user.Character.Aggro.UserId,
		"precondition: the PvP branch is what ran")
}

// ---------------------------------------------------------------------------
// 2. The refusal is NOT spoken to anyone who never asked for an ambush
// ---------------------------------------------------------------------------

// The control that matters. An implementation keying off "the opener is
// absent" rather than "the opener was refused" passes test 1 and fails here,
// having told every ordinary attacker in the game that their ambush failed.
func TestAttack_OrdinaryAttacker_IsNeverToldAnAmbushWasRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	// Deliberately WITH the timer claimed: the only difference from test 1 is
	// that this attacker never hid. If the line keys off the cooldown alone
	// rather than off the cooldown AND stealth, this is where it shows.
	claimSpecialMoveTimer(t, user.Character)
	require.False(t, user.Character.IsHidden(), "precondition: this attacker never hid")

	out := attackAndCollect(t, user, room, "skeleton")

	assert.NotContains(t, out, surpriseMeleeDeniedText,
		"an attacker who never hid asked for no ambush: telling them one was refused would "+
			"reach every ordinary attack in the game")
	assert.NotContains(t, out, surpriseMeleeRevealedText,
		"an attacker who never hid had no cover to lose")
	assert.NotEmpty(t, out,
		"the ordinary attack still narrates, so the checks above are not vacuous")
}

// A GRANTED opener must be silent too: the player is about to land the crit,
// and a refusal line would flatly contradict it.
func TestAttack_HiddenWithFreeTimer_SaysNothingAboutARefusal(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	hideForAmbushTest(t, user.Character)
	user.Character.Cooldowns = characters.Cooldowns{}

	out := attackAndCollect(t, user, room, "skeleton")

	require.NotNil(t, user.Character.Aggro)
	require.Equal(t, characters.SurpriseAttack, user.Character.Aggro.Type,
		"precondition: the opener was GRANTED, so any refusal line would be a lie")
	assert.NotContains(t, out, surpriseMeleeDeniedText,
		"a granted ambush must not be reported as refused")
}

// ---------------------------------------------------------------------------
// 3. The cover line reports what actually happened to the cover
// ---------------------------------------------------------------------------

// Losing stealth for nothing is the real cost of a refused ambush, and it was
// the wholly silent half of the defect. With the production cascade wired, the
// attacker is revealed by SetAggro and must be told.
func TestAttack_RefusedAmbushThatSpentTheCover_SaysTheCoverIsGone(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	wireRevealCascadeForTest(user.Character)
	hideForAmbushTest(t, user.Character)
	claimSpecialMoveTimer(t, user.Character)

	out := attackAndCollect(t, user, room, "skeleton")

	require.False(t, user.Character.IsHidden(),
		"precondition: the cascade ran and the attacker really was revealed")
	assert.Contains(t, out, surpriseMeleeDeniedText, "the refusal is still spoken")
	assert.Contains(t, out, surpriseMeleeRevealedText,
		"the ambush is off but the cover is spent anyway, and that is the more expensive half")
}

// The other direction. SetAggro has paths that return before the Combat Phase
// transition (the grace-period and taunt-hold guards in
// internal/characters/combat_state_compat.go) and paths where it is vetoed; on
// those the attacker keeps their cover. Claiming otherwise would be a lie about
// the one thing the line exists to report. Here the cascade is simply not
// wired, which reproduces that state.
func TestAttack_RefusedAmbushThatKeptTheCover_DoesNotClaimTheCoverIsGone(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	hideForAmbushTest(t, user.Character)
	claimSpecialMoveTimer(t, user.Character)

	out := attackAndCollect(t, user, room, "skeleton")

	require.True(t, user.Character.IsHidden(),
		"precondition: nothing revealed this attacker")
	assert.Contains(t, out, surpriseMeleeDeniedText, "the refusal is still spoken")
	assert.NotContains(t, out, surpriseMeleeRevealedText,
		"an attacker who kept their cover must not be told they lost it")
}

// ---------------------------------------------------------------------------
// 4. The copy rules
// ---------------------------------------------------------------------------

// Same rules shoot_narration_test.go applies to the ranged half: 80 columns,
// no raw numbers, no en or em dashes. renderedWidth lives there and counts
// what a client actually prints.
func TestMeleeAmbushDenialCopy_ObeysThePlayerCopyRules(t *testing.T) {
	lines := map[string]string{
		"denied":   surpriseMeleeDeniedText,
		"revealed": surpriseMeleeRevealedText,
	}

	for name, line := range lines {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, line)
			for _, r := range line {
				if r >= '0' && r <= '9' {
					t.Errorf("copy leaks a raw number: %q", line)
					break
				}
			}
			if strings.ContainsAny(line, "–—") {
				t.Errorf("copy contains an en or em dash: %q", line)
			}
			if w := renderedWidth(line); w > 80 {
				t.Errorf("copy renders %d columns, over the 80-column MUD width: %q", w, line)
			}
		})
	}

	assert.Equal(t, surpriseShotDeniedText, surpriseMeleeDeniedText,
		"one shared cooldown, one wording: the melee refusal is the ranged refusal verbatim, "+
			"so the two halves of the ambush read to a player as one feature")
}
