package mobcommands

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner rule (2026-10-05): a line announcing a change is judged against the
// state BEFORE it resolves. Smother blinds its target in the same tick as the
// hit, so a target who watched the Skeleton strike must read its name in the
// hit and cast_interrupt lines, not "Something". The scene: Aliceia (user 1)
// sees the lit room clearly, then is Blinded, as smother leaves her when its
// lines go out; the lines are sent the way smother.go sends them.
func TestSmotherActeeLinesAreJudgedAtTheSightBeforeTheSmoke(t *testing.T) {
	if err := movenarration.LoadFrom("../../_datafiles/world/dogmud/narration/special-moves"); err != nil {
		t.Fatalf("load special-move narration: %v", err)
	}
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	alice := users.GetByUserId(1)
	alice.Character.Perception = characters.New().Perception
	defer func() { alice.Character.Perception = characters.New().Perception }()

	// What smother captures as ActeeSightBefore: the sight before the hit.
	before := messaging.ParticipantSight(alice.Character, room)
	require.Equal(t, messaging.SightFull, before, "fixture: Aliceia must see the lit room clearly before the smoke")

	// The hit blinds her.
	require.NoError(t, alice.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	now := messaging.ParticipantSight(alice.Character, room)
	require.Equal(t, messaging.SightNone, now, "fixture: the smoke must leave Aliceia blind")

	mobName := mob.Character.Name
	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mobName),
		ActorPlain: mobName,
		Actee:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, alice.Character.Name),
		ActeePlain: alice.Character.Name,
	}
	aud := messaging.Audience{ActorName: mobName, Actee: alice, ActeeId: alice.UserId,
		ActeeName: alice.Character.Name, Room: room}
	damage := map[string]string{movenarration.TokenDamage: "a scratch"}

	readHit := func(sight messaging.SightDecision) string {
		events.DrainQueuedMessagesForTest(1)
		roles, ok := renderMoveEvent("smother", "hit", ids, damage)
		require.True(t, ok)
		sendMoveRolesJudgedBefore(roles, aud, messaging.CategoryHitNaturalSharp, sight)
		return strings.Join(mobSpeechHeard(1), "\n")
	}
	readInterrupt := func(sight messaging.SightDecision) string {
		events.DrainQueuedMessagesForTest(1)
		roles, ok := renderMoveEvent("smother", "cast_interrupt", ids, nil)
		require.True(t, ok)
		sendMoveRolesJudgedBefore(roles, aud, messaging.CategorySpellDisruption, sight, messaging.SoundChantBreaksOff)
		return strings.Join(mobSpeechHeard(1), "\n")
	}

	// Several draws, so every variant that names the actor is exercised.
	for i := 0; i < 20; i++ {
		hit := readHit(before)
		require.NotEmpty(t, hit, "the actee must be sent the hit line")
		require.Contains(t, hit, mobName, "a target who saw the strike reads who struck: %q", hit)
		require.NotContains(t, hit, "Something", "%q", hit)
	}
	sawNamed := false
	for i := 0; i < 20; i++ {
		line := readInterrupt(before)
		require.NotEmpty(t, line, "the actee must be sent the cast_interrupt line")
		require.NotContains(t, line, "Something", "%q", line)
		sawNamed = sawNamed || strings.Contains(line, mobName)
	}
	require.True(t, sawNamed, "the cast_interrupt variant that names the actor never named it")

	// Null probe: judged at her sight NOW, the same lines name "Something".
	// This is the defect the before-sight fixes, and it shows the check above
	// can fail.
	sawHidden := false
	for i := 0; i < 20; i++ {
		hit := readHit(now)
		require.NotContains(t, hit, mobName, "judged blind, the hit line must hide the actor: %q", hit)
		sawHidden = sawHidden || strings.Contains(hit, "Something")
	}
	require.True(t, sawHidden, "judged blind, the hit line never read Something")

	// And smother.go passes the before-sight for both events.
	raw, err := os.ReadFile("smother.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, `sendMoveRolesJudgedBefore(roles, aud, messaging.CategoryHitNaturalSharp, res.ActeeSightBefore)`)
	require.Contains(t, src, `sendMoveRolesJudgedBefore(roles, aud, messaging.CategorySpellDisruption, res.ActeeSightBefore, messaging.SoundChantBreaksOff)`)
	require.NotContains(t, src, `sendMoveEvent("smother", "hit"`, "the hit line must not be judged at the target's sight now")
	require.NotContains(t, src, `sendMoveEvent("smother", "cast_interrupt"`, "the cast_interrupt line must not be judged at the target's sight now")
}
