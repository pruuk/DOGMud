package usercommands

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// seedBondedDismisser is seedDismisser with the companion bonded, the kind
// the aicompanion module drives, charmed to its owner as login fields it.
func seedBondedDismisser(t *testing.T) (*characters.Character, func() bool, func()) {
	t.Helper()
	u, room, cleanup := seedDismisser(t, 0)
	u.Character.Companions[0].SourceType = characters.CompanionBonded
	u.Character.Companions[0].Name = "Mara"
	u.Character.TrackCharmed(777, true)
	// Beside its owner, where a betrayed charm would turn on them.
	mobs.GetInstance(777).Character.RoomId = u.Character.RoomId
	dismiss := func() bool {
		handled, err := Dismiss("Mara", u, room, 0)
		if err != nil || !handled {
			t.Fatalf("dismiss errored: handled=%v err=%v", handled, err)
		}
		return u.Character.GetCompanionByInstanceId(777) == nil
	}
	return u.Character, dismiss, cleanup
}

// drives installs the module's per-companion check: it drives exactly the
// companions of these mob templates.
func drives(t *testing.T, mobIds ...int) {
	t.Helper()
	companionai.SetDrivesCheck(func(mobId int) bool {
		return slices.Contains(mobIds, mobId)
	})
	t.Cleanup(func() { companionai.SetDrivesCheck(nil) })
}

// While the aicompanion module drives this bonded companion, parting ways
// is its business (companion-part), and dismiss refuses.
func TestDismiss_BondedCompanionRefusedWhileDriven(t *testing.T) {
	_, dismiss, cleanup := seedBondedDismisser(t)
	defer cleanup()

	drives(t, 9902)
	if dismiss() {
		t.Fatal("a driven bonded companion must not be dismissed by command")
	}
}

// With the module on but not driving THIS companion (its profile is not
// loaded, so nothing will ever take it up), dismiss is how the owner parts
// with it: the refusal is per companion, not for the module as a whole.
func TestDismiss_BondedCompanionNobodyDrivesPartsWhileTheModuleIsOn(t *testing.T) {
	_, dismiss, cleanup := seedBondedDismisser(t)
	defer cleanup()

	drives(t, 9800) // some other companion's profile, not this one's
	if !dismiss() {
		t.Fatal("a bonded companion the module does not drive must be dismissable")
	}
}

// With the module switched off its commands are gone, yet login still fields
// a bonded companion from its saved record. Dismiss is then the only way out
// of the bond, so it must work, and peacefully: no charm left behind, and
// no fury, since a bonded companion was never bent to anyone's will.
func TestDismiss_UndrivenBondedCompanionPartsPeacefully(t *testing.T) {
	ch, dismiss, cleanup := seedBondedDismisser(t)
	defer cleanup()

	if companionai.DrivesBonded(9902) {
		t.Fatal("fixture: nothing may drive bonded companions here")
	}
	if !dismiss() {
		t.Fatal("with nothing driving it, a bonded companion must be dismissable")
	}
	for _, id := range ch.GetCharmIds() {
		if id == 777 {
			t.Fatal("the owner must stop tracking the parted companion as charmed")
		}
	}
	if mob := mobs.GetInstance(777); mob != nil && mob.Character.IsInCombat() {
		t.Fatal("a parted bonded companion must not turn on its owner")
	}
}
