package hooks

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// logCapture is a mudlog tee that keeps every line it is handed.
type logCapture struct {
	mu    sync.Mutex
	lines []string
}

func (l *logCapture) Println(level string, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	parts := []string{level}
	for _, p := range v {
		if s, ok := p.(string); ok {
			parts = append(parts, s)
		}
	}
	l.lines = append(l.lines, strings.Join(parts, " "))
}

func (l *logCapture) find(msg string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []string{}
	for _, line := range l.lines {
		if strings.Contains(line, msg) {
			out = append(out, line)
		}
	}
	return out
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// spellDamageLineFor applies a test-blast from Aliceia (user 1) to the
// Skeleton (mob 100) in lit room 2 with the given contest outcome, and
// returns the one SpellDamage debug line it wrote, ANSI stripped.
func spellDamageLineFor(t *testing.T, out combat.ChannelDefenceResult) (string, int) {
	t.Helper()
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	m := spellChannelMob(t)
	rooms.LoadRoom(m.Character.RoomId).RemoveMob(m.InstanceId)
	room.AddMob(m.InstanceId)
	m.Character.RoomId = room.RoomId
	m.Character.Validate()
	caster := users.GetByUserId(1)
	sd := &spells.SpellData{SpellId: "test-blast", Name: "Test Blast", EffectType: "damage", DamageMultiplier: 1.0,
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle}

	capture := &logCapture{}
	mudlog.SetupLogger(capture, "DEBUG", "", false)
	t.Cleanup(func() { mudlog.SetupLogger(nil, "", "", false) })
	before := m.Character.Health

	applySpellDamage(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
		actions.NewMobActorInRoom(m, room), room, sd, 10, out))

	lines := capture.find("SpellDamage")
	if len(lines) != 1 {
		t.Fatalf("want exactly one SpellDamage debug line, got %d: %q", len(lines), lines)
	}
	return ansiEscape.ReplaceAllString(lines[0], ""), before - m.Character.Health
}

// TestSpellDamage_DebugLineCarriesTheTuningInputs pins the #480 debug line: a
// spell hit logs, at debug, the inputs a tuner needs to split raw damage
// from mitigation. Melee has had DistDamage and CritDamage for this; spells
// had nothing, which is why the Core Discharge one-shot could not be read
// from the logs.
func TestSpellDamage_DebugLineCarriesTheTuningInputs(t *testing.T) {
	line, taken := spellDamageLineFor(t, spellContestAttackWin())
	for _, want := range []string{
		"test-blast", "Aliceia", "Skeleton",
		"rawDmg=", "channel=\"physical\"", "rawMitig=", "effMitig=", "isCrit=false", "rolled=",
		"defended=false", "evaded=false", "dmg=" + strconv.Itoa(taken),
	} {
		if !strings.Contains(line, want) {
			t.Errorf("SpellDamage line lacks %q: %s", want, line)
		}
	}
	if taken < 1 {
		t.Errorf("a won contest dealt %d, want at least 1", taken)
	}
}

// #480 re-playtest: the line was written inside the damage roll, BEFORE the
// target's defence, so Warden-Prime blasts all three targets dodged logged
// full damage. It is now written by the applier after the defence: an
// evaded hit logs evaded=true and the 0 actually applied.
func TestSpellDamage_DebugLineLogsAnEvadedHitAsZero(t *testing.T) {
	line, taken := spellDamageLineFor(t, combat.ChannelDefenceResult{
		Defended: true, DefensiveCrit: true, Defence: combatvocab.DefenceDodge})
	if taken != 0 {
		t.Fatalf("fixture: an evaded hit dealt %d", taken)
	}
	for _, want := range []string{"defended=true", "evaded=true", "dmg=0"} {
		if !strings.Contains(line, want) {
			t.Errorf("an evaded hit's line lacks %q: %s", want, line)
		}
	}
}
