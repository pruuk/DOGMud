package usercommands

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var partialRuleTags = regexp.MustCompile("\x1b\\[[0-9;]*m|<[^>]*>")

func partialRulePlain(s string) string {
	return strings.Join(strings.Fields(partialRuleTags.ReplaceAllString(s, " ")), " ")
}

// partialRuleVariants renders every variant of one role of the rake store's
// partial event, plain.
func partialRuleVariants(t *testing.T, tokens map[string]string, role func(narration.Roles) string) []string {
	t.Helper()
	g := movenarration.GetMove("rake")
	require.NotNil(t, g)
	v, ok := g.Variants("partial")
	require.True(t, ok)
	var out []string
	for i := 0; i < v.Len(); i++ {
		out = append(out, partialRulePlain(role(narration.Render(v, tokens, nil, i))))
	}
	return out
}

func requirePartialLine(t *testing.T, got []string, want []string, who string) {
	t.Helper()
	joined := partialRulePlain(strings.Join(got, " "))
	for _, w := range want {
		if w != "" && strings.Contains(joined, w) {
			return
		}
	}
	t.Fatalf("%s read %q, not one of the partial lines %q", who, joined, want)
}

// #631, owner ruling 2026-10-19: a player's rake that a defence blunted but
// did not stop reads the rake store's partial tier for the attacker, the
// defender and the room alike. The mobcommands twin pins the mob attacker.
func TestPlayerPartialMove_EveryRoleReadsThePartialTier(t *testing.T) {
	require.NoError(t, movenarration.LoadFrom("../../_datafiles/world/dogmud/narration/special-moves"))

	send := func(t *testing.T, acteeName string, actee messaging.Recipient, acteeId int) map[string]string {
		alice := users.GetByUserId(1)
		room := rooms.LoadRoom(1)
		require.NotNil(t, room)
		events.DrainQueuedMessagesForTest(1)
		events.DrainQueuedMessagesForTest(2)
		ids := moveIdentities{
			Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, alice.Character.Name),
			ActorPlain: alice.Character.Name,
			Actee:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, acteeName),
			ActeePlain: acteeName,
		}
		aud := messaging.Audience{Actor: alice, ActorId: alice.UserId, ActorName: alice.Character.Name,
			Actee: actee, ActeeId: acteeId, ActeeName: acteeName, Room: room}
		sendPartialMove("rake", "partial", ids, aud, rakeCategories, map[string]string{movenarration.TokenDamage: "a scratch"}, "")
		return map[string]string{
			narration.TokenActor: ids.Actor, narration.TokenActorPlain: ids.ActorPlain,
			narration.TokenActee: ids.Actee, narration.TokenActeePlain: ids.ActeePlain,
			movenarration.TokenDamage: "a scratch",
		}
	}

	t.Run("attacker and room, against a mob", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		skeleton := mobs.GetInstance(100)
		require.NotNil(t, skeleton)
		tokens := send(t, skeleton.Character.Name, nil, 0)
		requirePartialLine(t, events.DrainQueuedMessagesForTest(1),
			partialRuleVariants(t, tokens, func(r narration.Roles) string { return r.Actor }), "the attacker")
		requirePartialLine(t, events.DrainQueuedMessagesForTest(2),
			partialRuleVariants(t, tokens, func(r narration.Roles) string { return r.Observer }), "the room")
	})

	t.Run("firebomb: thrower and room, through the real Throw", func(t *testing.T) {
		user, room, targets := seedThrowSeamFixture(t, 1, 0, 0)
		room.Lamp = rooms.LampPtr(60)
		witness := users.NewTestUser(79064, "witness", "Witness", 0)
		witness.Character.RoomId = room.RoomId
		t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{user.UserId: user, witness.UserId: witness}))
		room.AddPlayer(user.UserId)
		room.AddPlayer(witness.UserId)
		require.Equal(t, messaging.SightFull, messaging.ParticipantSight(witness.Character, room))
		// A bare defensive win: the defence takes most of the blast and the
		// splash still clips the target.
		restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
			return throwSeamResult(atkScore, entries, -0.5, 0.5)
		})
		defer restore()
		events.DrainQueuedMessagesForTest(user.UserId)
		events.DrainQueuedMessagesForTest(witness.UserId)

		_, err := Throw("bomb", user, room, 0)
		require.NoError(t, err)
		require.Less(t, targets[0].Character.Health, 500, "fixture: the splash must clip the defended target")

		g := movenarration.GetMove("throw")
		require.NotNil(t, g)
		v, ok := g.Variants("partial_hit")
		require.True(t, ok)
		tokens := map[string]string{
			narration.TokenActor: `<ansi fg="username">Thrower</ansi>`, narration.TokenActorPlain: "Thrower",
			narration.TokenActee: `<ansi fg="mobname">Target</ansi>`, narration.TokenActeePlain: "Target",
		}
		var actor, observer []string
		for i := 0; i < v.Len(); i++ {
			r := narration.Render(v, tokens, nil, i)
			// The damage words vary with the roll; match the text before them.
			actor = append(actor, partialRulePlain(strings.SplitN(r.Actor, "(", 2)[0]))
			observer = append(observer, partialRulePlain(r.Observer))
		}
		requirePartialLine(t, events.DrainQueuedMessagesForTest(user.UserId), actor, "the thrower")
		seen := events.DrainQueuedMessagesForTest(witness.UserId)
		requirePartialLine(t, seen, observer, "the room")
		require.NotContains(t, partialRulePlain(strings.Join(seen, " ")), "dodge",
			"the room must not read the defence triad on a partial")
	})

	t.Run("defender, against a player", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		bob := users.GetByUserId(2)
		tokens := send(t, bob.Character.Name, bob, bob.UserId)
		requirePartialLine(t, events.DrainQueuedMessagesForTest(2),
			partialRuleVariants(t, tokens, func(r narration.Roles) string { return r.Actee }), "the defender")
	})
}
