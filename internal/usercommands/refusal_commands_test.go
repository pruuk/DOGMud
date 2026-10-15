package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refusalRaw is everything userId was sent since the last drain, tags intact.
func refusalRaw(userId int) string {
	return strings.Join(events.DrainQueuedMessagesForTest(userId), "")
}

// Every special move verb is a registered command, and its refusal lines
// fit the 80 column rule: the cooldown line for every verb, the no-target
// line for the verbs whose command file calls refusal.NoTarget. This checks
// registration and width only, not which command sends which line.
func TestRefusalSpecialMoveVerbs(t *testing.T) {
	noTarget := []string{"kick", "drain", "gore", "grapple", "maul", "pounce", "rake", "taunt", "throttle", "trip"}
	cooldownOnly := []string{"bash", "rally", "throw", "warcry"}
	for _, verb := range append(append([]string{}, noTarget...), cooldownOnly...) {
		_, ok := userCommands[verb]
		assert.True(t, ok, "%s is not a registered command", verb)
		assert.LessOrEqual(t, util.VisibleWidth(refusal.CooldownText(verb)), 80, "%s cooldown line", verb)
	}
	for _, verb := range noTarget {
		assert.LessOrEqual(t, util.VisibleWidth(refusal.NoTargetText(verb)), 80, "%s no-target line", verb)
	}
}

func TestRefusalKickNoTargetAndCooldownLines(t *testing.T) {
	// aimScene reseeds the species registry, so the legged species goes in
	// after each scene is built (a legless kicker is refused as no target).
	legged := func(user *users.UserRecord) {
		t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
			refusalLeggedSpeciesId: {SpeciesId: refusalLeggedSpeciesId, Name: "walker", BodyParts: []string{"arms", "hands", "legs"}},
		}))
		user.Character.SpeciesId = refusalLeggedSpeciesId
	}

	// No target: in combat with a mob that is gone ("the target is gone").
	user, room := aimScene(t, aimFull)
	legged(user)
	require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 987654}, targeting.ReasonAttack))
	events.DrainQueuedMessagesForTest(1)
	_, err := Kick("", user, room, events.EventFlag(0))
	require.NoError(t, err)
	raw := refusalRaw(1)
	assert.Contains(t, raw, `fg="refusal"`)
	assert.Contains(t, raw, refusal.NoTargetText("kick"))
	targeting.Release(user.Character, targeting.ReasonAttack)

	// Cooldown: a live target and the shared special-move timer already running.
	user, room = aimScene(t, aimFull)
	legged(user)
	require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
	require.True(t, actions.ClaimSpecialMove(user.Character))
	t.Cleanup(func() {
		actions.ReleaseSpecialMove(user.Character)
		targeting.Release(user.Character, targeting.ReasonAttack)
	})
	events.DrainQueuedMessagesForTest(1)
	_, err = Kick("", user, room, events.EventFlag(0))
	require.NoError(t, err)
	raw = refusalRaw(1)
	assert.Contains(t, raw, `fg="refusal"`)
	assert.Contains(t, raw, refusal.CooldownText("kick"))
}

const refusalLeggedSpeciesId = 8291

// A name the player typed is repeated plain: no markup survives, and the
// line rides the refusal category.
func TestRefusalDropNotCarryingEchoesPlainName(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.Items = nil
	sentTo(user)

	_, err := Drop(`<ansi fg="red">zzyzx</ansi>`, user, room, 0)
	require.NoError(t, err)
	raw := refusalRaw(1)
	assert.Contains(t, raw, `fg="refusal"`)
	assert.Contains(t, raw, `'zzyzx'`)
	assert.NotContains(t, raw, `<ansi fg="red"`)
}

func TestRefusalDropNotCarryingCapsALongName(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.Items = nil
	sentTo(user)

	_, err := Drop(strings.Repeat("q", 60), user, room, 0)
	require.NoError(t, err)
	told := aimTagPattern.ReplaceAllString(refusalRaw(1), "")
	assert.Contains(t, told, "...")
	assert.NotContains(t, told, strings.Repeat("q", 30))
}

// A party member who is not the leader is told so by the leader-only
// subcommands; a player in no guild is told so by the guild subcommands.
func TestRefusalMembership(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	for _, uid := range []int{1, 2} {
		if p := parties.Get(uid); p != nil {
			p.Disband()
		}
	}
	p := parties.New(2)
	t.Cleanup(func() { p.Disband() })
	require.True(t, p.InvitePlayer(1))
	require.True(t, p.AcceptInvite(1))
	require.True(t, p.IsMember(1))
	require.False(t, p.IsLeader(1))

	for sub, action := range map[string]string{
		"disband":   "disband the party",
		"kick bob":  "remove members",
		"promote x": "promote a member",
	} {
		events.DrainQueuedMessagesForTest(1)
		handled, err := Party(sub, user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		raw := refusalRaw(1)
		assert.Contains(t, aimTagPattern.ReplaceAllString(raw, ""), refusal.NotPartyLeaderText(action), sub)
		assert.Contains(t, raw, `fg="refusal"`, sub)
	}

	for _, sub := range []string{"info", "leave", "kick bob"} {
		events.DrainQueuedMessagesForTest(1)
		_, err := Guild(sub, user, room, 0)
		require.NoError(t, err)
		raw := refusalRaw(1)
		assert.Contains(t, raw, `fg="refusal"`, "guild "+sub)
		assert.Contains(t, aimTagPattern.ReplaceAllString(raw, ""), aimTagPattern.ReplaceAllString(refusal.NotInGuildText(), ""), "guild "+sub)
	}
}

// give 0 gold is a player's mistake, not a server fault: the how-much line on
// the refusal category, nothing on the error category, no gold moves.
func TestRefusalGiveZeroGold(t *testing.T) {
	const line = `Give how much gold? Try <ansi fg="command">give <amount> gold <name></ansi>.`
	for _, who := range []string{"bobrick", "skeleton"} {
		t.Run(who, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			user, room := getTestUserAndRoom(t)
			other := users.GetByUserId(2)
			user.Character.Gold = 10
			other.Character.Gold = 5
			events.DrainQueuedMessagesForTest(1)

			handled, err := Give("0 gold "+who, user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)
			raw := refusalRaw(1)
			assert.Contains(t, raw, `fg="refusal"`)
			assert.Contains(t, raw, line)
			assert.NotContains(t, raw, `fg="error"`)
			assert.NotContains(t, raw, refusal.FailedText(""))
			assert.Equal(t, 10, user.Character.Gold)
			assert.Equal(t, 5, other.Character.Gold)
		})
	}
}
