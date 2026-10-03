package aicompanion

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// The engine's dismiss asks, per companion, whether this module drives it:
// only while the module is on, and only for a companion whose profile it
// has.
func TestDrivesBondedIsPerCompanion(t *testing.T) {
	m := &AICompanionModule{cfg: Config{Enabled: true}, byMob: map[int]*Profile{9800: {Id: `mara`, MobId: 9800}}}
	if !m.drivesBonded(9800) {
		t.Fatal("a companion whose profile is loaded is driven")
	}
	if m.drivesBonded(9902) {
		t.Fatal("a bonded companion with no profile here is driven by nobody")
	}
	m.cfg.Enabled = false
	if m.drivesBonded(9800) {
		t.Fatal("switched off, the module drives nothing")
	}
}

// Nobody is handed a companion any more: a new character is only marked to
// be told where the Waystone Hollow is (hintHollow), and their record gains
// nothing.
func TestNewCharacterIsToldNotGiven(t *testing.T) {
	owner, _, _, _ := harmWorld(t, configs.PVPDisabled)
	m, _ := consentModule()
	m.cfg.Enabled = true
	m.newcomers = map[int]bool{}

	m.onCharacterCreated(events.CharacterCreated{UserId: owner.UserId})
	if !m.newcomers[owner.UserId] {
		t.Fatal("a new character is marked to hear about the Hollow")
	}
	if comp, _ := m.bondedCompanionOf(owner); comp != nil {
		t.Fatal("a new character is not handed a companion")
	}
	if len(owner.Character.Companions) != 0 {
		t.Fatalf("nothing is added to the new character's record: %+v", owner.Character.Companions)
	}

	m.cfg.Enabled = false
	m.newcomers = map[int]bool{}
	m.onCharacterCreated(events.CharacterCreated{UserId: owner.UserId})
	if m.newcomers[owner.UserId] {
		t.Fatal("switched off, the module marks nobody")
	}
}
