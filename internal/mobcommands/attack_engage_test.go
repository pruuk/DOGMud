package mobcommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mob-versus-mob room line comes from the prepare pool, once per
// reader, and the double-announce guard still holds on a repeat command.
func TestAttackMob_EngageRoomLineComesFromThePreparePool(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	one := func(s string) items.SkillTieredMessages {
		return items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(s)}}
	}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {OptionId: items.Generic, Options: items.AttackTypes{items.Prepare: {Together: items.TogetherMessages{
			ToAttacker: one(`ENGAGE-ACTOR`), ToDefender: one(`ENGAGE-ACTEE`), ToRoom: one(`ENGAGE-ROOM {actor} faces {actee}.`),
		}}}},
	}))

	attacker, room := getTestMobAndRoom(t)
	var targetInstanceId int
	for _, id := range room.GetMobs() {
		if id != attacker.InstanceId {
			targetInstanceId = id
			break
		}
	}
	require.NotZero(t, targetInstanceId, "need a second mob in the room to attack")
	captured, mu, done := captureAnnounces(t)
	defer done()
	target := fmt.Sprintf("#%d", targetInstanceId)

	_, err := Attack(target, attacker, room)
	require.NoError(t, err)
	require.Equal(t, 1, countPerRecipient(t, captured, mu, "ENGAGE-ROOM"))
	assert.Zero(t, countPerRecipient(t, captured, mu, "prepares to fight"))

	_, err = Attack(target, attacker, room)
	require.NoError(t, err)
	assert.Equal(t, 1, countPerRecipient(t, captured, mu, "ENGAGE-ROOM"),
		"re-attacking a mob already being fought must NOT announce a second time")
}
