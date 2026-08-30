package mobcommands

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureAnnounces collects every events.Message so a test can count how many
// times a given line reached the room.
func captureAnnounces(t *testing.T) (*[]events.Message, *sync.Mutex, func()) {
	t.Helper()
	var mu sync.Mutex
	captured := []events.Message{}
	id := events.RegisterListener(events.Message{}, func(e events.Event) events.ListenerReturn {
		if m, ok := e.(events.Message); ok {
			mu.Lock()
			captured = append(captured, m)
			mu.Unlock()
		}
		return events.Continue
	})
	return &captured, &mu, func() {
		events.UnregisterListener(events.Message{}, id)
	}
}

// countPerRecipient returns, for the given line, the HIGHEST number of copies
// any single user received.
//
// ⚠️ Counting raw events is WRONG here and produced a false positive on the
// first draft of this test: Room.SendTextVisual emits one events.Message PER
// PLAYER in the room (rooms.go:333), so a single broadcast into a two-player
// room is legitimately two events. What a player actually complains about is
// seeing the same line twice THEMSELVES.
func countPerRecipient(t *testing.T, captured *[]events.Message, mu *sync.Mutex, needle string) int {
	t.Helper()
	events.ProcessEvents()
	mu.Lock()
	defer mu.Unlock()
	perUser := map[int]int{}
	for _, m := range *captured {
		if strings.Contains(m.Text, needle) {
			perUser[m.UserId]++
		}
	}
	worst := 0
	for _, n := range perUser {
		if n > worst {
			worst = n
		}
	}
	return worst
}

// ⚠️ REGRESSION: a mob re-issuing `attack` against a mob it is ALREADY fighting
// must not re-announce "prepares to fight".
//
// The player-target branch of Attack has always guarded this with an
// `alreadyFighting` check. The mob-target branch had NO such guard, so every
// repeat command announced again.
//
// ⚠️ `alreadyFighting` is the load-bearing half, verified by neutering each
// condition separately: dropping it fails this test, dropping the `engaged`
// check does not. targeting.Commit returns true when re-committing a target
// already held, so `engaged` alone does NOT suppress a repeat. It is kept
// because the player branch has it (U12c-0b: a vetoed commit must not announce
// a fight nobody is having), which this test does not cover.
//
// That is the companion assist double. A companion assists its owner by
// attacking the ENEMY MOB, so it goes down the mob-vs-mob branch, and two
// systems command it: the reactive path in CombatPhase_CompanionAssist.go and
// the polling handleCompanionOwnerAssist. TryClaimAssistCommand dedupes them
// WITHIN a round, but the reactive path fires a round earlier, so the second
// command lands in the next round with a fresh claim -- exactly the "two
// consecutive rounds" case that Character.AssistCommandedRound's own comment
// describes. The command is harmless; the unguarded announce is what the player
// sees twice.
//
// lookfortrouble.go carries a comment describing the same shape: commands that
// bounce harmlessly while "the 'prepares to fight' message is still sent each
// time".
func TestAttackMob_DoesNotReAnnounceAgainstSameMobTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	attacker, room := getTestMobAndRoom(t)

	// A second mob in the same room to be the target.
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

	handled, err := Attack(target, attacker, room)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, countPerRecipient(t, captured, mu, "prepares to fight"),
		"the first attack must announce exactly once per player")

	// Same command again, same target, already fighting it.
	handled, err = Attack(target, attacker, room)
	require.True(t, handled)
	require.NoError(t, err)

	assert.Equal(t, 1, countPerRecipient(t, captured, mu, "prepares to fight"),
		"re-attacking a mob already being fought must NOT announce a second time")
}
