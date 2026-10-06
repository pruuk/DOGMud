package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
)

// #415: a defence line names a creature, not its status. The formatted name
// carries an adjective span ("(dead)", "(friend)") for room listings; in a
// narrated line it read "Arena Champion (dead) sets their jaw". Melee and
// counter lines name through meleeIdentityTag, channel lines (taunt, spell,
// ranged) through RenderChannelDefenceMessages: both drop the span and keep
// the identity tag, so sight-based name hiding still finds the name.
func TestDefenceLinesNameWithoutAdjectives(t *testing.T) {
	dead := characters.New()
	dead.Name = "arena champion"
	dead.MobInstanceId = 77
	dead.Health = 0
	full := dead.GetMobName(0).String()
	if !strings.Contains(full, `black-bold">(`) {
		t.Fatalf("precondition: a dead mob's formatted name carries an adjective span, got %q", full)
	}

	if got := meleeIdentityTag(dead); strings.Contains(got, `black-bold">(`) || !strings.Contains(got, `<ansi fg="mobname`) {
		t.Errorf("meleeIdentityTag = %q, want the tagged name with no adjective span", got)
	}

	out := ChannelDefenceResult{Defended: true, Defence: combatvocab.DefenceDefy}
	triad := RenderChannelDefenceMessages(out, ChannelDefenceIdentities{Attacker: full, Defender: full}, "taunt")
	for role, line := range map[string]string{"attacker": string(triad.ToAttacker), "defender": string(triad.ToDefender), "room": string(triad.ToRoom)} {
		if strings.Contains(line, `black-bold">(`) {
			t.Errorf("the %s line carries the adjective span: %q", role, line)
		}
	}
	if !strings.Contains(string(triad.ToRoom), "rena") {
		t.Errorf("the room line lost the name itself: %q", triad.ToRoom)
	}
}
