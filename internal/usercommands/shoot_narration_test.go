package usercommands

// U10d Task 14 — the VOICE of the ranged half of the surprise-attack redesign.
//
// ExecuteFire already sets Revealed, SurpriseOnCooldown and AimedWhileEngaged;
// until this task nothing spoke them. These tests pin the copy rules the
// project applies to every player-facing line, the branch order of the
// shot-from-cover narration, and the once-per-engagement latch behind the
// engaged-aim cue.

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderedWidth counts the columns a client actually prints: ANSI tags are
// markup and occupy none.
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

// The worst realistic substitution these lines see.
//
// wideName is 20 characters, the p90 of the authored mob names (median 13, max
// 29); it arrives ANSI-tagged because that is how sendShootMessages builds it.
// widestBand is the longest string combat.GetDamageDescription returns.
// Measuring with a 5-character name would pass copy that overflows in play.
const (
	wideTargetPlain = "a Carrion Highland-H"
	wideTargetTag   = `<ansi fg="mobname">a Carrion Highland-H</ansi>`
	widestBand      = "devastating wounds"
)

// TestShootNarrationCopy_ObeysThePlayerCopyRules covers EVERY authored constant
// and EVERY composition the shooter can receive, not a sample of them. The
// first version of this guard measured one composition and missed two.
func TestShootNarrationCopy_ObeysThePlayerCopyRules(t *testing.T) {
	// A plausible defence triad line, the same shape
	// combat.RenderChannelDefenceMessages produces for an aimed shot.
	triad := `Your aimed shot is dodged by ` + wideTargetTag + `!`

	lines := map[string]string{
		"revealed":         surpriseShotRevealedText,
		"denied":           surpriseShotDeniedText,
		"engaged":          aimedWhileEngagedText,
		"from-cover/hit":   surpriseShotShooterLine(true, "", wideTargetTag, widestBand, true),
		"from-cover/miss":  surpriseShotShooterLine(false, "", wideTargetTag, "", false),
		"from-cover/triad": surpriseShotShooterLine(false, triad, wideTargetTag, widestBand, true),
		// The stopped shot: triad alone, no band appended.
		"from-cover/stopped": surpriseShotShooterLine(false, triad, wideTargetTag, "", false),
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
			// The triad itself is pre-existing copy owned by another package;
			// only measure the compositions this task authored.
			if strings.HasPrefix(name, "from-cover/triad") || strings.HasPrefix(name, "from-cover/stopped") {
				if w := renderedWidth(line) - renderedWidth(triad); w > 80 {
					t.Errorf("the band this task appends adds %d columns: %q", w, line)
				}
				return
			}
			if w := renderedWidth(line); w > 80 {
				t.Errorf("copy renders %d columns wide, want at most 80: %q", w, line)
			}
		})
	}
}

// THE REGRESSION TEST for the case-shadowing bug.
//
// combat.SkillMoveResult.Hit is `!Defended`, so a defended shot that still drew
// blood is `hit == false, dealtDamage == true`. The ordinary shot arms test
// that combination BEFORE the defence triad, deliberately; copying that order
// onto the ambush path meant the shooter was told "your shot only clips them"
// and never told what stopped it, on the one shot where that matters most.
//
// Built to FAIL if the `hit` and `triad` arms are reordered, or if the triad
// arm is moved below a damage-carrying one.
func TestSurpriseShotShooterLine_DefendedPartialNamesTheDefence(t *testing.T) {
	const triad = `Your aimed shot is parried aside!`

	t.Run("defended and drew blood: the defence is named AND the band is carried", func(t *testing.T) {
		got := surpriseShotShooterLine(false, triad, "Selka", "light wounds", true)
		if !strings.Contains(got, triad) {
			t.Fatalf("a defended-partial ambush must name the defence that stopped it, got %q", got)
		}
		if !strings.Contains(got, "light wounds") {
			t.Errorf("a defended-partial ambush must still carry the damage band, got %q", got)
		}
		if strings.Contains(got, "goes wide") || strings.Contains(got, "unaware") {
			t.Errorf("a defended-partial ambush took a hit-or-miss arm: %q", got)
		}
	})

	t.Run("defended and stopped dead: the defence is named, no band", func(t *testing.T) {
		got := surpriseShotShooterLine(false, triad, "Selka", "negligible damage", false)
		if got != triad {
			t.Fatalf("a stopped ambush must be the bare triad, got %q", got)
		}
		if strings.Contains(got, "negligible damage") {
			t.Error("a shot stopped dead must not append a damage band")
		}
	})

	t.Run("clean hit outranks everything", func(t *testing.T) {
		got := surpriseShotShooterLine(true, triad, "Selka", "light wounds", true)
		if !strings.Contains(got, "unaware") {
			t.Fatalf("a landed ambush must use the from-cover hit line, got %q", got)
		}
		if strings.Contains(got, triad) {
			t.Error("a landed ambush must not narrate a defence that did not happen")
		}
	})

	t.Run("clean miss: no triad, no band", func(t *testing.T) {
		got := surpriseShotShooterLine(false, "", "Selka", "light wounds", false)
		if !strings.Contains(got, "goes wide") {
			t.Fatalf("a missed ambush must use the from-cover miss line, got %q", got)
		}
		if strings.Contains(got, "light wounds") {
			t.Error("a clean miss must not carry a damage band")
		}
	})
}

// Every from-cover line must read as a shot from cover, or the whole point of
// the branch is lost and it is indistinguishable from an ordinary shot.
func TestSurpriseShotShooterLine_IsDistinctFromAnOrdinaryShot(t *testing.T) {
	for _, got := range []string{
		surpriseShotShooterLine(true, "", "Selka", "light wounds", true),
		surpriseShotShooterLine(false, "", "Selka", "", false),
	} {
		if !strings.Contains(got, "from cover") {
			t.Errorf("line does not read as a shot from cover: %q", got)
		}
	}
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
