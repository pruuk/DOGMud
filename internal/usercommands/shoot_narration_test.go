package usercommands

// U10d Task 14 — the VOICE of the ranged half of the surprise-attack redesign.
//
// ExecuteFire already sets Revealed, SurpriseOnCooldown and AimedWhileEngaged;
// until this task nothing spoke them. These tests pin the copy rules the
// project applies to every player-facing line, and the once-per-engagement
// latch behind the engaged-aim cue.

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShootNarrationCopy_ObeysThePlayerCopyRules checks the three rules a
// reviewer cannot see by reading the diff: no raw numbers, no en/em dashes, and
// a rendered width that fits an 80-column client. The messaging pipeline's wrap
// stage is off (shouldWrap returns false for every Category), so an over-long
// line is handed to the client exactly as authored.
func TestShootNarrationCopy_ObeysThePlayerCopyRules(t *testing.T) {
	lines := map[string]string{
		"revealed": surpriseShotRevealedText,
		"denied":   surpriseShotDeniedText,
		"engaged":  aimedWhileEngagedText,
		// The banner plus the longest surprise-shot line it prefixes, with a
		// plausible name and damage band substituted, so the composed line is
		// measured rather than its pieces.
		"banner+hit": surpriseShotBanner + `Your shot takes <ansi fg="mobname">a tomb skeleton</ansi> unaware! (<ansi fg="damage">serious wounds</ansi>)`,
	}

	for name, line := range lines {
		t.Run(name, func(t *testing.T) {
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
				t.Errorf("copy renders %d columns wide, want at most 80: %q", w, line)
			}
		})
	}
}

// renderedWidth counts the characters a client actually prints: ANSI tags are
// markup and occupy no columns.
func renderedWidth(line string) int {
	n, inTag := 0, false
	for _, r := range line {
		switch {
		case r == '<':
			inTag = true
		case r == '>' && inTag:
			inTag = false
		case !inTag:
			n++
		}
	}
	return n
}

// The latch truth table. speak fires on the FIRST engaged shot of a run and
// never again until a shot goes off clear (or EndAggro clears the field).
func TestShouldSpeakEngagedCue_TruthTable(t *testing.T) {
	cases := []struct {
		engaged, already bool
		wantSpeak        bool
		wantStored       bool
	}{
		{engaged: true, already: false, wantSpeak: true, wantStored: true},
		{engaged: true, already: true, wantSpeak: false, wantStored: true},
		{engaged: false, already: true, wantSpeak: false, wantStored: false},
		{engaged: false, already: false, wantSpeak: false, wantStored: false},
	}
	for _, tc := range cases {
		speak, stored := shouldSpeakEngagedCue(tc.engaged, tc.already)
		if speak != tc.wantSpeak || stored != tc.wantStored {
			t.Errorf("shouldSpeakEngagedCue(%v, %v) = (%v, %v), want (%v, %v)",
				tc.engaged, tc.already, speak, stored, tc.wantSpeak, tc.wantStored)
		}
	}
}

// Driven as a sequence, which is what "once per engagement" actually means:
// four engaged shots in a row speak once, and a clear shot re-arms the cue.
func TestShouldSpeakEngagedCue_SpeaksOncePerRunOfEngagedShots(t *testing.T) {
	shots := []bool{true, true, true, false, true, true}
	spoken, stored := 0, false
	for _, engaged := range shots {
		var speak bool
		speak, stored = shouldSpeakEngagedCue(engaged, stored)
		if speak {
			spoken++
		}
	}
	if spoken != 2 {
		t.Fatalf("the cue fired %d times across %v, want 2 (once per run of engaged shots)", spoken, shots)
	}
}

// EndAggro is the other end of the engagement. Without this the field would
// stay latched for the rest of the session and a shooter who heard the cue in
// one fight would never hear it in the next.
func TestEndAggro_ClearsTheEngagedCueLatch(t *testing.T) {
	c := characters.New()
	c.RangedEngagedCueSpoken = true
	c.EndAggro()
	if c.RangedEngagedCueSpoken {
		t.Fatal("EndAggro must clear RangedEngagedCueSpoken: the engagement is over")
	}
}

// The wiring, driven through the real command. A shot taken while a mob has the
// shooter as its aggro target latches the cue; a shot taken with the room clear
// releases it.
func TestShoot_EngagedCueLatchesThroughTheRealCommand(t *testing.T) {
	pinContestFloorOff(t)

	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Stats.Strength.ValueAdj = 1
	user.Character.Aggro = nil
	user.Character.RangedEngagedCueSpoken = false
	equipBow(user.Character, true)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.Health = 100000
	mob.Character.HealthMax.Value = 100000

	// Something in the room is on the shooter: no unengaged bonus, so the cue
	// must be spoken and latched.
	mob.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
	require.NotNil(t, mob.Character.Aggro, "fixture precondition: the mob must be aggroed on the shooter")

	handled, err := Shoot("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	assert.True(t, user.Character.RangedEngagedCueSpoken,
		"a shot taken while something is on the shooter must latch the engaged-aim cue")

	// Now clear the room and shoot again: the latch must release so the next
	// time the shooter is pinned down the cue is news again.
	clearRoomAggro(t, room)
	user.Character.Aggro = nil
	equipBow(user.Character, true)

	handled, err = Shoot("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	assert.False(t, user.Character.RangedEngagedCueSpoken,
		"a shot taken with nothing on the shooter must release the latch")
}

// clearRoomAggro drops every mob in the room out of combat, so
// shooterIsUnengaged's room scan finds nothing targeting the shooter.
func clearRoomAggro(t *testing.T, room *rooms.Room) {
	t.Helper()
	for _, instId := range room.GetMobs() {
		if m := mobs.GetInstance(instId); m != nil {
			m.Character.EndAggro()
		}
	}
}
