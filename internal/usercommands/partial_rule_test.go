package usercommands

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

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

	t.Run("defender, against a player", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		bob := users.GetByUserId(2)
		tokens := send(t, bob.Character.Name, bob, bob.UserId)
		requirePartialLine(t, events.DrainQueuedMessagesForTest(2),
			partialRuleVariants(t, tokens, func(r narration.Roles) string { return r.Actee }), "the defender")
	})
}
