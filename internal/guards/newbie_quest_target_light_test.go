package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// newbieQuestTargetRooms are the rooms where a newbie quest asks for an
// AIMED action, which needs full sight (a normal observer reading faces):
// shoot.go refuses a shot below it, cast aims by the same rule, and attack.go
// refuses an attack on a non-hostile mob below it. The 2026-10-17 newbie quest
// audit (B1, B2) found every one of them below full sight at midnight, so a
// new player arriving at night could not finish the step. Owner rule: quest
// rooms are lit so a new player can act at any hour.
var newbieQuestTargetRooms = []struct {
	roomId int
	why    string
}{
	{5305, "Star Chamber: quest 44 casts at the mote"},
	{5354, "Echoing Narrows: quests 51 and 59 fire north at the scout"},
	{5356, "Sounding Floor: the canyon scout, quests 51 and 59"},
	{5289, "Thornbrake: quests 42 and 56 attack a pronghorn"},
	{5291, "Carrion Flat: quests 42 and 56 attack a pronghorn"},
}

// TestNewbieQuestTargetRoomsGiveFullSightAtEveryHour holds that rule: at
// every sampled hour across the year (lighting_daycycle_golden_test.go's
// twelve samples, midnight included), a normal-sighted player with no light
// of their own has full sight in each of these rooms.
func TestNewbieQuestTargetRoomsGiveFullSightAtEveryHour(t *testing.T) {
	loadItemBehaviourWorld(t)
	cfg := configs.GetLightingConfig()

	samples := sampleRounds()
	if len(samples) < 12 {
		t.Fatalf("only %d sample rounds; the walk would not cover the day", len(samples))
	}
	for _, tc := range newbieQuestTargetRooms {
		room := rooms.LoadRoom(tc.roomId)
		if room == nil {
			t.Fatalf("room %d (%s) must load", tc.roomId, tc.why)
		}
		for _, s := range samples {
			util.SetRoundCount(s.Round)
			lvl := room.LightLevel()
			if got := messaging.SightThroughWindow(lvl, 0, 0, cfg.BlindBelow, cfg.DimBelow); got != messaging.SightFull {
				t.Errorf("room %d (%s) at %s: light %d gives a normal observer %v, want full sight (light %d or more)",
					tc.roomId, tc.why, s.Label, lvl, got, cfg.DimBelow)
			}
		}
	}
}
