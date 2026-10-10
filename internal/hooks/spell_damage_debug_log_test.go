package hooks

import (
	"strings"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/spells"
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

// TestCalcSpellDamage_WritesSpellDamageDebugLine pins the #480 debug line: a
// spell hit through the damage pipeline logs, at debug, the inputs a tuner
// needs to split raw damage from mitigation. Melee has had DistDamage and
// CritDamage for this; spells had nothing, which is why the Core Discharge
// one-shot could not be read from the logs.
func TestCalcSpellDamage_WritesSpellDamageDebugLine(t *testing.T) {
	capture := &logCapture{}
	mudlog.SetupLogger(capture, "DEBUG", "", false)
	t.Cleanup(func() { mudlog.SetupLogger(nil, "", "", false) })

	sd := &spells.SpellData{SpellId: "test-blast", DamageMultiplier: 1.0, DamageType: combatvocab.DamagePhysical}
	caster := spellTestCaster(20)
	caster.Name = "Testcaster"
	target := &characters.Character{Name: "Testtarget"}

	dmg := calcSpellDamageForCharacter(sd, caster, target, 0, false)

	lines := capture.find("SpellDamage")
	if len(lines) != 1 {
		t.Fatalf("want exactly one SpellDamage debug line, got %d: %q", len(lines), lines)
	}
	line := lines[0]
	for _, want := range []string{
		"test-blast", "Testcaster", "Testtarget",
		"rawDmg=", "channel=", "physical", "rawMitig=", "effMitig=", "isCrit=", "dmg=",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("SpellDamage line lacks %q: %s", want, line)
		}
	}
	if dmg < 1 {
		t.Errorf("damage = %d, want at least 1", dmg)
	}
}
