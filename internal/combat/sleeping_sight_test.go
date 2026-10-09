package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// #455 review (K8): a sleeping player is attacked (the sleeping-victim
// auto-crit), and the first hit's personal line reached them while they
// were still asleep. The prompt and GMCP judged the sleeper at
// messaging.ReaderSight (sleep sees nothing); the combat line judged them
// at ParticipantSight (optics only) and named the attacker. The combat
// narration gate now carries the sleep test too.
func TestCombatContext_ASleeperSeesNothingInALitRoom(t *testing.T) {
	lit := verdictLight(100)
	attacker := verdictObserver(t, false, false, false, false)
	sleeper := verdictObserver(t, false, false, true, false)

	ctx := newCombatContext(attacker, sleeper, lit, true)
	if ctx.targetSight != messaging.SightNone {
		t.Fatalf("sleeping defender's sight = %v, want SightNone", ctx.targetSight)
	}
	if ctx.sourceSight != messaging.SightFull {
		t.Fatalf("awake attacker's sight = %v, want SightFull (control)", ctx.sourceSight)
	}

	// The comfort distances that score the contest stay optics only: sleep
	// must not hand the sleeper a darkness term (CanSeeSightImpairedOnly).
	awake := newCombatContext(attacker, verdictObserver(t, false, false, false, false), lit, true)
	if ctx.targetDark != awake.targetDark || ctx.targetBright != awake.targetBright {
		t.Fatalf("sleep moved the comfort distances: asleep (%v,%v) awake (%v,%v)",
			ctx.targetDark, ctx.targetBright, awake.targetDark, awake.targetBright)
	}

	// The real personal line: the sleeper reads no attacker name.
	result := buildDeflectedSwing(t, "Grimwald", "Shade")
	if !strings.Contains(result.MessagesToTarget[0].Text, "Grimwald") {
		t.Fatalf("control: the composite never named the attacker: %q", result.MessagesToTarget[0].Text)
	}
	hideIdentitiesInPersonalLines(result, &characters.Character{Name: "Grimwald"}, &characters.Character{Name: "Shade"}, ctx)
	if got := result.MessagesToTarget[0].Text; strings.Contains(got, "Grimwald") {
		t.Fatalf("the sleeping defender reads the attacker's name: %q", got)
	}
}
